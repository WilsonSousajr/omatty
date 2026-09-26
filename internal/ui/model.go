package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/coverage"
	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/keys"
	"github.com/WilsonSousajr/omatty/internal/notify"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/watcher"
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
	state   registry.State
	sidebar *Sidebar
	terms   map[string]termwrap.Terminal
	router  *keys.Router
	leader  string // the key the router intercepts; DefaultLeader unless configured (#44)
	create  CreateFunc
	start   StartFunc
	// status is the live per-session state from the watcher; events feeds it.
	status    map[string]watcher.SessionState
	events    <-chan watcher.Event
	clock     func() time.Time
	tailStart func(registry.Session)
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
	// gap) maps. notGitHub (gh cannot map the checkout to GitHub) and ghMissing
	// (no gh at all) are shared by both lists: they are facts about the
	// checkout and the machine, not about a list. None persisted.
	prList       PRListFunc
	prs          map[string][]forge.PR
	prPending    map[string]bool
	prFailed     map[string]bool
	prAsked      map[string]time.Time
	issueList    IssueListFunc
	itemFuncs    ForgeItemFuncs
	browse       BrowseFunc
	items        map[itemKey]forge.Detail
	itemPending  map[itemKey]bool
	itemFailed   map[itemKey]bool
	issues       map[string][]forge.Issue
	issuePending map[string]bool
	issueFailed  map[string]bool
	issueAsked   map[string]time.Time
	notGitHub    map[string]bool
	ghMissing    bool
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
	gateReports <-chan gate.Report
	gateRun     GateRunFunc
	gateAuto    bool
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
	// reattached is Deps.Reattached: the panes to nudge once at boot (#191).
	reattached map[string]bool
	// The archive path's three halves: forget the session, stop its tailer,
	// and optionally delete its worktree (#40).
	archive          ArchiveFunc
	removeWorktree   RemoveWorktreeFunc
	removeProject    RemoveProjectFunc // forgets an empty project (#159)
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
	return m.withSources(d).withGate(d).withWindow().withRuntimeMaps().withSweep(d)
}

// withSources attaches the injected functions that reach outside ui: the
// review column's readers (#21, #24) and the lifecycle commands (#40, #41).
func (m *Model) withSources(d Deps) *Model {
	m.diff, m.files, m.preview = d.Diff, d.Files, d.Preview
	m.generatedFn = d.Generated
	m.turn, m.hooksDown = d.Turn, d.HooksDown
	m.prList, m.issueList, m.itemFuncs, m.browse = d.PRs, d.Issues, d.Item, d.Browse
	m.rename, m.name, m.archive = d.Rename, d.Name, d.Archive
	m.rebind = d.Rebind
	m.renameBranch = d.RenameBranch
	m.modelNamer = d.ModelName
	m.removeWorktree, m.tailStop = d.RemoveWorktree, d.TailStop
	m.removeProject = d.RemoveProject
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
	return m
}

// withRuntimeMaps allocates the per-session maps the model fills as it runs:
// live status, notification times, and each session's queued review comments.
// They are never nil, so no method needs a nil guard (issue #76).
func (m *Model) withRuntimeMaps() *Model {
	m.status = map[string]watcher.SessionState{}
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

// Update routes messages to one handler per type, so it stays a router and
// stays inside the 20-line function limit.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Any message may change what is on screen, so the memoised frame is
	// dropped unless the message took the one path that provably cannot
	// touch model state - onWindowFocus's broadcast, which sets paneOnly.
	// Invalidating by default is what keeps a message type added later from
	// silently leaving a stale screen behind: the cost of forgetting is a
	// rebuild, never a lie.
	m.paneOnly = false
	cmd := tea.Batch(m.routeMsg(msg), m.armSpin())
	if !m.paneOnly {
		m.frameMemo.valid = false
	}
	return m, cmd
}

// routeMsg hands one message to the table that owns it.
func (m *Model) routeMsg(msg tea.Msg) tea.Cmd {
	if cmd, ok := m.onInput(msg); ok {
		return cmd
	}
	if cmd, ok := m.onHeartbeat(msg); ok {
		return cmd
	}
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		return m.onResize(size)
	}
	return m.onDataMsg(msg)
}

// onHeartbeat answers the periodic ticks, and says whether it did, so routeMsg
// stays a short list of tables rather than growing a case per timer - #394
// added the fifth.
func (m *Model) onHeartbeat(msg tea.Msg) (tea.Cmd, bool) {
	switch msg.(type) {
	case TickMsg:
		return scheduleTick(), true
	case SpinTickMsg:
		return m.onSpinTick(), true
	case StatTickMsg:
		return m.onStatTick(), true
	case PRTickMsg:
		return m.onPRTick(), true
	case IssueTickMsg:
		return m.onIssueTick(), true
	case SweepTickMsg:
		return m.onSweepTick(), true
	}
	return nil, false
}

// onInput routes what the operator did - a key, the mouse, a paste - and
// reports whether msg was one. Everything else Update sees is what the
// program did. The host's paste brackets arrive as messages of their own and
// are not text: onPaste re-brackets the content itself (#190, invariant 8).
func (m *Model) onInput(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.onKey(msg), true
	case tea.MouseMsg:
		return m.onMouse(msg), true
	case tea.PasteMsg:
		return m.onPaste(msg), true
	case tea.PasteStartMsg, tea.PasteEndMsg:
		return nil, true
	}
	return nil, false
}

// onDataMsg handles the results of work that ran off the Update goroutine, then
// falls through to onSessionMsg and, past that, window focus and the broadcast.
//
// Named switches rather than a default arm that is really a second, unnamed
// one. Every case must be matched by type, because anything unmatched reaches
// broadcast and is fanned out to every emulator at once - so the next person
// adding a message type has to see these lists, not discover them. The mouse
// has its own case in Update for the same reason: a pointer event carries
// window coordinates that mean nothing to an individual pane (#40, #107).
//
// The split is paneCommand's: one table ran past the length limit, and the
// ones after it are named here so they still read as one list (#122).
func (m *Model) onDataMsg(msg tea.Msg) tea.Cmd {
	if cmd, handled := m.onStreamMsg(msg); handled {
		return cmd
	}
	switch typed := msg.(type) {
	case DiffLoadedMsg:
		return m.onDiffLoaded(typed)
	case FilesLoadedMsg:
		return m.onFilesLoaded(typed)
	case WorktreeRemovedMsg:
		return m.onWorktreeRemoved(typed)
	}
	if cmd, handled := m.onNamingMsg(msg); handled {
		return cmd
	}
	if cmd, handled := m.onTurnMsg(msg); handled {
		return cmd
	}
	return m.onPaneMsg(msg)
}

// onTurnMsg answers the turn baseline's two results: a snapshot taken, and a
// turn diff loaded (#311). A table of its own, as onNamingMsg is, because the
// two cases took onDataMsg past the length limit.
func (m *Model) onTurnMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch typed := msg.(type) {
	case TurnLoadedMsg:
		return m.onTurnLoaded(typed), true
	case TurnSnappedMsg:
		return m.onTurnSnapped(typed), true
	}
	return nil, false
}

// onNamingMsg answers what names a session: its first prompt, the model's
// improvement on it, and the branch that takes the result (#127, #151). A
// table of its own because onDataMsg ran past the length limit with them in
// it, and they are one conversation.
func (m *Model) onNamingMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch typed := msg.(type) {
	case NamedMsg:
		return m.onNamed(typed), true
	case ModelNamedMsg:
		return m.onModelNamed(typed), true
	case BranchNamedMsg:
		return m.onBranchNamed(typed), true
	}
	return nil, false
}

// onStreamMsg is the messages fed by a long-lived source over a channel: the
// watcher's status events and the gate Runner's reports. Each folds itself in
// and re-arms its own wait, which is what separates them from the one-shot
// results below - those answer a tea.Cmd omatty issued once and are done.
//
// A table of its own because onDataMsg was already at the statement limit that
// split it from onSessionMsg (#122), and #231's gate reports pushed it over.
// The second return says whether the message was one of these, so a nil
// command from a handler is not mistaken for "not mine".
func (m *Model) onStreamMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch typed := msg.(type) {
	case StatusMsg:
		return m.onStatus(typed), true
	case GateMsg:
		return m.onGate(typed), true
	case coverageMsg:
		m.onCoverage(typed)
		return nil, true
	case generatedMsg:
		m.onGenerated(typed)
		return nil, true
	case RevertedMsg:
		return m.onReverted(typed), true
	}
	return nil, false
}

// onPaneMsg is onDataMsg's second table: what a running pane produced on its
// own, as opposed to work omatty went and did. A clipboard write is the first
// of those - the child copied something, and nothing omatty scheduled asked
// it to (#212).
//
// A table of its own rather than a seventh arm above, because onDataMsg is
// already at the statement limit that split it from onSessionMsg in the first
// place (#122), and because the distinction is real: everything above is a
// result coming back, this is a pane speaking unprompted.
func (m *Model) onPaneMsg(msg tea.Msg) tea.Cmd {
	if clip, ok := msg.(ClipboardMsg); ok {
		return m.onClipboard(clip)
	}
	return m.onSessionMsg(msg)
}

// onSessionMsg is the third table: the messages that change which sessions
// exist, or which process is behind one.
func (m *Model) onSessionMsg(msg tea.Msg) tea.Cmd {
	if cmd, ok := m.onForgeMsg(msg); ok {
		return cmd
	}
	switch typed := msg.(type) {
	case ProjectsProposedMsg:
		return m.onProjectsProposed(typed)
	case SessionsProposedMsg:
		return m.onSessionsProposed(typed)
	case sessionRelaunchMsg:
		return m.relaunch(typed.Session)
	case RepoStatMsg:
		return m.onRepoStat(typed)
	}
	return m.onWindowFocus(msg)
}

// onForgeMsg is what the three gh-backed reads answer with (#310, #394, #397),
// split out of onSessionMsg when the third pushed it past the statement limit -
// and they belong together: one forge, three reads.
func (m *Model) onForgeMsg(msg tea.Msg) (tea.Cmd, bool) {
	switch typed := msg.(type) {
	case PRsLoadedMsg:
		return m.onPRs(typed), true
	case IssuesLoadedMsg:
		return m.onIssues(typed), true
	case ItemLoadedMsg:
		return m.onItem(typed), true
	case BrowsedMsg:
		return m.onBrowsed(typed), true
	}
	return nil, false
}

// onWindowFocus records whether omatty itself has the operator's attention,
// which is what gates notifications, and otherwise broadcasts.
func (m *Model) onWindowFocus(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case tea.FocusMsg:
		m.hasFocus = true
		// Catch up on whatever changed while the poll was gated off, rather
		// than leaving stale cards up for the rest of the ten-second period.
		return tea.Batch(m.pollAll(), m.pollPRs(), m.pollIssues())
	case tea.BlurMsg:
		m.hasFocus = false
		return nil
	}
	// Everything else is emulator traffic. Broadcast it: each bubbleterm
	// ignores messages from other emulators, and the message that re-arms a
	// poll must reach the terminal that scheduled it. Unfocused sessions are
	// pumped too, or they stop reading their PTYs (issue #33). Keys are
	// deliberately not broadcast - they belong to the focused session only.
	//
	// This is the one path that mutates a terminal and never the model, so
	// the frame outlives it. The single thing it can change - the focused
	// pane's content - is part of the memo's key, so it is checked rather
	// than assumed.
	m.paneOnly = true
	return m.broadcast(msg)
}

// broadcast forwards msg to every terminal and batches whatever they return.
func (m *Model) broadcast(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.terms))
	for _, term := range m.terms {
		if cmd := term.Update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

// onResize gives the terminal pane whatever the sidebar and diff pane leave.
// Off a tty bubbletea reports 0x0, which would floor every pane; the default
// stands instead (issue #74).
func (m *Model) onResize(msg tea.WindowSizeMsg) tea.Cmd {
	if msg.Width == 0 || msg.Height == 0 {
		return nil
	}
	m.width, m.height = msg.Width, msg.Height
	// A wider window raises the review column's width, which lowers the
	// ceiling on how far it may be panned. Nothing else re-clamps ColOffset, so
	// an offset left over from a narrow window made panLine drop every row
	// shorter than it and the column rendered blank until h/l was pressed - the
	// same "a resize reaches nothing" class #95 is about. renderEntries and
	// renderTree already recompute their vertical offsets for this reason.
	m.panReview(0)
	return m.resizeSelected()
}

// moveCursor moves the sidebar cursor, sizes the terminal it lands on (issue
// #73) and moves an open review column along with it (#21). It sizes what the
// cursor selects, not what holds the keyboard - the distinction #95 drew.
func (m *Model) moveCursor(move func()) tea.Cmd {
	move()
	return tea.Batch(m.resizeSelected(), m.followSession())
}

// ptySize is the live embedded-terminal size for the current window.
func (m *Model) ptySize() (int, int) { return PTYSize(m.width, m.height, m.review.Open) }

// resizeSelected sizes the selected session's terminal to the pane, whether or
// not a prompt currently owns the keyboard (issue #95). Only the selected
// terminal follows the window (issue #34), so the one just selected may still
// be at the size it was born or last selected at (issue #73).
func (m *Model) resizeSelected() tea.Cmd {
	term := m.selectedTerminal()
	if term == nil {
		return nil
	}
	return term.Resize(m.ptySize())
}

// selectedTerminal is the terminal the sidebar cursor is on, whether or not any
// surface currently owns the keyboard. Layout asks this one; key routing asks
// focusedTerminal. Answering both questions with one nil is issue #95: a resize
// arriving behind an open prompt reached no terminal at all.
//
// focusedTerminal is not "does the PTY own the keyboard" either - it only nils
// out for a modal, and the review column and note editor take keys without one.
// The question it answers is "no modal is open and a session is selected"; for
// the keyboard itself, ask focus().
//
// No guard on an empty id: a missing key yields the nil interface the caller
// already tests for, and a guard that cannot fire reads as an invariant.
func (m *Model) selectedTerminal() termwrap.Terminal { return m.terms[m.Selected()] }

// focusedTerminal returns nil while a modal surface is open, which is what
// keeps its keys out of the PTY without special-casing the router: an
// unfocused terminal already routes every key to omatty. Only key routing and
// rendering may ask this - sizing the pane must not (issue #95).
func (m *Model) focusedTerminal() termwrap.Terminal {
	if m.modalOpen() {
		return nil
	}
	return m.selectedTerminal()
}
