// The contract an agent's status reader fulfils (ADR 0001's Adapter port).
// It moved here from internal/service/status by migration Amendment 7 (#653):
// internal/domain/agent profiles carry an Adapter, and domain may not import
// the service layer. It names domain types only; the parsers that implement
// it stay in the service.

package status

import "time"

// Entry is the slice of a transcript line that status needs. Every other line
// type - attachments, queue operations, titles, snapshots - is dropped at
// parse, so callers only ever hold user and assistant turns.
type Entry struct {
	Type         string // "user" or "assistant"
	MessageID    string // assistant: one API response spans several lines under one id
	At           time.Time
	StopReason   string // assistant
	UserIsPrompt bool   // user: a typed prompt (a string, or text/image blocks)
	ToolUse      bool   // assistant: a tool_use block is present
	ToolResult   bool   // user: a tool_result block is present
	Usage        Tokens // assistant
	// Cwd is the directory the agent was working in when it wrote the line,
	// empty when the line does not say (#659).
	Cwd string
}

// Adapter turns one agent's transcript lines and hook payloads into events.
//
//	tl := status.Tail(id, path, events, time.Now, time.Second, status.ClaudeAdapter())
type Adapter interface {
	// ParseEntry parses one transcript line. ok is false for a line status
	// does not need, including malformed JSON.
	ParseEntry(line []byte) (Entry, bool)
	// DeriveKind is the event implied by the most recent relevant entry in a
	// tail.
	DeriveKind(entries []Entry) (Kind, time.Time, bool)
	// KindOf maps a hook payload to the event it represents.
	KindOf(p HookPayload) (Kind, bool)
}
