package hooks

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/status"
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

// Report reads a hook payload from stdin and forwards it to omatty's socket as
// one JSON line, stamped with omattySession - the value of SessionEnv in the
// hook's environment, empty for a claude omatty did not launch. It is the
// whole of `omatty hook`.
//
//	_ = hooks.Report(os.Stdin, paths.HookSocket(home), time.Second, os.Getenv(session.SessionEnv))
//
// Invariant 11: a hook must never block or fail claude. Every failure — no
// socket (omatty closed), refused connection, malformed input — returns nil so
// the command exits 0. The error return exists only so tests can assert the
// forwarding path; cmd discards it.
func Report(stdin io.Reader, socketPath string, dialTimeout time.Duration, omattySession string) error {
	return ReportAs(stdin, ParsePayload, socketPath, dialTimeout, omattySession)
}

// ReportAs is Report with the agent's own payload parser, for `omatty hook
// --agent <name>`: each agent's shape becomes the typed payload here, at the
// edge, and the socket only ever sees that (#522). Invariant 11 holds exactly
// as for Report: every failure returns nil.
//
//	_ = hooks.ReportAs(os.Stdin, profile.ParseHook, socket, time.Second, owner)
func ReportAs(stdin io.Reader, parse func(io.Reader) (status.HookPayload, bool), socketPath string, dialTimeout time.Duration, omattySession string) error {
	p, ok := parse(stdin)
	if !ok {
		return nil
	}
	p.OmattySession = omattySession
	conn, err := net.DialTimeout("unix", socketPath, dialTimeout)
	if err != nil {
		return nil // omatty is not listening; that is fine
	}
	defer func() { _ = conn.Close() }()
	sendLine(conn, p, dialTimeout)
	return nil
}

// sendLine writes p to conn as one JSON line, under a write deadline.
//
// The deadline is the whole point: a peer that accepts the connection and then
// never reads would otherwise park this write forever, and every claude session
// on the machine stalls behind its own hook (issue #57, invariant 11). It is
// extracted from Report so the parked-peer case can be tested over an
// unbuffered net.Pipe — a real socket's kernel buffer swallows a payload this
// small, so the deadline never engages there and the guard would be untestable.
func sendLine(conn net.Conn, p status.HookPayload, timeout time.Duration) {
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	line, err := json.Marshal(p)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(conn, "%s\n", line)
}

// ParsePayload scans the routable fields out of a hook's stdin. Values it
// does not need - tool_input, tool_response - pass through the decoder
// without being held, so their size never matters. ok is false for
// unreadable, malformed, or session-less input, all dropped silently
// (invariant 11).
//
//	p, ok := hooks.ParsePayload(os.Stdin)
func ParsePayload(stdin io.Reader) (status.HookPayload, bool) {
	return parseRouted(stdin, routableField)
}

// fieldRouter names where a top-level field's string value goes, or nil
// for a field to skip. A parser for another agent's shape routes one more
// field to a local of its own (#152).
type fieldRouter func(name string, p *status.HookPayload) *string

// parseRouted is ParsePayload with the routing as a parameter.
func parseRouted(stdin io.Reader, route fieldRouter) (status.HookPayload, bool) {
	dec := json.NewDecoder(io.LimitReader(stdin, maxPayload))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return status.HookPayload{}, false
	}
	var p status.HookPayload
	if !scanFields(dec, &p, route) {
		return status.HookPayload{}, false
	}
	return p, p.SessionID != "" && p.HookEventName != ""
}

// scanFields walks the top-level object. It stops quietly where the cap cut
// the input - the routable fields come first in claude's payloads - and
// reports false only for a routable field that is not a sane string.
func scanFields(dec *json.Decoder, p *status.HookPayload, route fieldRouter) bool {
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return true
		}
		name, _ := key.(string)
		if dst := route(name, p); dst != nil {
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

func routableField(name string, p *status.HookPayload) *string {
	switch name {
	case "session_id":
		return &p.SessionID
	case "hook_event_name":
		return &p.HookEventName
	case "notification_type":
		return &p.NotificationType
	case "tool_name":
		return &p.ToolName
	case "source":
		return &p.Source
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
