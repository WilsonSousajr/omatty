package watcher

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"time"

	"github.com/WilsonSousajr/omatty/internal/hooks"
)

// maxLine bounds a single hook payload read from the socket, matching the cap
// in the hook writer. A payload larger than this is dropped, not buffered.
const maxLine = 64 << 10

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
	ln    net.Listener
	clock func() time.Time
}

// Listen accepts hook connections on path and sends an Event per valid payload
// to sink. A stale socket file is replaced; the socket is user-only. clock
// stamps each event so it compares like-for-like with tailer timestamps.
//
//	l, err := watcher.Listen(paths.HookSocket(home), events, time.Now)
//	defer l.Close()
func Listen(path string, sink chan<- Event, clock func() time.Time) (*Listener, error) {
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
	l := &Listener{ln: ln, clock: clock}
	go l.accept(sink)
	return l, nil
}

// Close stops accepting connections.
func (l *Listener) Close() error { return l.ln.Close() }

func (l *Listener) accept(sink chan<- Event) {
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			return // listener closed
		}
		l.handle(conn, sink)
	}
}

// handle reads one bounded line from conn and, if it is a tracked event,
// sends it. One connection per hook, so serving inline keeps ordering and
// avoids unbounded goroutines under a burst (invariant 5's spirit).
func (l *Listener) handle(conn net.Conn, sink chan<- Event) {
	defer func() { _ = conn.Close() }()
	line, ok := readLine(conn)
	if !ok {
		return
	}
	var p hooks.Payload
	if json.Unmarshal(line, &p) != nil {
		return
	}
	if kind, ok := KindOf(p); ok {
		sink <- Event{SessionID: p.SessionID, Kind: kind, At: l.clock(), Tool: p.ToolName}
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
