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

// StartTerminals launches an embedded terminal for each session in want,
// keyed by session id. w and h are the WINDOW size; each PTY is born at the
// pane size, so claude paints at the right width from its first frame instead
// of racing a later resize (issue #51).
//
//	terms := ui.StartTerminals(st, sessionsToStart(d, held), l, termwrap.Start, w, h, leader)
//
// A session left out - not wanted, or failing to start - has no terminal,
// which its pane shows as stopped with enter to start it (#318). One failed
// start used to abort the whole boot, so a single bad session kept every
// other one from opening; it is logged and skipped instead (#317).
func StartTerminals(
	st registry.State, want map[string]bool, l *supervisor.Launcher, f termwrap.Factory, w, h int, leader string,
) map[string]termwrap.Terminal {
	// The review column is closed at birth, so the terminal gets the full
	// width beside the sidebar (#21).
	pw, ph := PTYSize(w, h, false)
	terms := make(map[string]termwrap.Terminal, len(want))
	for _, sess := range st.Sessions {
		if !want[sess.ID] {
			continue
		}
		term, err := l.Start(f, sess, pw, ph)
		if err != nil {
			slog.Warn("starting a session's terminal at boot; its pane shows it stopped",
				"session", sess.ID, "dir", sess.Dir, "err", err)
			continue
		}
		// Invariant 6: one emulator panic must not take down the app.
		terms[sess.ID] = termwrap.NewGuard(term, leader+" r")
	}
	return terms
}

// sessionsToStart is the boot policy in one place (#317). Under lazy start it
// is exactly the sessions a holder is already keeping alive: attaching one
// costs a dtach client and a PTY, while not attaching frees nothing - its
// claude runs on either way - and would leave #191's repaint nudge with no
// terminal to repaint. Every other session waits for enter. Without dtach
// nothing is held, so a lazy boot starts none, which is right: nothing
// survived the quit, so each was going to cold-start anyway, and now does so
// on demand. lazy_start = false starts every session, as before.
func sessionsToStart(d RunDeps, held map[string]bool) map[string]bool {
	if d.LazyStart {
		return held
	}
	all := make(map[string]bool, len(d.State.Sessions))
	for _, sess := range d.State.Sessions {
		all[sess.ID] = true
	}
	return all
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
	// Tally records one gate run that followed a turn (#332).
	Tally TallyFunc
	// Stat reads a session's branch and diffstat for its card (#180).
	Stat RepoStatFunc
	// Turn reaches a session's turn baseline (#311).
	Turn TurnFuncs
	// PRs lists a project's pull requests for its cards (#310) and Issues its
	// open issues for the tracker (#394).
	PRs    PRListFunc
	Issues IssueListFunc
	// Item reads one issue or pull request in full (#397), and Browse opens one
	// in the operator's browser (#398).
	Item   ForgeItemFuncs
	Browse BrowseFunc
	// Rename persists a session's new title (#41); Name reads the first prompt
	// that titles a session created without one (#127).
	Rename RenameFunc
	// Rebind persists the conversation a /clear moved a session to (#316).
	Rebind RebindFunc
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
	// IdleStop stops a session quiet this long; zero is off (#319).
	IdleStop time.Duration
	// LazyStart boots only the sessions a holder already keeps alive; the
	// rest wait for enter. On unless [sessions] lazy_start = false (#317).
	LazyStart bool
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
	terms := StartTerminals(d.State, sessionsToStart(d, held), d.Launch, d.Factory, d.Width, d.Height, d.Leader)
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
		Diff: d.Diff, Files: d.Files, Tally: d.Tally, Stat: d.Stat, Turn: d.Turn, PRs: d.PRs, Issues: d.Issues, Item: d.Item, Browse: d.Browse, Rename: d.Rename, Rebind: d.Rebind, RenameBranch: d.RenameBranch, Name: d.Name, ModelName: d.ModelName,
		Archive: d.Archive, RemoveWorktree: d.RemoveWorktree, RemoveProject: d.RemoveProject,
		Discover: d.Discover, AddProject: d.AddProject,
		AdoptPropose: d.AdoptPropose, AdoptCommit: d.AdoptCommit,
		Stop: d.Stop, Notice: d.Notice, Leader: d.Leader, Reattached: held,
		Events: watch.Events(), HooksDown: !watch.HooksLive(), Clock: time.Now, Notifier: notify.New(),
		TailStart: watch.Add, TailStop: watch.Remove,
		GateReports: gates.Reports(), GateRun: gates.Start, GateAuto: d.GateAuto,
		IdleStop: d.IdleStop,
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
