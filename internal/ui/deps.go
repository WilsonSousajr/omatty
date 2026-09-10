// Everything a Model is given, and the defaults that mean no method needs a
// nil guard (#76). Split from model.go when M7's config, naming and agent
// dependencies took the struct past the file limit on its own.

package ui

import (
	"time"

	"github.com/WilsonSousajr/omatty/internal/notify"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// CreateFunc registers a new session in project and returns it. branch is
// empty for a session on the project's main checkout.
type CreateFunc func(project, title, branch string) (registry.Session, error)

// StartFunc launches the embedded terminal for a session at w by h. Injected
// so the model can start a session created at runtime without knowing how;
// the size is a parameter so it is never frozen at startup (issue #73).
type StartFunc func(sess registry.Session, w, h int) (termwrap.Terminal, error)

// RepoStatFunc reads a session's branch and diffstat for its sidebar card
// (#180). Injected so ui never touches git (invariant 4). Nil is the switch,
// as ModelName's is: with nothing wired the card draws its lane alone, which
// is what every test's Deps gets.
type RepoStatFunc func(sess registry.Session, projectRoot string) (review.Stat, error)

// Deps is everything a Model needs. Constructor injection, so no field is
// set after the fact and no method needs a nil guard (issue #76). The zero
// value of an optional field means: no status stream, the wall clock, a
// silent notifier, no tailer for runtime sessions.
//
//	m := ui.NewModel(ui.Deps{State: st, Terms: terms, Create: create, Start: start,
//	        Events: w.Events(), Clock: time.Now, Notifier: notify.New(), TailStart: w.Add})
type Deps struct {
	State     registry.State
	Terms     map[string]termwrap.Terminal
	Create    CreateFunc
	Start     StartFunc
	Events    <-chan watcher.Event
	Clock     func() time.Time
	Notifier  notify.Notifier
	TailStart func(registry.Session)
	// Diff loads a session's changes for the review column (#21).
	Diff DiffFunc
	// Files lists a session's worktree and Preview reads one of its files,
	// for the tree view (#24).
	Files   ListFilesFunc
	Preview PreviewFunc
	// Stat reads a session's branch and diffstat for its card; nil means no
	// git to ask (#180).
	Stat RepoStatFunc
	// Rename persists a session's new title (#41), and Name reads the first
	// prompt that titles a session created without one (#127).
	Rename RenameFunc
	Name   NameFunc
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
	// Reattached names the sessions whose claude was already running when
	// omatty started: their panes come back blank and are asked to repaint
	// once at boot (#191). Nil means none.
	Reattached map[string]bool
}

// withDefaults fills the optional fields: the wall clock and a silent
// notifier, so no method needs a nil guard.
func (d Deps) withDefaults() Deps {
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Notifier == nil {
		d.Notifier = notify.Silent{}
	}
	if d.Leader == "" {
		d.Leader = DefaultLeader
	}
	return d.withReviewDefaults().withLifecycleDefaults()
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
	if d.Rename == nil {
		d.Rename = noRename
	}
	if d.Name == nil {
		d.Name = noName
	}
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
