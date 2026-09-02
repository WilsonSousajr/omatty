package ui

import (
	"fmt"
	"log/slog"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/keys"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// Leader is the one key omatty intercepts while the terminal has focus.
const Leader = "ctrl+o"

// footerRows is the chrome below the terminal. The sidebar is rendered above
// it and its height is counted from the actual row count.
const footerRows = 1

// CreateFunc registers a new session in project and returns it. branch is
// empty for a session on the project's main checkout.
type CreateFunc func(project, title, branch string) (registry.Session, error)

// StartFunc launches the embedded terminal for a session. Injected so the
// model can start a session created at runtime without knowing how.
type StartFunc func(sess registry.Session) (termwrap.Terminal, error)

// Prompt is the pending new-session input. The zero value means no prompt.
type Prompt struct {
	Active bool
	// Worktree is true when the prompt was opened with N, meaning the buffer
	// names a branch to create a worktree on.
	Worktree bool
	Buffer   string
}

// Model is omatty's root Bubble Tea model.
//
//	m := ui.NewModel(state, terms, create)
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
	prompt  Prompt
	lastErr string
	width   int
	height  int
}

// NewModel builds the root model over a registered state and one Terminal
// per session id.
func NewModel(
	st registry.State, terms map[string]termwrap.Terminal,
	create CreateFunc, start StartFunc,
) *Model {
	return &Model{
		state:   st,
		sidebar: NewSidebar(SidebarRows(st, nil)),
		terms:   terms,
		router:  keys.NewRouter(Leader),
		create:  create,
		start:   start,
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
	cmds := make([]tea.Cmd, 0, len(m.terms))
	for _, term := range m.terms {
		if cmd := term.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
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
		m.sidebar.MoveDown()
	case "k":
		m.sidebar.MoveUp()
	case "n":
		m.prompt = Prompt{Active: true}
	// Keystroke() spells a shifted letter "shift+N"; the bare "N" is accepted
	// too because not every terminal reports the modifier.
	case "shift+N", "N":
		m.prompt = Prompt{Active: true, Worktree: true}
	case "q":
		return tea.Quit
	}
	return nil
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
	term, err := m.start(sess)
	if err != nil {
		return nil, fmt.Errorf("starting session %s: %w", sess.ID, err)
	}
	m.terms[sess.ID] = term
	m.state.Sessions = append(m.state.Sessions, sess)
	m.sidebar = NewSidebar(SidebarRows(m.state, nil))
	m.selectSession(sess.ID)
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
func (m *Model) onResize(msg tea.WindowSizeMsg) tea.Cmd {
	m.width, m.height = msg.Width, msg.Height
	term := m.focusedTerminal()
	if term == nil {
		return nil
	}
	return term.Resize(m.width, m.terminalHeight())
}

// terminalHeight is what the window leaves after the sidebar above and the
// footer below. The terminal takes the full width: nothing is rendered beside
// it yet, and reserving columns for a diff pane that does not exist left
// claude wrapping at width-64 with the rest of the screen blank (issue #34).
func (m *Model) terminalHeight() int {
	h := m.height - len(m.sidebar.Rows()) - footerRows
	if h < 4 {
		return 4
	}
	return h
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

// View renders the sidebar above the focused session's terminal.
func (m *Model) View() tea.View {
	var b strings.Builder
	b.WriteString(m.renderSidebar())
	if m.prompt.Active {
		b.WriteString(m.promptLine())
	}
	if m.lastErr != "" {
		b.WriteString("error: " + m.lastErr + "\n")
	}
	b.WriteString(m.renderBody())
	return tea.NewView(b.String())
}

func (m *Model) renderSidebar() string {
	var b strings.Builder
	for _, row := range m.sidebar.Rows() {
		b.WriteString(renderRow(row))
		b.WriteByte('\n')
	}
	return b.String()
}

// renderBody is the focused session's terminal, or the empty-state guidance
// when there is none. Both end in a keymap so the exit is never hidden.
func (m *Model) renderBody() string {
	if term := m.focusedTerminal(); term != nil {
		return term.View() + "\n" + footer
	}
	var b strings.Builder
	if !m.prompt.Active {
		b.WriteString(m.emptyStateHint())
	}
	// With no session focused ctrl+c also quits, which is worth saying because
	// it is the reflex an operator reaches for first (issue #28).
	b.WriteString("ctrl+c or " + Leader + " q to quit\n")
	return b.String()
}

// footer is the keymap, rendered on every frame. It stays visible while a
// session fills the pane because that is exactly the state where ctrl+c
// belongs to Claude and `ctrl+o q` is the only exit (issues #28, #30).
const footer = Leader + " j/k switch  " + Leader + " n new  " +
	Leader + " N worktree  " + Leader + " q quit"

// emptyStateHint names the next useful action. With no projects registered,
// creating a session can only fail, so it points at `omatty add` instead.
func (m *Model) emptyStateHint() string {
	if len(m.sidebar.Rows()) == 0 {
		return "no projects - run `omatty add <dir>` to register one\n"
	}
	return "no sessions - press " + Leader + " n to create one\n"
}

// promptLine renders the open new-session prompt.
func (m *Model) promptLine() string {
	label := "new session title"
	if m.prompt.Worktree {
		label = "new branch (worktree)"
	}
	return label + ": " + m.prompt.Buffer + "_\n"
}

func renderRow(row Row) string {
	if row.Session == nil {
		return "> " + row.Project
	}
	return "  " + statusGlyph(row.Status) + " " + row.Session.Title
}

func statusGlyph(s registry.Status) string {
	switch s {
	case registry.StatusThinking:
		return "*"
	case registry.StatusTool:
		return "@"
	case registry.StatusWaiting:
		return "!"
	case registry.StatusDone:
		return "+"
	case registry.StatusError:
		return "x"
	default:
		return "-"
	}
}
