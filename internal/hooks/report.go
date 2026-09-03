package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

// maxPayload bounds what a hook reads. A PostToolUse carries the whole
// tool_response, which is routinely over 64 KiB and was dropped at that cap
// (issue #55): the routable fields are now scanned out and every other value
// is skipped token by token, so the cap guards only a runaway producer
// (invariant 11).
const maxPayload = 4 << 20

// maxField bounds any routable string. A session id or event name longer than
// this is not one claude wrote.
const maxField = 1024

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
	p, ok := ParsePayload(stdin)
	if !ok {
		return nil
	}
	conn, err := net.DialTimeout("unix", socketPath, dialTimeout)
	if err != nil {
		return nil // omatty is not listening; that is fine
	}
	defer func() { _ = conn.Close() }()
	// A peer that accepts and never reads must not hold the hook past
	// claude's own timeout (issue #57).
	_ = conn.SetWriteDeadline(time.Now().Add(dialTimeout))
	if line, err := json.Marshal(p); err == nil {
		_, _ = fmt.Fprintf(conn, "%s\n", line)
	}
	return nil
}

// ParsePayload scans the routable fields out of a hook's stdin. Values it
// does not need - tool_input, tool_response - pass through the decoder
// without being held, so their size never matters. ok is false for
// unreadable, malformed, or session-less input, all dropped silently
// (invariant 11).
//
//	p, ok := hooks.ParsePayload(os.Stdin)
func ParsePayload(stdin io.Reader) (Payload, bool) {
	dec := json.NewDecoder(io.LimitReader(stdin, maxPayload))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return Payload{}, false
	}
	var p Payload
	if !scanFields(dec, &p) {
		return Payload{}, false
	}
	return p, p.SessionID != "" && p.HookEventName != ""
}

// scanFields walks the top-level object. It stops quietly where the cap cut
// the input - the routable fields come first in claude's payloads - and
// reports false only for a routable field that is not a sane string.
func scanFields(dec *json.Decoder, p *Payload) bool {
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return true
		}
		name, _ := key.(string)
		if dst := routableField(name, p); dst != nil {
			if !readString(dec, dst) {
				return false
			}
			continue
		}
		if !skipValue(dec) {
			return true
		}
	}
	return true
}

func routableField(name string, p *Payload) *string {
	switch name {
	case "session_id":
		return &p.SessionID
	case "hook_event_name":
		return &p.HookEventName
	case "notification_type":
		return &p.NotificationType
	case "tool_name":
		return &p.ToolName
	}
	return nil
}

// readString decodes one routable value, refusing anything that is not a
// string of sane length: a 2 MiB session id is a runaway producer, not a
// session (issue #18).
func readString(dec *json.Decoder, dst *string) bool {
	tok, err := dec.Token()
	s, ok := tok.(string)
	if err != nil || !ok || len(s) > maxField {
		return false
	}
	*dst = s
	return true
}

// skipValue consumes one value of any size token by token, so a large
// tool_response flows through the decoder's buffer without being kept.
func skipValue(dec *json.Decoder) bool {
	depth := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		switch tok {
		case json.Delim('{'), json.Delim('['):
			depth++
		case json.Delim('}'), json.Delim(']'):
			depth--
		}
		if depth == 0 {
			return true
		}
	}
}
