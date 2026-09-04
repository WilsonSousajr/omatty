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

// Prompt is the pending new-session input. The zero value means no prompt.
type Prompt struct {
	Active bool
	// Worktree is true when the prompt was opened with N, meaning the buffer
	// names a branch to create a worktree on.
	Worktree bool
	Buffer   string
}

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
	prompt    Prompt
	review    ReviewPane
	// comments is each session's pending review queue, kept across opening and
	// closing the column; only submit drains it (#22).
	comments map[string]*review.Comments
	diff     DiffFunc
	files    ListFilesFunc
	preview  PreviewFunc
	lastErr  string
	width    int
	height   int
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
		// The three review-column sources, grouped: they are one concern.
		diff: d.Diff, files: d.Files, preview: d.Preview,
		startedAt: d.Clock(),
		hasFocus:  true,
		// Until the first WindowSizeMsg arrives the frame is laid out for a
		// conventional terminal rather than a 0x0 one (issue #74).
		width:  DefaultWidth,
		height: DefaultHeight,
	}
	return m.withRuntimeMaps()
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

// Prompt returns the pending new-session input, if any.
func (m *Model) Prompt() Prompt { return m.prompt }

// Focused returns the selected session's id, or "" when none is selected.
func (m *Model) Focused() string {
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
	case StatusMsg:
		return m, m.onStatus(msg)
	case DiffLoadedMsg:
		return m, m.onDiffLoaded(msg)
	case FilesLoadedMsg:
		return m, m.onFilesLoaded(msg)
	case TickMsg:
		return m, scheduleTick()
	default:
		return m, m.onWindowFocus(msg)
	}
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

// restartFocused relaunches the focused session's process in place (issue
// #15). It covers a crashed pane and a claude that exited. The old terminal
// is closed only after the new one starts, so a failed restart never leaves
// the pane empty; the launcher resumes the transcript (#36) so nothing is
// lost.
func (m *Model) restartFocused() tea.Cmd {
	row, ok := m.sidebar.Selected()
	if !ok {
		return nil
	}
	sess := *row.Session
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

// onPromptKey edits the prompt buffer. A worktree prompt uses the buffer as
// both the session title and the branch name.
func (m *Model) onPromptKey(key string) tea.Cmd {
	switch key {
	case "esc":
		m.prompt = Prompt{}
	case "enter":
		return m.submitPrompt()
	case "backspace":
		m.prompt.Buffer = trimLastRune(m.prompt.Buffer)
	default:
		if len([]rune(key)) == 1 {
			m.prompt.Buffer += key
		}
	}
	return nil
}

// submitPrompt creates the session. An empty title leaves the prompt open
// rather than registering a nameless session.
func (m *Model) submitPrompt() tea.Cmd {
	if m.prompt.Buffer == "" {
		return nil
	}
	branch := ""
	if m.prompt.Worktree {
		branch = m.prompt.Buffer
	}
	m.lastErr = ""
	project := m.SelectedProject()
	title := m.prompt.Buffer
	m.prompt = Prompt{}
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
	w, h := m.ptySize()
	term, err := m.start(sess, w, h)
	if err != nil {
		return nil, fmt.Errorf("starting session %s: %w", sess.ID, err)
	}
	m.terms[sess.ID] = term
	m.state.Sessions = append(m.state.Sessions, sess)
	m.sidebar = NewSidebar(SidebarRows(m.state, m.statusMap()))
	m.selectSession(sess.ID)
	if m.tailStart != nil {
		m.tailStart(sess)
	}
	// The new terminal needs its own poll started; the others already have
	// theirs (issue #33).
	return term.Init(), nil
}

// selectSession moves the cursor onto id, so a freshly created session is the
// one you are looking at.
func (m *Model) selectSession(id string) {
	for {
		row, ok := m.sidebar.Selected()
		if !ok || row.Session.ID == id {
			return
		}
		before := row.Session.ID
		m.sidebar.MoveDown()
		if next, _ := m.sidebar.Selected(); next.Session.ID == before {
			return // reached the end without finding it
		}
	}
}

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
	return m.resizeFocused()
}

// moveCursor moves the sidebar cursor, sizes the terminal it lands on (issue
// #73) and moves an open review column along with it (#21).
func (m *Model) moveCursor(move func()) tea.Cmd {
	move()
	return tea.Batch(m.resizeFocused(), m.followSession())
}

// ptySize is the live embedded-terminal size for the current window.
func (m *Model) ptySize() (int, int) { return PTYSize(m.width, m.height, m.review.Open) }

// resizeFocused sizes the newly focused terminal to the pane. Only the
// focused terminal follows the window (issue #34), so the one just focused
// may still be at the size it was born or last focused at (issue #73).
func (m *Model) resizeFocused() tea.Cmd {
	term := m.focusedTerminal()
	if term == nil {
		return nil
	}
	return term.Resize(m.ptySize())
}

// focusedTerminal returns nil while a prompt is open, which is what keeps
// prompt keys out of the PTY without special-casing the router: an unfocused
// terminal already routes every key to omatty.
func (m *Model) focusedTerminal() termwrap.Terminal {
	if m.prompt.Active {
		return nil
	}
	id := m.Focused()
	if id == "" {
		return nil
	}
	return m.terms[id]
}
