// Package registry holds omatty's projects and sessions and persists them.
package registry

// Version is the state.json schema version. Bump only with a migration.
const Version = 1

// Status is a session's live state. It is derived from the JSONL transcript
// and hook events (invariant 2) and is deliberately never persisted.
type Status string

// The states a session can be in, as shown in the sidebar.
const (
	StatusIdle     Status = "idle"
	StatusThinking Status = "thinking"
	StatusTool     Status = "tool"
	StatusWaiting  Status = "waiting"
	StatusDone     Status = "done"
	StatusError    Status = "error"
)

// Project is a registered git repository.
type Project struct {
	Name string `json:"name"`
	Root string `json:"root"` // absolute path to the main checkout
}

// Session is one Claude Code process in one directory.
//
// Every field here is required to relaunch the session with
// `claude --resume <ID>` after a crash (invariant 9). Status is absent by
// design: it is derived at runtime, never stored.
type Session struct {
	ID       string `json:"id"` // uuid, passed to claude --session-id
	Project  string `json:"project"`
	Title    string `json:"title"`
	Dir      string `json:"dir"` // absolute working directory
	Branch   string `json:"branch"`
	Worktree bool   `json:"worktree"` // true if omatty created Dir
}

// State is the whole persisted registry.
type State struct {
	Version  int       `json:"version"`
	Projects []Project `json:"projects"`
	Sessions []Session `json:"sessions"`
}
