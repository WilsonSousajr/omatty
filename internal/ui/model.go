package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/domain/coverage"
	"github.com/WilsonSousajr/omatty/internal/domain/gate"
	"github.com/WilsonSousajr/omatty/internal/infra/forge"
	"github.com/WilsonSousajr/omatty/internal/infra/notify"
	"github.com/WilsonSousajr/omatty/internal/keys"
	"github.com/WilsonSousajr/omatty/internal/pubsub"
	"github.com/WilsonSousajr/omatty/internal/review"
	sgate "github.com/WilsonSousajr/omatty/internal/service/gate"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/service/status"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// DefaultLeader is the key omatty intercepts while the terminal has focus,
// unless the config file names another (#44). It stays a constant because
// keys.NewRouter, the help gutter and the footers all need a value before
// any Deps is read.
const DefaultLeader = "ctrl+o"

// Model is omatty's root Bubble Tea model.
//
//	m := ui.NewModel(ui.Deps{State: state, Terms: terms, Create: create, Start: start})
//	tea.NewProgram(m).Run()
type Model struct {
	// state is held so a session created at runtime can be folded in and the
	// sidebar rebuilt (issue #32).
	state   sessions.State
	sidebar *Sidebar
	terms   map[string]termwrap.Terminal
	router  *keys.Router
	leader  string // the key the router intercepts; DefaultLeader unless configured (#44)
	create  CreateFunc
	start   StartFunc
	// starting is every session whose process is being started off the
	// Update goroutine, so a second enter while one is on its way does not
	// start another (migration step 5.6a, #653). Nil until the first start.
	starting map[string]bool
	// status is the live per-session state from the watcher; events feeds it.
	status    map[string]status.SessionState
	events    <-chan pubsub.Event[status.Event]
	clock     func() time.Time
	tailStart func(sessions.Session)
	notifier  notify.Notifier
	// notified is when each session last posted a notification (issue #69).
	notified map[string]time.Time
	// startedAt gates notifications to transitions newer than this run: the
	// first tailer poll replays old turns (issue #70).
	startedAt time.Time
	hasFocus  bool
	// modal is the surface that currently owns the keyboard, if any: the
	// new-session prompt, the rename box (#41) or the archive confirmation
	// (#40).
	modal  modal
	review ReviewPane
	// frameMemo is the last frame and the inputs it was built from, and
	// paneOnly records that the message just handled could not have touched
	// any of them. See frame().
	frameMemo frameCache
	paneOnly  bool
	// comments is each session's pending review queue, kept across opening and
	// closing the column; only submit drains it (#22).
	comments    map[string]*review.Comments
	diff        DiffFunc
	files       ListFilesFunc
	generatedFn GeneratedFunc
	ship        ShipFuncs
	tally       TallyFunc
	preview     PreviewFunc
	rename      RenameFunc
	rebind      RebindFunc // follows a /clear onto its new conversation (#316)
	// renameBranch renames a worktree session's branch once its first prompt
	// has said what the work is (#151).
	renameBranch BranchRenameFunc
	name         NameFunc
	// modelNamer is nil unless the config opted in to a headless naming call
	// (#127).
	modelNamer ModelNameFunc
	// namePending guards one in-flight name read per session (#127). Not
	// persisted, and correctly empty after a relaunch: whether a session
	// still needs a name is derived from its title, not from this map.
	namePending map[string]bool
	// gates is each session's last gate report, display-only like repoStat
	// and never persisted: state.json must suffice alone
	// (invariant 9). Absent means no gate has run, which the card shows as a
	// blank line rather than as a pass (#230).
	gates map[string]gate.Report
	// gateRunning is the sessions with a run in flight, so the pane says so
	// rather than showing the last verdict as if it were current.
	gateRunning map[string]bool
	// gateSent is how far S has got with each session's current report, so a
	// second S warns before sending the same failures again (#335). A new
	// report replaces the entry's meaning, so onGate deletes it.
	gateSent map[string]gateSend
	// turn reaches the turn baselines (#311). turnPending holds a session
	// whose snapshot is in flight, so a second prompt inside it starts none;
	// turnErr is the last snapshot's failure, shown instead of a turn diff
	// that would silently span two turns. Neither is persisted.
	turn      TurnFuncs
	hooksDown bool // the hook socket did not bind (#49): no baseline will come
	// prs is each project's pull requests (#310) and issues its open issues
	// (#394), keyed by project name like their Pending (a call in flight),
	// Failed (the last call failed) and Asked (when it was last asked, for the
	// gap) maps. forgeStopped (the forge's tool is missing, or the checkout is
	// on no forge omatty reads) is shared by both lists: it is a fact about the
	// checkout and the machine, not about a list (#449). labelOf names each
	// project's forge for the copy. None persisted.
	prList       PRListFunc
	prs          map[string][]forge.PR
	prPending    map[string]bool
	prFailed     map[string]bool
	prAsked      map[string]time.Time
	issueList    IssueListFunc
	itemFuncs    ForgeItemFuncs
	browse       ForgeBrowseFuncs
	items        map[itemKey]forge.Detail
	itemPending  map[itemKey]bool
	itemFailed   map[itemKey]bool
	issues       map[string][]forge.Issue
	issuePending map[string]bool
	issueFailed  map[string]bool
	issueAsked   map[string]time.Time
	forgeStopped map[string]error
	noTracker    map[string]bool // the forge keeps no issues for it (#460)
	labelOf      LabelFunc
	turnPending  map[string]bool
	turnErr      map[string]error
	// covers is each session's coverage overlay, read when its gate finishes
	// (#254). Display-only like gates and never persisted; coverFailed makes
	// the warning once per session rather than once per run.
	covers      map[string]coverage.Profile
	coverFailed map[string]bool
	// gateReports and gateRun are the gate's two halves, shaped like the
	// watcher's: a channel of results in, a request out. Concrete types stay
	// out of the model so a test substitutes a recorder for the Runner.
	gateReports  <-chan pubsub.Event[gate.Report]
	gateRun      GateRunFunc
	profiles     sgate.ProfileReader // reads a gate's coverage profile (#254, #653)
	gateAuto     bool
	gateStarted  map[string]time.Time // when the run in flight began, for its title (#428)
	glyphs       glyphSet             // every state mark, plain or Nerd Font (#425)
	previewArmed itemKey              // the row the tracker's preview is waiting to rest on (#434)
	nerdIcons    bool                 // [ui] icons = "nerd": file-type icons in the tree too (#431)
	// spinArmed is whether a spin tick is pending, so there is one spin
	// chain at most however many sessions start working; spinTick schedules
	// it (#412).
	spinArmed bool
	spinTick  TickFunc
	// stat reads a card's branch and diffstat; repoStat is the last answer per
	// session, display-only and never persisted - state.json
	// must suffice alone (invariant 9). statPending guards one poll in flight
	// per session; statFailed makes the warning once per outage (#180).
	stat        RepoStatFunc
	repoStat    map[string]review.Stat
	statPending map[string]bool
	statFailed  map[string]bool
	// filesPending guards one worktree listing in flight per session (#195).
	filesPending map[string]bool
	// reviewed is, per session, the digest each file's diff had when the
	// operator marked it read (#337). Keyed by session because the column
	// keeps one Tree: a mark stored on the Tree would be dropped the moment
	// they looked at another session, which is the review this exists to
	// save. Display-only and never persisted, like covers and repoStat -
	// state.json must suffice to relaunch a session (invariant 9), and what
	// somebody has read is not part of that.
	reviewed map[string]map[string]string
	// generated is, per session, which of its files nobody wrote (#338).
	// Display-only and never persisted, like reviewed above it.
	generated map[string]map[string]bool
	// turnGated marks the sessions whose gate run was started by a turn
	// ending rather than by hand, which is the set #332's rate is over.
	turnGated map[string]bool
	// reattached is Deps.Reattached: the panes to nudge once at boot (#191).
	reattached map[string]bool
	// The archive path's three halves: forget the session, stop its tailer,
	// and optionally delete its worktree (#40).
	archive          ArchiveFunc
	removeWorktree   RemoveWorktreeFunc
	removeProject    RemoveProjectFunc // forgets an empty project (#159)
	fold             FoldFunc          // persists a project's fold (#505)
	tailStop         func(sessionID string)
	discover         DiscoverFunc
	registerProjects AddProjectFunc
	adoptPropose     AdoptFunc
	adoptCommit      AdoptCommitFunc
	// adoptable is the last adoption scan's proposals, kept so committing a
	// marked row resolves back to the proposal it came from - a pickItem
	// carries a label and a detail, not a working directory (#122).
	adoptable []SessionProposal
	// scanToken numbers discovery scans so a stale result cannot overwrite a
	// newer picker (#91).
	scanToken int
	// diffSeq and turnSeq number the diff loads, so an answer that arrives
	// after a newer load's cannot paint over it (#352). On the model, not the
	// pane: the pane is rebuilt when the session changes, and a per-pane count
	// restarting at zero could match an old load after A, B, A. Two, because
	// loadDiff starts both at once.
	diffSeq uint64
	turnSeq uint64
	lastErr string
	// stop ends an archived session's held claude (#43).
	stop StopFunc
	// notice is the startup line, cleared by the first keypress the way
	// lastErr is: the keymap it displaces is worth more than a warning already
	// read (#43).
	notice string
	// wheel counts scroll notches so a momentum flick becomes a few pages of
	// transcript rather than tens of them (#107).
	wheel wheelAccumulator
	// mouseReleased is true while the host terminal owns the pointer, so it
	// can make a selection of its own (#217). Display-only, never persisted.
	mouseReleased bool
	// sel is the drag in progress over the session pane, which a release
	// copies to the host clipboard (#360).
	sel    paneSelection
	width  int
	height int
	// idleStop and activeAt are the idle sweep (#319): its threshold, zero
	// when off, and when omatty last started or the operator last typed into
	// each session - the floors under the transcript's own last turn.
	idleStop time.Duration
	activeAt map[string]time.Time
}

// NewModel builds the root model from its dependencies.
func NewModel(deps Deps) *Model {
	d := deps.withDefaults()
	m := &Model{
		state:      d.State,
		sidebar:    NewSidebar(SidebarRows(d.State, nil)),
		terms:      d.Terms,
		router:     keys.NewRouter(d.Leader),
		leader:     d.Leader,
		create:     d.Create,
		start:      d.Start,
		events:     d.Events,
		clock:      d.Clock,
		spinTick:   d.SpinTick,
		tailStart:  d.TailStart,
		notifier:   d.Notifier,
		startedAt:  d.Clock(),
		hasFocus:   true,
		reattached: d.Reattached,
	}
	m.glyphs, m.nerdIcons = glyphsFor(d.NerdIcons), d.NerdIcons
	return m.withSources(d).withGate(d).withWindow().withRuntimeMaps().withSweep(d)
}

// withSources attaches the injected functions that reach outside ui: the
// review column's readers (#21, #24) and the lifecycle commands (#40, #41).
func (m *Model) withSources(d Deps) *Model {
	m.diff, m.files, m.preview = d.Diff, d.Files, d.Preview
	m.generatedFn, m.ship, m.tally = d.Generated, d.Ship, d.Tally
	m.turn, m.hooksDown = d.Turn, d.HooksDown
	m.prList, m.issueList, m.itemFuncs, m.browse, m.labelOf = d.PRs, d.Issues, d.Item, d.Browse, d.Label
	m.rename, m.name, m.archive = d.Rename, d.Name, d.Archive
	m.rebind = d.Rebind
	m.renameBranch = d.RenameBranch
	m.modelNamer = d.ModelName
	m.removeWorktree, m.tailStop = d.RemoveWorktree, d.TailStop
	m.removeProject, m.fold = d.RemoveProject, d.Fold
	m.discover, m.registerProjects = d.Discover, d.AddProject
	m.adoptPropose, m.adoptCommit = d.AdoptPropose, d.AdoptCommit
	m.stop, m.notice = d.Stop, d.Notice
	m.stat = d.Stat
	return m
}

// withWindow lays the first frame out for a conventional terminal rather than
// a 0x0 one: before the first WindowSizeMsg, and off a tty entirely,
// bubbletea reports zero (issue #74).
func (m *Model) withWindow() *Model {
	m.width, m.height = DefaultWidth, DefaultHeight
	return m
}

// withGate attaches the gate's two halves and whether it runs itself. Its own
// builder rather than three more lines in NewModel, which was already at the
// length limit (#233) - and the three belong together: a Runner with no
// reports channel, or auto-run with no Runner, would each be a half-wiring.
func (m *Model) withGate(d Deps) *Model {
	m.gateReports, m.gateRun, m.gateAuto = d.GateReports, d.GateRun, d.GateAuto
	m.profiles = d.Profiles
	m.gateStarted = map[string]time.Time{}
	return m
}

// withRuntimeMaps allocates the per-session maps the model fills as it runs:
// live status, notification times, and each session's queued review comments.
// They are never nil, so no method needs a nil guard (issue #76).
func (m *Model) withRuntimeMaps() *Model {
	m.status = map[string]status.SessionState{}
	m.notified = map[string]time.Time{}
	m.comments = map[string]*review.Comments{}
	m.namePending = map[string]bool{}
	m.gates = map[string]gate.Report{}
	m.gateRunning = map[string]bool{}
	m.gateSent = map[string]gateSend{}
	m.covers = map[string]coverage.Profile{}
	m.coverFailed = map[string]bool{}
	m.repoStat = map[string]review.Stat{}
	m.statPending = map[string]bool{}
	m.statFailed = map[string]bool{}
	m.filesPending = map[string]bool{}
	m.turnGated = map[string]bool{}
	return m.withReviewMaps().withTurnMaps().withPRMaps().withIssueMaps().withItemMaps()
}

// withReviewMaps allocates what the review column remembers per session that
// nothing else does: which files have been read (#337) and which of them nobody
// wrote (#338). Split from withRuntimeMaps when they took it past the statement
// limit, the way withTurnMaps was.
func (m *Model) withReviewMaps() *Model {
	m.reviewed = map[string]map[string]string{}
	m.generated = map[string]map[string]bool{}
	return m
}

// withTurnMaps allocates the turn baseline's two maps (#311). Split from
// withRuntimeMaps when they took it past the statement limit.
func (m *Model) withTurnMaps() *Model {
	m.turnPending = map[string]bool{}
	m.turnErr = map[string]error{}
	return m
}

// Selected returns the selected session's id, or "" when none is selected.
//
// Selected, not Focused: it answers where the sidebar cursor is, which #95
// separated from who owns the keyboard. The old name put it on the wrong side
// of that split - selectedTerminal was implemented by calling Focused() - and
// left "focus" spanning six unrelated concepts across the package, where
// AGENTS.md asks a name to return fewer than five grep hits.
//
//	if id := m.Selected(); id != "" { ... }
func (m *Model) Selected() string {
	row, ok := m.sidebar.Selected()
	if !ok {
		return ""
	}
	return row.Session.ID
}

// SelectedProject returns the project the cursor is in: the selected
// session's, or the empty project whose header is selected (#158). With no
// project registered it is "".
func (m *Model) SelectedProject() string { return m.sidebar.CursorProject() }

// Init starts every session's terminal reading from its PTY.
//
// bubbleterm.Init returns a self-rescheduling blocking poll; without it no
// terminal ever reads anything and every pane stays blank (issue #33).
func (m *Model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, 2*len(m.terms)+2)
	for id, term := range m.terms {
		if cmd := term.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
		// A pane's copies are waited on from the start, like its output
		// (#212). Sessions restored from state.json get theirs here; the
		// two sites that add a session later arm their own.
		if cmd := m.waitForClipboard(id); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	cmds = append(cmds, m.repaintHeld()...)
	if cmd := m.waitForEvent(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.waitForGate(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	// The first stat poll runs at start rather than a tick later, so a card
	// names its branch before the operator has read the screen (#180).
	cmds = append(cmds, scheduleTick(), m.onStatTick(), m.onPRTick(), m.onIssueTick(), m.scheduleSweep())
	return tea.Batch(cmds...)
}

// repaintHeld nudges every pane that came back from a dtach-held claude: the
// attach cleared it and a same-size SIGWINCH got nothing back, so the pane
// stayed blank until the session next wrote something and the operator was
// pressing ctrl+o r on every session, every restart (#191). A fresh claude
// paints on its own and is left alone.
func (m *Model) repaintHeld() []tea.Cmd {
	var cmds []tea.Cmd
	for id := range m.reattached {
		if term := m.terms[id]; term != nil {
			cmds = append(cmds, term.Repaint())
		}
	}
	return cmds
}
