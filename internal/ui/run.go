package ui

import (
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/agent"
	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/notify"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// StartTerminals launches one embedded terminal per registered session, keyed
// by session id. w and h are the WINDOW size; each PTY is born at the pane
// size, so claude paints at the right width from its first frame instead of
// racing a later resize (issue #51).
func StartTerminals(
	st registry.State, l *supervisor.Launcher, f termwrap.Factory, w, h int, leader string,
) (map[string]termwrap.Terminal, error) {
	// The review column is closed at birth, so the terminal gets the full
	// width beside the sidebar (#21).
	pw, ph := PTYSize(w, h, false)
	terms := make(map[string]termwrap.Terminal, len(st.Sessions))
	for _, sess := range st.Sessions {
		term, err := l.Start(f, sess, pw, ph)
		if err != nil {
			return nil, fmt.Errorf("ui: starting terminal for session %s: %w", sess.ID, err)
		}
		// Invariant 6: one emulator panic must not take down the app.
		terms[sess.ID] = termwrap.NewGuard(term, leader+" r")
	}
	return terms, nil
}

// HeldSessions is the set of sessions whose claude is already running from
// an earlier omatty, so the boot can tell a pane that will come back blank
// from one that will paint itself (#191). A failed liveness check is logged
// and reads as fresh: the pane then behaves as it did before this existed.
//
//	held := ui.HeldSessions(launcher, state)
func HeldSessions(l *supervisor.Launcher, st registry.State) map[string]bool {
	held := map[string]bool{}
	for _, sess := range st.Sessions {
		ok, err := l.Reattaching(sess.ID)
		if err != nil {
			slog.Warn("checking whether a session is held", "session", sess.ID, "err", err)
			continue
		}
		if ok {
			held[sess.ID] = true
		}
	}
	return held
}

// RunDeps is everything Run needs: the runtime plumbing, plus the functions
// injected so ui never reaches git or the registry store itself (invariant 4).
//
// A struct rather than a parameter list because M4 adds one injected function
// per feature - rename here (#41), archive and discovery after it - and the
// list was already at eight (#41).
//
//	ui.Run(ui.RunDeps{Home: home, State: st, Launch: l, Factory: termwrap.Start,
//	        Width: w, Height: h, Create: create, Diff: diff, Files: files, Rename: rename})
type RunDeps struct {
	Home    string
	State   registry.State
	Launch  *supervisor.Launcher
	Factory termwrap.Factory
	Width   int
	Height  int
	// Create is called when the operator finishes a new-session prompt; the
	// model starts that session's terminal itself through the same launcher.
	Create CreateFunc
	// Diff loads a session's changes for the review column and Files lists its
	// worktree for the tree (#21, #24).
	Diff  DiffFunc
	Files ListFilesFunc
	// Stat reads a session's branch and diffstat for its card (#180).
	Stat RepoStatFunc
	// Rename persists a session's new title (#41); Name reads the first prompt
	// that titles a session created without one (#127).
	Rename RenameFunc
	// RenameBranch names a worktree's branch from its first prompt (#151).
	RenameBranch BranchRenameFunc
	Name         NameFunc
	// ModelName is the opt-in headless naming call, nil when off (#127).
	ModelName ModelNameFunc
	// Archive drops a session from the registry and RemoveWorktree deletes its
	// worktree (#40). The tailer is stopped through the Watch this owns.
	Archive        ArchiveFunc
	RemoveWorktree RemoveWorktreeFunc
	RemoveProject  RemoveProjectFunc
	// Discover proposes repositories to register and AddProject registers one
	// (#91).
	Discover   DiscoverFunc
	AddProject AddProjectFunc
	// AdoptPropose lists a project's adoptable claude sessions and AdoptCommit
	// registers the chosen ones (#122).
	AdoptPropose AdoptFunc
	AdoptCommit  AdoptCommitFunc
	// Stop ends an archived session's held claude, and Notice says once at
	// startup when no holder is keeping them (#43).
	Stop   StopFunc
	Notice string
	// Leader is the configured leader key; empty means DefaultLeader (#44).
	Leader string
	// Agent is the profile every session runs: its status adapter and its
	// transcript location feed the watcher (#46). The zero value is claude.
	Agent agent.Profile
	// GateParallel bounds how many gates run at once (#229). Zero is raised
	// to one by the Runner, so an unset config is a working default.
	GateParallel int
	// GateAuto runs a session's gate when its turn ends (#233). Off unless
	// the config asks for it.
	GateAuto bool
}

// Run starts every session's terminal, the status watcher, and the TUI, and
// runs until the user quits.
func Run(d RunDeps) error {
	d.Leader = leaderOr(d.Leader)
	// Asked before the terminals start: once a client is attached the socket
	// exists whether or not a claude was already behind it (#191).
	held := HeldSessions(d.Launch, d.State)
	terms, err := StartTerminals(d.State, d.Launch, d.Factory, d.Width, d.Height, d.Leader)
	if err != nil {
		return err
	}
	defer closeTerminals(terms)
	watch := watcher.Start(watchDeps(d), d.State.Sessions)
	defer watch.Close()
	// One Runner for the whole app, bounded: four concurrent `go test -race`
	// would make the machine unusable, and a laggy TUI is the one thing that
	// would make the gate worse than running it by hand (#229).
	gates := gate.NewRunner(d.GateParallel)
	defer gates.Close()
	return runProgram(modelFor(d, terms, held, watch, gates), len(terms))
}

// modelFor assembles the root model's dependencies. Split out of Run because
// the assembly is one long literal and Run is the lifecycle around it - the
// terminals, the watcher and the gate Runner, each with its own defer.
func modelFor(
	d RunDeps, terms map[string]termwrap.Terminal, held map[string]bool,
	watch *watcher.Watch, gates *gate.Runner,
) *Model {
	return NewModel(Deps{
		State: d.State, Terms: terms, Create: d.Create, Start: guardedStarter(d.Launch, d.Factory, d.Leader),
		Diff: d.Diff, Files: d.Files, Stat: d.Stat, Rename: d.Rename, RenameBranch: d.RenameBranch, Name: d.Name, ModelName: d.ModelName,
		Archive: d.Archive, RemoveWorktree: d.RemoveWorktree, RemoveProject: d.RemoveProject,
		Discover: d.Discover, AddProject: d.AddProject,
		AdoptPropose: d.AdoptPropose, AdoptCommit: d.AdoptCommit,
		Stop: d.Stop, Notice: d.Notice, Leader: d.Leader, Reattached: held,
		Events: watch.Events(), Clock: time.Now, Notifier: notify.New(),
		TailStart: watch.Add, TailStop: watch.Remove,
		GateReports: gates.Reports(), GateRun: gates.Start, GateAuto: d.GateAuto,
	})
}

// watchDeps is the watcher's slice of the agent profile: its status adapter
// and its transcript location. An unset profile is claude (#46).
func watchDeps(d RunDeps) watcher.WatchDeps {
	profile := d.Agent
	if profile.Status == nil {
		profile = agent.Claude()
	}
	return watcher.WatchDeps{Home: d.Home, Clock: time.Now,
		Adapter: profile.Status, TranscriptPath: profile.TranscriptPath}
}

// leaderOr is the configured leader, or DefaultLeader for an empty one (#44).
func leaderOr(leader string) string {
	if leader == "" {
		return DefaultLeader
	}
	return leader
}

// closeTerminals closes every PTY on the way out (issue #72). The map is the
// one the model adds runtime sessions to, so those close too. Until #72 the OS
// closed the masters at exit, which is neither a guarantee nor omatty's
// decision.
//
// What this ends depends on the holder, and the distinction is the whole point
// of M6: under Plain it closes the PTY and the claude on the other side of it
// dies with the SIGHUP, exactly as before; under a detach holder it closes only
// the dtach *client*, and the master and its claude go on running. Archiving is
// the one place omatty ends a claude on purpose, and dropSession is where that
// is written (#43).
func closeTerminals(terms map[string]termwrap.Terminal) {
	for id, t := range terms {
		if err := t.Close(); err != nil {
			slog.Warn("closing a terminal on exit", "session", id, "err", err)
		}
	}
}

// runProgram runs the bubbletea program to completion.
func runProgram(model *Model, sessions int) error {
	if _, err := tea.NewProgram(model).Run(); err != nil {
		return fmt.Errorf("ui: running the program with %d sessions: %w", sessions, err)
	}
	return nil
}

// guardedStarter starts a session's terminal wrapped in a panic guard
// (invariant 6). The model passes the live pane size on every call.
func guardedStarter(l *supervisor.Launcher, f termwrap.Factory, leader string) StartFunc {
	return func(sess registry.Session, w, h int) (termwrap.Terminal, error) {
		term, err := l.Start(f, sess, w, h)
		if err != nil {
			return nil, err
		}
		return termwrap.NewGuard(term, leader+" r"), nil
	}
}
