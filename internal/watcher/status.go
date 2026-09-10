package watcher

// Status is a session's live state, derived from the JSONL transcript and
// hook events (invariant 2). It lives with its producer and is deliberately
// never persisted (issue #77: registry declared it but never used it).
type Status string

// The states a session can be in, as shown in the sidebar.
const (
	StatusIdle     Status = "idle"
	StatusThinking Status = "thinking"
	StatusTool     Status = "tool"
	StatusWaiting  Status = "waiting"
	StatusDone     Status = "done"
	StatusError    Status = "error"
	// StatusExited means claude quit (SessionEnd). Not an error and not idle:
	// the operator needs to know ctrl+o r applies.
	StatusExited Status = "exited"
)
