package watcher

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WilsonSousajr/omatty/internal/hooks"
)

// maxLine bounds a single hook payload read from the socket, matching the cap
// in the hook writer. A payload larger than this is dropped, not buffered.
const maxLine = 64 << 10

// readTimeout bounds how long a connected hook may take to send its line. A
// hook writes immediately, so a slower peer is stuck or hostile and must not
// hold a slot (issue #67).
const readTimeout = 2 * time.Second

// maxInFlight bounds concurrent connections. A hook is one line, so a burst
// beyond this waits in the kernel backlog rather than spawning goroutines.
const maxInFlight = 32

// kindByEvent maps a hook event name to its status event. Notification is
// absent here because it depends on notification_type.
var kindByEvent = map[string]Kind{
	"SessionStart":      SessionStarted,
	"UserPromptSubmit":  PromptSubmitted,
	"PreToolUse":        ToolStarted,
	"PostToolUse":       ToolFinished,
	"PermissionRequest": PermissionRequested,
	"Stop":              TurnEnded,
	"SessionEnd":        SessionEnded,
}

// HookEventNames lists every hook event the listener maps to a Kind, plus
// Notification, whose kind depends on notification_type. hooks.Render takes
// this list, so the settings file and the listener can never drift (issue
// #78). Sorted, so the rendered file is stable.
//
//	content, _ := hooks.Render(bin, watcher.HookEventNames())
func HookEventNames() []string {
	names := make([]string, 0, len(kindByEvent)+1)
	for name := range kindByEvent {
		names = append(names, name)
	}
	names = append(names, "Notification")
	sort.Strings(names)
	return names
}

// KindOf maps a hook payload to the status event it represents. ok is false
// for events omatty does not track, which the listener drops.
func KindOf(p hooks.Payload) (Kind, bool) {
	if p.HookEventName == "Notification" {
		return notificationKind(p.NotificationType)
	}
	kind, ok := kindByEvent[p.HookEventName]
	return kind, ok
}

func notificationKind(notifType string) (Kind, bool) {
	switch notifType {
	case "idle_prompt":
		return Idle, true
	case "permission_prompt":
		return PermissionRequested, true
	default:
		return 0, false
	}
}

// Listener turns hook connections on a unix socket into Events.
type Listener struct {
	ln      net.Listener
	clock   func() time.Time
	sink    chan<- Event
	stop    chan struct{}
	slots   chan struct{}
	wg      sync.WaitGroup
	once    sync.Once
	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	dropped atomic.Int64
}

// Listen accepts hook connections on path and sends an Event per valid payload
// to sink. A stale socket file is replaced; the socket is user-only. clock
// stamps each event so it compares like-for-like with tailer timestamps.
//
//	l, err := watcher.Listen(paths.HookSocket(home), events, time.Now)
//	defer l.Close()
func Listen(path string, sink chan<- Event, clock func() time.Time) (*Listener, error) {
	if err := refuseIfLive(path); err != nil {
		return nil, err
	}
	// A leftover socket file from a previous run makes bind fail; remove it.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("watcher: clearing stale socket %q: %w", path, err)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("watcher: listening on %q: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("watcher: securing socket %q: %w", path, err)
	}
	l := &Listener{ln: ln, clock: clock, sink: sink, stop: make(chan struct{}),
		slots: make(chan struct{}, maxInFlight), conns: map[net.Conn]struct{}{}}
	l.wg.Add(1)
	go l.accept()
	return l, nil
}

// refuseIfLive returns an error when another omatty already answers on path,
// so a second instance degrades to tailer-only instead of stealing the socket
// from the first (issue #68). A stale file from a dead process does not
// answer and is removed as before.
func refuseIfLive(path string) error {
	c, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err != nil {
		return nil
	}
	_ = c.Close()
	return fmt.Errorf("watcher: another omatty is listening on %q; hook status is disabled in this instance", path)
}

// Close stops accepting, closes every in-flight connection, and waits for the
// goroutines to exit (issue #67).
func (l *Listener) Close() error {
	var err error
	l.once.Do(func() {
		close(l.stop)
		err = l.ln.Close()
		l.closeConns()
	})
	l.wg.Wait()
	return err
}

// Dropped counts hook events that found the sink full. A non-zero value
// means the UI fell behind; the tailer has since restored the truth.
func (l *Listener) Dropped() int64 { return l.dropped.Load() }

func (l *Listener) accept() {
	defer l.wg.Done()
	defer recoverLoop("listener", "")
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			return // listener closed
		}
		select {
		case l.slots <- struct{}{}:
		case <-l.stop:
			_ = conn.Close()
			return
		}
		l.track(conn, true)
		l.wg.Add(1)
		go l.serve(conn)
	}
}

// serve reads one bounded line from conn within readTimeout and, if it is a
// tracked event, offers it to the sink. One connection per hook.
func (l *Listener) serve(conn net.Conn) {
	defer l.wg.Done()
	defer func() { <-l.slots }()
	defer recoverLoop("hook connection", "")
	defer l.track(conn, false)
	defer func() { _ = conn.Close() }()
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
	if ev, ok := l.decode(conn); ok {
		l.offer(ev)
	}
}

// decode reads one bounded line and maps it to an event. ok is false for an
// empty, oversized, malformed, or untracked payload, all of which are dropped.
func (l *Listener) decode(conn net.Conn) (Event, bool) {
	line, ok := readLine(conn)
	if !ok {
		return Event{}, false
	}
	var p hooks.Payload
	if json.Unmarshal(line, &p) != nil {
		return Event{}, false
	}
	kind, ok := KindOf(p)
	if !ok {
		return Event{}, false
	}
	return Event{SessionID: p.SessionID, Kind: kind, At: l.clock()}, true
}

// offer sends without blocking. A full sink means the UI is behind; the
// tailer restores the truth within a second, so dropping a hook event costs
// only latency, while blocking would stall every hook on the machine.
func (l *Listener) offer(ev Event) {
	select {
	case l.sink <- ev:
	default:
		l.dropped.Add(1)
		slog.Debug("hook event dropped, sink full", "session", ev.SessionID)
	}
}

func (l *Listener) track(conn net.Conn, add bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if add {
		l.conns[conn] = struct{}{}
		return
	}
	delete(l.conns, conn)
}

func (l *Listener) closeConns() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for c := range l.conns {
		_ = c.Close()
	}
}

// readLine reads one payload, capped at maxLine. ok is false for an empty read
// or an oversized line, both of which are dropped.
func readLine(conn net.Conn) ([]byte, bool) {
	line, err := bufio.NewReaderSize(conn, maxLine).ReadSlice('\n')
	if (err != nil && len(line) == 0) || len(line) >= maxLine {
		if len(line) >= maxLine {
			slog.Debug("hook payload exceeded the cap, dropped")
		}
		return nil, false
	}
	return line, true
}
