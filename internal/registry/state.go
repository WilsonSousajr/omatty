// Package registry holds omatty's projects and sessions and persists them.
package registry

// Version is the state.json schema version. Bump only with a migration.
const Version = 1

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
	ID      string `json:"id"` // uuid, passed to claude --session-id
	Project string `json:"project"`
	Title   string `json:"title"`
	Dir     string `json:"dir"` // absolute working directory
	Branch  string `json:"branch"`
	// Base is the branch the worktree was forked from, recorded at creation
	// so review has a merge-base to diff against (#21). Empty for a
	// main-checkout session and for worktrees made before M3, which fall back
	// to the project root's current branch at review time (invariant 9: the
	// empty value is derivable).
	Base     string `json:"base,omitempty"`
	Worktree bool   `json:"worktree"` // true if omatty created Dir
	// Agent names which coding agent runs this session. Empty means claude,
	// which is what every row written before #46 has and what agent.Lookup
	// resolves it to - so the field is derivable and Version stays 1
	// (invariant 9), the argument Base carries above. Write "" for claude,
	// never "claude": two spellings of one agent in one file is the drift the
	// derivable default exists to avoid.
	Agent string `json:"agent,omitempty"`
}

// State is the whole persisted registry.
type State struct {
	Version  int       `json:"version"`
	Projects []Project `json:"projects"`
	Sessions []Session `json:"sessions"`
}
