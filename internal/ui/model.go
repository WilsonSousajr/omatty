package ui

import (
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/keys"
	"github.com/WilsonSousajr/omatty/internal/notify"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// Leader is the one key omatty intercepts while the terminal has focus.
const Leader = "ctrl+o"

// CreateFunc registers a new session in project and returns it. branch is
// empty for a session on the project's main checkout.
type CreateFunc func(project, title, branch string) (registry.Session, error)

// StartFunc launches the embedded terminal for a session at w by h. Injected
// so the model can start a session created at runtime without knowing how;
// the size is a parameter so it is never frozen at startup (issue #73).
type StartFunc func(sess registry.Session, w, h int) (termwrap.Terminal, error)

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
	// Rename persists a session's new title (#41).
	Rename RenameFunc
	// Archive drops a session from the registry, RemoveWorktree deletes its
	// worktree, and TailStop ends its status tailer (#40).
	Archive        ArchiveFunc
	RemoveWorktree RemoveWorktreeFunc
	TailStop       func(sessionID string)
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
}

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
	// The archive path's three halves: forget the session, stop its tailer,
	// and optionally delete its worktree (#40).
	archive          ArchiveFunc
	removeWorktree   RemoveWorktreeFunc
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

// withDefaults fills the optional fields: the wall clock and a silent
// notifier, so no method needs a nil guard.
func (d Deps) withDefaults() Deps {
	if d.Clock == nil {
		d.Clock = time.Now
	}
	if d.Notifier == nil {
		d.Notifier = notify.Silent{}
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
	if d.Archive == nil {
		d.Archive = noArchive
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

// NewModel builds the root model from its dependencies.
func NewModel(deps Deps) *Model {
	d := deps.withDefaults()
	m := &Model{
		state:     d.State,
		sidebar:   NewSidebar(SidebarRows(d.State, nil)),
		terms:     d.Terms,
		router:    keys.NewRouter(Leader),
		create:    d.Create,
		start:     d.Start,
		events:    d.Events,
		clock:     d.Clock,
		tailStart: d.TailStart,
		notifier:  d.Notifier,
		startedAt: d.Clock(),
		hasFocus:  true,
	}
	return m.withSources(d).withWindow().withRuntimeMaps()
}

// withSources attaches the injected functions that reach outside ui: the
// review column's readers (#21, #24) and the lifecycle commands (#40, #41).
func (m *Model) withSources(d Deps) *Model {
	m.diff, m.files, m.preview = d.Diff, d.Files, d.Preview
	m.rename, m.archive = d.Rename, d.Archive
	m.removeWorktree, m.tailStop = d.RemoveWorktree, d.TailStop
	m.discover, m.registerProjects = d.Discover, d.AddProject
	m.adoptPropose, m.adoptCommit = d.AdoptPropose, d.AdoptCommit
	m.stop, m.notice = d.Stop, d.Notice
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

// SelectedProject returns the project the cursor is in. With no session
// selected it falls back to the first project header, so creating the very
// first session still lands somewhere sensible.
func (m *Model) SelectedProject() string {
	if row, ok := m.sidebar.Selected(); ok {
		return row.Project
	}
	if rows := m.sidebar.Rows(); len(rows) > 0 {
		return rows[0].Project
	}
	return ""
}

// Init starts every session's terminal reading from its PTY.
//
// bubbleterm.Init returns a self-rescheduling blocking poll; without it no
// terminal ever reads anything and every pane stays blank (issue #33).
func (m *Model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.terms)+2)
	for _, term := range m.terms {
		if cmd := term.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if cmd := m.waitForEvent(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	cmds = append(cmds, scheduleTick())
	return tea.Batch(cmds...)
}

// Update routes messages to one handler per type, so it stays a router and
// stays inside the 20-line function limit.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m, m.onKey(msg)
	case tea.WindowSizeMsg:
		return m, m.onResize(msg)
	case tea.MouseMsg:
		return m, m.onMouse(msg)
	case TickMsg:
		return m, scheduleTick()
	default:
		return m, m.onDataMsg(msg)
	}
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
// The split into two is paneCommand's: one table ran past the length limit, and
// the second is named here so the pair still reads as one list (#122).
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
	}
	return m.onSessionMsg(msg)
}

// onSessionMsg is onDataMsg's second table: the messages that change which
// sessions exist, or which process is behind one.
func (m *Model) onSessionMsg(msg tea.Msg) tea.Cmd {
	switch typed := msg.(type) {
	case ProjectsProposedMsg:
		return m.onProjectsProposed(typed)
	case SessionsProposedMsg:
		return m.onSessionsProposed(typed)
	case sessionRelaunchMsg:
		return m.relaunch(typed.Session)
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

// sessionRelaunchMsg carries a session whose held claude has been ended and
// whose replacement process should now start. It exists because the stop half
// runs off the Update goroutine and the start half must not begin until it has
// finished (#15, #43).
type sessionRelaunchMsg struct{ Session registry.Session }

// restartSelected relaunches the focused session's process in place (issue
// #15). It covers a crashed pane and a claude that exited.
//
// The held process is ended first, and the relaunch waits for that. Under a
// detach holder `dtach -A` attaches to a live master and discards the command
// it was handed, so starting without stopping swapped the client and left the
// wedged claude exactly where it was: the key did nothing and said nothing,
// and archiving the row became the only way to end a stuck session (#43).
func (m *Model) restartSelected() tea.Cmd {
	row, ok := m.sidebar.Selected()
	if !ok {
		return nil
	}
	sess := *row.Session
	return m.stopSessionCmd(sess, sessionRelaunchMsg{Session: sess})
}

// relaunch starts the replacement process. The old terminal is closed only
// after the new one starts, so a failed restart never leaves the pane empty;
// the launcher resumes the transcript (#36) so nothing is lost.
func (m *Model) relaunch(sess registry.Session) tea.Cmd {
	w, h := m.ptySize()
	term, err := m.start(sess, w, h)
	if err != nil {
		m.lastErr = fmt.Sprintf("restarting %s: %v", sess.Title, err)
		return nil
	}
	if old := m.terms[sess.ID]; old != nil {
		_ = old.Close()
	}
	m.terms[sess.ID] = term
	// Born at the live size, so no Resize races claude's startup (issue #73).
	return term.Init()
}

// submitPrompt creates the session. A worktree prompt uses the buffer as both
// the session title and the branch name. The buffer is known non-empty:
// commitEditor leaves the editor open rather than registering a nameless
// session.
func (m *Model) submitPrompt() tea.Cmd {
	branch := ""
	if m.modal.Editor.Worktree {
		branch = m.modal.Editor.Buffer
	}
	m.lastErr = ""
	project := m.SelectedProject()
	title := m.modal.Editor.Buffer
	m.modal = modal{}
	cmd, err := m.addSession(project, title, branch)
	if err != nil {
		slog.Error("creating session",
			"project", project, "title", title, "branch", branch, "err", err)
		m.lastErr = err.Error()
		return nil
	}
	return cmd
}

// addSession registers the session, starts its terminal, and rebuilds the
// sidebar so it is visible and focused immediately (issue #32). A session
// whose terminal will not start is not added: it would be a row you cannot
// focus.
func (m *Model) addSession(project, title, branch string) (tea.Cmd, error) {
	sess, err := m.create(project, title, branch)
	if err != nil {
		return nil, err
	}
	return m.foldInSession(sess)
}

// foldInSession brings a session omatty has just learned about into the running
// app: its terminal, its state, its sidebar row, its tailer.
//
// Shared by creation (#32) and adoption (#122), which differ only in where the
// Session came from - the registry made one, the transcript store named the
// other. Every step here is load-bearing, and a second copy that dropped one
// would fail quietly: no terminal is a row you cannot focus, no tailer is a
// session that never shows status (#33).
func (m *Model) foldInSession(sess registry.Session) (tea.Cmd, error) {
	w, h := m.ptySize()
	term, err := m.start(sess, w, h)
	if err != nil {
		return nil, fmt.Errorf("starting session %s: %w", sess.ID, err)
	}
	m.terms[sess.ID] = term
	m.state.Sessions = append(m.state.Sessions, sess)
	m.sidebar = NewSidebar(SidebarRows(m.state, m.statusMap()))
	if !m.selectSession(sess.ID) {
		slog.Warn("a new session is not in the rebuilt sidebar",
			"session", sess.ID, "project", sess.Project)
	}
	m.tailStart(sess)
	// The new terminal needs its own poll started; the others already have
	// theirs (issue #33). followSession goes with it: this moves the selection,
	// and every other site that moves the selection drags an open review column
	// along. Without it the column kept showing the previous session's diff,
	// title and comments beside the new session's terminal, and r/S/c acted on
	// the wrong session (#21, #95).
	return tea.Batch(term.Init(), m.followSession()), nil
}

// selectSession moves the cursor onto id, so a freshly created session is the
// one you are looking at, and reports whether the row was there to move to.
//
// The bool is not discarded: a session that was just created and added to the
// rebuilt rows and is still not found means the rebuild dropped it, which is
// the one case addSession would want to hear about.
func (m *Model) selectSession(id string) bool { return m.sidebar.SelectByID(id) }

func trimLastRune(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
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
