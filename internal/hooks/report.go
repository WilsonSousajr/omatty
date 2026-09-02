package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

// maxPayload bounds what a hook reads and forwards. Real payloads are a few
// hundred bytes; the cap stops a runaway producer making the hook allocate
// megabytes (invariant 11).
const maxPayload = 64 << 10

// Payload is the slice of a hook's stdin that status needs.
type Payload struct {
	SessionID        string `json:"session_id"`
	HookEventName    string `json:"hook_event_name"`
	NotificationType string `json:"notification_type,omitempty"`
	ToolName         string `json:"tool_name,omitempty"`
}

// Report reads a hook payload from stdin and forwards it to omatty's socket as
// one JSON line. It is the whole of `omatty hook`.
//
// Invariant 11: a hook must never block or fail claude. Every failure — no
// socket (omatty closed), refused connection, malformed input — returns nil so
// the command exits 0. The error return exists only so tests can assert the
// forwarding path; cmd discards it.
func Report(stdin io.Reader, socketPath string, dialTimeout time.Duration) error {
	p, ok := parsePayload(stdin)
	if !ok {
		return nil
	}
	conn, err := net.DialTimeout("unix", socketPath, dialTimeout)
	if err != nil {
		return nil // omatty is not listening; that is fine
	}
	defer func() { _ = conn.Close() }()
	if line, err := json.Marshal(p); err == nil {
		_, _ = fmt.Fprintf(conn, "%s\n", line)
	}
	return nil
}

// parsePayload reads at most maxPayload bytes and extracts the routable
// fields. ok is false for unreadable, malformed, or session-less input, all
// of which are dropped silently (invariant 11).
func parsePayload(stdin io.Reader) (Payload, bool) {
	raw, err := io.ReadAll(io.LimitReader(stdin, maxPayload))
	if err != nil {
		return Payload{}, false
	}
	var p Payload
	if json.Unmarshal(raw, &p) != nil || p.SessionID == "" {
		return Payload{}, false
	}
	return p, true
}
