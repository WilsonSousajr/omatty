// Everything a Model is given, and the defaults that mean no method needs a
// nil guard (#76). Split from model.go when M7's config, naming and agent
// dependencies took the struct past the file limit on its own.

package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/notify"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// CreateFunc registers a new session in project and returns it.
//
// worktree says whether omatty creates one, which since #151 is no longer the
// same question as whether branch is empty: a worktree session may arrive with
// no branch named at all, and the registry names it.
type CreateFunc func(project, title, branch string, worktree bool) (registry.Session, error)

// StartFunc launches the embedded terminal for a session at w by h. Injected
// so the model can start a session created at runtime without knowing how;
// the size is a parameter so it is never frozen at startup (issue #73).
type StartFunc func(sess registry.Session, w, h int) (termwrap.Terminal, error)

// TickFunc schedules fn after d, as tea.Tick does (#412).
//
//	var t ui.TickFunc = tea.Tick
type TickFunc func(d time.Duration, fn func(time.Time) tea.Msg) tea.Cmd

// RepoStatFunc reads a session's branch and diffstat for its sidebar card
// (#180). Injected so ui never touches git (invariant 4). Nil is the switch,
// as ModelName's is: with nothing wired the card's second line is blank,
// which is what every test's Deps gets.
type RepoStatFunc func(sess registry.Session, projectRoot string) (review.Stat, error)

// TurnFuncs are the three calls #311 makes on a session's turn baseline,
// injected because ui may not touch git (invariant 4). Snap records the
// baseline as a prompt is submitted, Diff loads what changed since it, and
// Drop deletes it when the session is archived.
type TurnFuncs struct {
	Snap func(sess registry.Session) error
	Diff DiffFunc
	Drop func(sess registry.Session, projectRoot string) error
}

// Deps is everything a Model needs. Constructor injection, so no field is
// set after the fact and no method needs a nil guard (issue #76). The zero
// value of an optional field means: no status stream, the wall clock, a
// silent notifier, no tailer for runtime sessions.
//
//	m := ui.NewModel(ui.Deps{State: st, Terms: terms, Create: create, Start: start,
//	        Events: w.Events(), Clock: time.Now, Notifier: notify.New(), TailStart: w.Add})
type Deps struct {
	State  registry.State
	Terms  map[string]termwrap.Terminal
	Create CreateFunc
	Start  StartFunc
	Events <-chan watcher.Event
	// GateReports and GateRun wire internal/gate's Runner in. Both optional:
	// without them the gate pane still opens and explains itself, which is
	// what a model built by a test sees.
	GateReports <-chan gate.Report
	GateRun     GateRunFunc
	// GateAuto runs a session's gate when its turn ends. Off by default: a
	// test suite on every idle costs real time, so it is asked for (#233).
	GateAuto bool
	Clock    func() time.Time
	// SpinTick schedules the spinner's next frame (#412). Nil is tea.Tick; a
	// test passes one that answers at once, since test helpers run every
	// command they are handed and a real tick is a real 100 ms wait.
	SpinTick  TickFunc
	Notifier  notify.Notifier
	TailStart func(registry.Session)
	// Diff loads a session's changes for the review column (#21).
	Diff DiffFunc
	// Files lists a session's worktree and Preview reads one of its files,
	// for the tree view (#24).
	Files   ListFilesFunc
	Preview PreviewFunc
	// Generated reports which of a session's files nobody wrote, so the tree
	// can fold them and the coverage markers can leave them alone (#338).
	// Unwired, nothing is generated - which is what every tree looked like
	// before this and is the safe direction to be wrong in.
	Generated GeneratedFunc
	// Stat reads a session's branch and diffstat for its card; nil means no
	// git to ask (#180).
	Stat RepoStatFunc
	// Turn reaches a session's turn baseline (#311). Unwired, Snap and Drop
	// do nothing and Diff says there is no turn - what every test sees.
	Turn TurnFuncs
	// HooksDown says the hook socket did not bind (#49), so no turn baseline
	// will ever be taken; the turn view says so instead of diffing (#311).
	HooksDown bool
	// PRs lists a project's pull requests for its cards (#310) and Issues its
	// open issues for the tracker (#394). Unwired, each says gh is missing,
	// which stops every poll: what every test sees.
	PRs    PRListFunc
	Issues IssueListFunc
	// Item reads one issue or pull request in full, on the keypress that opens
	// it (#397). Unwired, it says gh is missing.
	Item ForgeItemFuncs
	// Browse opens one item in the operator's browser (#398). Unwired, it names
	// the missing wiring.
	Browse BrowseFunc
	// Rename persists a session's new title (#41), and Name reads the first
	// prompt that titles a session created without one (#127).
	Rename RenameFunc
	Name   NameFunc
	// Rebind persists the conversation a session's claude moved to on /clear
	// (#316).
	Rebind RebindFunc
	// RenameBranch gives a worktree's placeholder branch the name its first
	// prompt settled on (#151).
	RenameBranch BranchRenameFunc
	// ModelName asks the agent to improve an auto-derived title. Nil means
	// off, which is the default and the operator's opt-in (#127).
	ModelName ModelNameFunc
	// Archive drops a session from the registry, RemoveWorktree deletes its
	// worktree, and TailStop ends its status tailer (#40).
	Archive        ArchiveFunc
	RemoveWorktree RemoveWorktreeFunc
	TailStop       func(sessionID string)
	// RemoveProject forgets a registered project that holds no sessions (#159).
	RemoveProject RemoveProjectFunc
	// Discover proposes repositories to register and AddProject registers one
	// (#91).
	Discover   DiscoverFunc
	AddProject AddProjectFunc
	// AdoptPropose lists the claude sessions inside a project that omatty does
	// not yet hold, and AdoptCommit registers the chosen ones (#122).
	AdoptPropose AdoptFunc
	AdoptCommit  AdoptCommitFunc
	// Stop ends the claude a detach holder keeps alive for a session. It is
	// called when a session is archived and never when omatty quits, which is
	// the whole point of holding it (#43).
	Stop StopFunc
	// Notice is said once, in the footer, until the first keypress. It carries
	// what an operator has to know at startup and cannot discover from the
	// screen - today, that dtach is missing so sessions will not survive quit.
	Notice string
	// Leader is the key omatty intercepts. Empty means DefaultLeader (#44).
	Leader string
	// IdleStop stops a session quiet this long, as ctrl+o s would; zero is
	// off, and the default (#319).
	IdleStop time.Duration
	// Reattached names the sessions whose claude was already running when
	// omatty started: their panes come back blank and are asked to repaint
	// once at boot (#191). Nil means none.
	Reattached map[string]bool
}

// withDefaults fills the optional fields: the wall clock, the real timer and
// a silent notifier, so no method needs a nil guard.
func (d Deps) withDefaults() Deps {
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.SpinTick == nil {
		d.SpinTick = tea.Tick
	}
	if d.Notifier == nil {
		d.Notifier = notify.Silent{}
	}
	if d.Leader == "" {
		d.Leader = DefaultLeader
	}
	return d.withReviewDefaults().withTurnDefaults().withPRDefaults().withLifecycleDefaults()
}

// withReviewDefaults fills the review column's three readers (#21, #24).
func (d Deps) withReviewDefaults() Deps {
	if d.Diff == nil {
		d.Diff = noDiff
	}
	if d.Files == nil {
		d.Files = noFiles
	}
	// The real reader has no dependency to inject, so it is the default
	// rather than an error: only a test replaces it (#24).
	if d.Preview == nil {
		d.Preview = review.ReadPreview
	}
	if d.Generated == nil {
		d.Generated = noGenerated
	}
	return d
}

// withPRDefaults fills the pull request reader (#310): unwired, a project has
// no pull requests, which is what a test sees and what a machine without gh
// would show anyway.
func (d Deps) withPRDefaults() Deps {
	if d.PRs == nil {
		d.PRs = noPRs
	}
	if d.Issues == nil {
		d.Issues = noIssues
	}
	if d.Item.Issue == nil {
		d.Item.Issue = noItem
	}
	if d.Item.PR == nil {
		d.Item.PR = noItem
	}
	if d.Browse == nil {
		d.Browse = noBrowse
	}
	return d
}

// withTurnDefaults fills the turn baseline's calls (#311): unwired, Snap and
// Drop do nothing and Diff says there is no turn, which is what a test sees.
func (d Deps) withTurnDefaults() Deps {
	if d.Turn.Snap == nil {
		d.Turn.Snap = func(registry.Session) error { return nil }
	}
	if d.Turn.Diff == nil {
		d.Turn.Diff = func(registry.Session, string) (review.Diff, error) { return review.Diff{}, review.ErrNoTurn }
	}
	if d.Turn.Drop == nil {
		d.Turn.Drop = func(registry.Session, string) error { return nil }
	}
	return d
}

// withLifecycleDefaults fills the rename and archive commands (#40, #41).
//
// Every default here is non-nil once this has run, so no method needs a guard -
// which is the promise Deps makes. Most name their missing wiring so an unwired
// dependency shows up in the pane rather than failing silently; noTailStop and
// noTailStart are the exceptions, because with no watcher running there is
// genuinely no tailer to start or stop and doing nothing is the right answer.
func (d Deps) withLifecycleDefaults() Deps {
	d = d.withNamingDefaults()
	if d.Archive == nil {
		d.Archive = noArchive
	}
	if d.RemoveProject == nil {
		d.RemoveProject = noRemoveProject
	}
	if d.RemoveWorktree == nil {
		d.RemoveWorktree = noRemoveWorktree
	}
	if d.Stop == nil {
		d.Stop = noStop
	}
	return d.withTailDefaults().withDiscoveryDefaults()
}

// withNamingDefaults fills what names a session: its title, the first prompt
// that derives one, the branch that takes it, and the conversation it is on
// (#41, #127, #151, #316). Split out
// of withLifecycleDefaults because #151 pushed that one past the length limit,
// and these three answer one question between them.
func (d Deps) withNamingDefaults() Deps {
	if d.Rename == nil {
		d.Rename = noRename
	}
	if d.RenameBranch == nil {
		d.RenameBranch = noBranchRename
	}
	if d.Name == nil {
		d.Name = noName
	}
	if d.Rebind == nil {
		d.Rebind = noRebind
	}
	return d
}

// withTailDefaults fills both halves of the status tailer.
//
// Both, not one: while only TailStop was defaulted, addSession still needed
// `if m.tailStart != nil` where dropSession did not, so a reader could not tell
// from Deps which injected funcs are guaranteed non-nil (#40).
func (d Deps) withTailDefaults() Deps {
	if d.TailStart == nil {
		d.TailStart = noTailStart
	}
	if d.TailStop == nil {
		d.TailStop = noTailStop
	}
	return d
}

// withDiscoveryDefaults fills the project picker's two dependencies (#91).
func (d Deps) withDiscoveryDefaults() Deps {
	if d.Discover == nil {
		d.Discover = noDiscover
	}
	if d.AddProject == nil {
		d.AddProject = noAddProject
	}
	if d.AdoptPropose == nil {
		d.AdoptPropose = noAdopt
	}
	if d.AdoptCommit == nil {
		d.AdoptCommit = noAdoptCommit
	}
	return d
}

// GateRunFunc asks for a gate run. It is gate.Runner.Start, named here so the
// model takes a function rather than the Runner itself (invariant: the UI
// holds no concurrency of its own).
type GateRunFunc func(sessionID, dir string, steps []gate.Step)

// noGenerated is the unwired Generated: nothing is generated, so every file
// stays in the tree and every coverage marker stands. Wrong in the safe
// direction - a file shown is a file the operator can judge, where a file
// folded away by a broken detection is one they never see.
func noGenerated(registry.Session, []string) (map[string]bool, error) { return nil, nil }
