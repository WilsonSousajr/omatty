// Package registry holds omatty's projects and sessions and persists them.
package registry

import (
	"time"

	"github.com/WilsonSousajr/omatty/internal/gate"
)

// Version is the state.json schema version. Bump only with a migration.
const Version = 1

// Project is a registered git repository and the gate it is verified by.
type Project struct {
	Name string `json:"name"`
	Root string `json:"root"` // absolute path to the main checkout
	// Gate is the project's own verification commands, run in a session's
	// directory when the session goes quiet (#227).
	//
	// Nil is not missing data, it is "not configured yet", which is what
	// gate.Detect and the confirm picker are for - so a file written before
	// M9 needs no migration and Version stays 1. That is the argument Agent
	// and Base already carry above: the empty value is derivable, so it is
	// not a schema break (invariant 9).
	//
	// Never written as an empty array. A project with no gate omits the key,
	// so the file stays readable and a review diff stays quiet.
	Gate []gate.Step `json:"gate,omitempty"`
	// Carry is the gitignored paths copied into every new worktree of this
	// project - .env, local certificates, generated config (#309).
	//
	// Nil carries nothing, which is the same argument Gate makes above: the
	// empty value is derivable, so a file written before this needs no
	// migration and Version stays 1 (invariant 9). Each path is relative to
	// Root; anything absolute or climbing out of it is refused when it runs.
	Carry []string `json:"carry,omitempty"`
	// GateRuns counts the gate runs that followed a turn and GateFirstPass how
	// many of those passed, which together are #332's first-pass rate.
	//
	// Two ints rather than a struct because `omitempty` does not omit a struct:
	// a project that has measured nothing must leave the keys out entirely, so
	// a file written before this needs no migration and a review diff stays
	// quiet. Absent means "nothing measured yet", which is the derivable empty
	// value Gate and Carry above make the same argument for (invariant 9).
	//
	// Counters, not a history. #332 asks for two numbers and R9 says not to
	// widen the surface; a list of runs would be a history browser nobody
	// proposed.
	GateRuns      int `json:"gate_runs,omitempty"`
	GateFirstPass int `json:"gate_first_pass,omitempty"`
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
	// Conversation is the uuid claude is on now, when that is no longer ID.
	// /clear moves claude to a new uuid and a new transcript; ID stays the
	// row's permanent key, because it also names the dtach socket that holds
	// the process and every map the UI keeps (#316). Empty means ID, which is
	// every row written before #316 - derivable, so Version stays 1
	// (invariant 9, the argument Base and Agent make above).
	Conversation string `json:"conversation,omitempty"`
	// Started is when omatty registered this session, which is where lead time
	// is measured from (#332).
	//
	// Zero means unknown - every session written before #332 - and lead time
	// reads as absent for it rather than as an implausible number. That is the
	// derivable empty value Base, Agent, Conversation, Gate and Carry all rely
	// on, so Version stays 1 (invariant 9). Not needed to relaunch a session,
	// which is why it took a reason beyond curiosity to add.
	Started time.Time `json:"started,omitempty"`
}

// ConversationID is the uuid to resume and the transcript to tail.
//
//	path := paths.Transcript(home, sess.Dir, sess.ConversationID())
func (s Session) ConversationID() string {
	if s.Conversation != "" {
		return s.Conversation
	}
	return s.ID
}

// State is the whole persisted registry.
type State struct {
	Version  int       `json:"version"`
	Projects []Project `json:"projects"`
	Sessions []Session `json:"sessions"`
}
