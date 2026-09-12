package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
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
	// comments is each session's pending review queue, kept across opening and
	// closing the column; only submit drains it (#22).
	comments map[string]*review.Comments
	diff     DiffFunc
	files    ListFilesFunc
	preview  PreviewFunc
	rename   RenameFunc
	name     NameFunc
	// modelNamer is nil unless the config opted in to a headless naming call
	// (#127).
	modelNamer ModelNameFunc
	// namePending guards one in-flight name read per session (#127). Not
	// persisted, and correctly empty after a relaunch: whether a session
	// still needs a name is derived from its title, not from this map.
	namePending map[string]bool
	// gates is each session's last gate report, display-only like the lane and
	// repoStat and never persisted: state.json must suffice alone
	// (invariant 9). Absent means no gate has run, which the card shows as a
	// blank line rather than as a pass (#230).
	gates map[string]gate.Report
	// lane is each session's recent-status trace for the sidebar (#128).
	lane map[string]activityLane
	// stat reads a card's branch and diffstat; repoStat is the last answer per
	// session, display-only like the lane and never persisted - state.json
	// must suffice alone (invariant 9). statPending guards one poll in flight
	// per session; statFailed makes the warning once per outage (#180).
	stat        RepoStatFunc
	repoStat    map[string]review.Stat
	statPending map[string]bool
	statFailed  map[string]bool
	// filesPending guards one worktree listing in flight per session (#195).
	filesPending map[string]bool
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
	lastErr   string
	// stop ends an archived session's held claude (#43).
	stop StopFunc
	// notice is the startup line, cleared by the first keypress the way
	// lastErr is: the keymap it displaces is worth more than a warning already
	// read (#43).
	notice string
	// wheel counts scroll notches so a momentum flick becomes a few pages of
	// transcript rather than tens of them (#107).
	wheel  wheelAccumulator
	width  int
	height int
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
		tailStart:  d.TailStart,
		notifier:   d.Notifier,
		startedAt:  d.Clock(),
		hasFocus:   true,
		reattached: d.Reattached,
	}
	return m.withSources(d).withWindow().withRuntimeMaps()
}

// withSources attaches the injected functions that reach outside ui: the
// review column's readers (#21, #24) and the lifecycle commands (#40, #41).
func (m *Model) withSources(d Deps) *Model {
	m.diff, m.files, m.preview = d.Diff, d.Files, d.Preview
	m.rename, m.name, m.archive = d.Rename, d.Name, d.Archive
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

// withRuntimeMaps allocates the per-session maps the model fills as it runs:
// live status, notification times, and each session's queued review comments.
// They are never nil, so no method needs a nil guard (issue #76).
func (m *Model) withRuntimeMaps() *Model {
	m.status = map[string]watcher.SessionState{}
	m.notified = map[string]time.Time{}
	m.comments = map[string]*review.Comments{}
	m.namePending = map[string]bool{}
	m.lane = map[string]activityLane{}
	m.gates = map[string]gate.Report{}
	m.repoStat = map[string]review.Stat{}
	m.statPending = map[string]bool{}
	m.statFailed = map[string]bool{}
	m.filesPending = map[string]bool{}
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
	// The first stat poll runs at start rather than a tick later, so a card
	// names its branch before the operator has read the screen (#180).
	cmds = append(cmds, scheduleTick(), m.onStatTick())
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
	if cmd, ok := m.onInput(msg); ok {
		return m, cmd
	}
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m, m.onResize(msg)
	case TickMsg:
		return m, scheduleTick()
	case StatTickMsg:
		return m, m.onStatTick()
	default:
		return m, m.onDataMsg(msg)
	}
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
	switch typed := msg.(type) {
	case StatusMsg:
		return m.onStatus(typed)
	case DiffLoadedMsg:
		return m.onDiffLoaded(typed)
	case FilesLoadedMsg:
		return m.onFilesLoaded(typed)
	case WorktreeRemovedMsg:
		return m.onWorktreeRemoved(typed)
	case NamedMsg:
		return m.onNamed(typed)
	case ModelNamedMsg:
		return m.onModelNamed(typed)
	}
	return m.onPaneMsg(msg)
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

// onWindowFocus records whether omatty itself has the operator's attention,
// which is what gates notifications, and otherwise broadcasts.
func (m *Model) onWindowFocus(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case tea.FocusMsg:
		m.hasFocus = true
		return nil
	case tea.BlurMsg:
		m.hasFocus = false
		return nil
	}
	// Everything else is emulator traffic. Broadcast it: each bubbleterm
	// ignores messages from other emulators, and the message that re-arms a
	// poll must reach the terminal that scheduled it. Unfocused sessions are
	// pumped too, or they stop reading their PTYs (issue #33). Keys are
	// deliberately not broadcast - they belong to the focused session only.
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
