package ui

import (
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/keys"
	"github.com/WilsonSousajr/omatty/internal/notify"
	"github.com/WilsonSousajr/omatty/internal/registry"
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
	lastErr   string
	width     int
	height    int
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
	return d
}

// NewModel builds the root model from its dependencies.
func NewModel(deps Deps) *Model {
	d := deps.withDefaults()
	return &Model{
		state:     d.State,
		sidebar:   NewSidebar(SidebarRows(d.State, nil)),
		terms:     d.Terms,
		router:    keys.NewRouter(Leader),
		create:    d.Create,
		start:     d.Start,
		status:    map[string]watcher.SessionState{},
		events:    d.Events,
		clock:     d.Clock,
		tailStart: d.TailStart,
		notifier:  d.Notifier,
		notified:  map[string]time.Time{},
		startedAt: d.Clock(),
		hasFocus:  true,
		// Until the first WindowSizeMsg arrives the frame is laid out for a
		// conventional terminal rather than a 0x0 one (issue #74).
		width:  DefaultWidth,
		height: DefaultHeight,
	}
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
	case TickMsg:
		return m, scheduleTick()
	case tea.FocusMsg:
		m.hasFocus = true
	case tea.BlurMsg:
		m.hasFocus = false
	}
	// Everything else is emulator traffic. Broadcast it: each bubbleterm
	// ignores messages from other emulators, and the message that re-arms a
	// poll must reach the terminal that scheduled it. Unfocused sessions are
	// pumped too, or they stop reading their PTYs (issue #33). Keys are
	// deliberately not broadcast - they belong to the focused session only.
	return m, m.broadcast(msg)
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

// onKey applies invariant 1: with the terminal focused every key reaches the
// PTY except the leader.
//
// A key reaches the terminal as the message itself, not as text: bubbleterm
// does its own key-to-escape translation, so forwarding msg.String() would
// type the literal word "esc" into Claude.
func (m *Model) onKey(msg tea.KeyPressMsg) tea.Cmd {
	m.lastErr = "" // any keypress acknowledges the last error
	term := m.focusedTerminal()
	switch m.router.Next(msg.Keystroke(), term != nil) {
	case keys.ToTerminal:
		return term.Update(msg)
	case keys.ToOmatty:
		return m.command(msg.Keystroke())
	default: // keys.Swallow - the leader itself
		return nil
	}
}

// command runs an omatty command key, pressed after the leader or while a
// prompt is open.
func (m *Model) command(key string) tea.Cmd {
	// ctrl+c is the unconditional escape hatch, checked before the prompt so
	// an open prompt cannot trap the operator (issue #28). With a session
	// focused this is never reached: ctrl+c belongs to Claude, which uses it
	// to interrupt a turn (invariant 1).
	if key == "ctrl+c" {
		return tea.Quit
	}
	if m.prompt.Active {
		return m.onPromptKey(key)
	}
	return m.navigate(key)
}

// navigate runs a command key while no prompt is open.
func (m *Model) navigate(key string) tea.Cmd {
	switch key {
	case "j":
		return m.moveCursor(m.sidebar.MoveDown)
	case "k":
		return m.moveCursor(m.sidebar.MoveUp)
	case "n":
		m.prompt = Prompt{Active: true}
	// Keystroke() spells a shifted letter "shift+N"; the bare "N" is accepted
	// too because not every terminal reports the modifier.
	case "shift+N", "N":
		m.prompt = Prompt{Active: true, Worktree: true}
	case "r":
		return m.restartFocused()
	case "q":
		return tea.Quit
	}
	return nil
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

// moveCursor moves the sidebar cursor and sizes the terminal it lands on
// (issue #73).
func (m *Model) moveCursor(move func()) tea.Cmd {
	move()
	return m.resizeFocused()
}

// ptySize is the live embedded-terminal size for the current window.
func (m *Model) ptySize() (int, int) { return PTYSize(m.width, m.height) }

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
