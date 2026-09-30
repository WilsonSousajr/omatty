// Package hookserver is omatty's end of the hook socket: it accepts the
// connections `omatty hook` makes, reads one bounded payload from each, and
// offers it on - never waiting, so a hook never waits on omatty (invariant
// 11). What a payload means is internal/watcher's business (ADR 0001,
// migration step 5.2d, #653).
//
//	payloads := make(chan status.HookPayload, 64)
//	l, err := hookserver.Listen(paths.HookSocket(home), payloads)
package hookserver

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
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

// Listener turns hook connections on a unix socket into payloads.
type Listener struct {
	ln      net.Listener
	sink    chan<- dstatus.HookPayload
	stop    chan struct{}
	slots   chan struct{}
	wg      sync.WaitGroup
	once    sync.Once
	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	dropped atomic.Int64
}

// Listen accepts hook connections on path and offers each well-formed payload
// to sink. A stale socket file is replaced; the socket is user-only.
//
//	l, err := hookserver.Listen(paths.HookSocket(home), payloads)
//	defer l.Close()
func Listen(path string, sink chan<- dstatus.HookPayload) (*Listener, error) {
	if err := refuseIfLive(path); err != nil {
		return nil, err
	}
	// A leftover socket file from a previous run makes bind fail; remove it.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("hookserver: clearing stale socket %q: %w", path, err)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("hookserver: listening on %q: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("hookserver: securing socket %q: %w", path, err)
	}
	l := &Listener{ln: ln, sink: sink, stop: make(chan struct{}),
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
	return fmt.Errorf("hookserver: another omatty is listening on %q; hook status is disabled in this instance", path)
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

// Dropped counts payloads that found the sink full. A non-zero value means
// omatty fell behind; the tailer has since restored the truth.
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

// serve reads one bounded line from conn within readTimeout and, if it
// decodes, offers it to the sink. One connection per hook.
func (l *Listener) serve(conn net.Conn) {
	defer l.wg.Done()
	defer func() { <-l.slots }()
	defer recoverLoop("hook connection", "")
	defer l.track(conn, false)
	defer func() { _ = conn.Close() }()
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
	if p, ok := l.decode(conn); ok {
		l.offer(p)
	}
}

// decode reads one bounded line as a payload. ok is false for an empty,
// oversized or malformed payload, all of which are dropped. Whether the
// payload names an event anyone tracks is decided upstream, by the agent's
// adapter.
func (l *Listener) decode(conn net.Conn) (dstatus.HookPayload, bool) {
	line, ok := readLine(conn)
	if !ok {
		return dstatus.HookPayload{}, false
	}
	var p dstatus.HookPayload
	if json.Unmarshal(line, &p) != nil {
		return dstatus.HookPayload{}, false
	}
	return p, true
}

// offer sends without blocking. A full sink means omatty is behind; the
// tailer restores the truth within a second, so dropping a hook payload costs
// only latency, while blocking would stall every hook on the machine.
func (l *Listener) offer(p dstatus.HookPayload) {
	select {
	case l.sink <- p:
	default:
		l.dropped.Add(1)
		slog.Debug("hook payload dropped, sink full", "session", p.SessionID)
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

// recoverLoop keeps a panic in one connection's goroutine from taking omatty
// down with it (invariant 6): it logs and lets the goroutine end. A copy of
// watcher's, which this package cannot import.
func recoverLoop(what, id string) {
	if r := recover(); r != nil {
		slog.Error("recovered a panic", "in", what, "session", id, "panic", r)
	}
}
