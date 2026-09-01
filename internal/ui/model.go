package ui

import (
	"log/slog"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/keys"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// Leader is the one key omatty intercepts while the terminal has focus.
const Leader = "ctrl+o"

// Widths of the panes that flank the terminal. Fixed for M1; the terminal
// takes whatever is left.
const (
	sidebarWidth = 24
	diffWidth    = 40
)

// CreateFunc registers a new session in project. branch is empty for a
// session on the project's main checkout.
type CreateFunc func(project, title, branch string) error

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
	sidebar *Sidebar
	terms   map[string]termwrap.Terminal
	router  *keys.Router
	create  CreateFunc
	prompt  Prompt
	lastErr string
	width   int
	height  int
}

// NewModel builds the root model over a registered state and one Terminal
// per session id.
func NewModel(st registry.State, terms map[string]termwrap.Terminal, create CreateFunc) *Model {
	return &Model{
		sidebar: NewSidebar(SidebarRows(st, nil)),
		terms:   terms,
		router:  keys.NewRouter(Leader),
		create:  create,
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

// Init starts nothing: terminals are started by the supervisor before the
// model is built.
func (m *Model) Init() tea.Cmd { return nil }

// Update routes messages to one handler per type, so it stays a router and
// stays inside the 20-line function limit.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m, m.onKey(msg)
	case tea.WindowSizeMsg:
		return m, m.onResize(msg)
	}
	return m, nil
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
		m.submitPrompt()
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
func (m *Model) submitPrompt() {
	if m.prompt.Buffer == "" {
		return
	}
	branch := ""
	if m.prompt.Worktree {
		branch = m.prompt.Buffer
	}
	m.lastErr = ""
	project := m.SelectedProject()
	if err := m.create(project, m.prompt.Buffer, branch); err != nil {
		slog.Error("creating session",
			"project", project, "title", m.prompt.Buffer, "branch", branch, "err", err)
		m.lastErr = err.Error()
	}
	m.prompt = Prompt{}
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
	return term.Resize(m.terminalWidth(), msg.Height)
}

// terminalWidth is what remains of the window after the flanking panes, with
// a floor so a narrow window still renders something.
func (m *Model) terminalWidth() int {
	w := m.width - sidebarWidth - diffWidth
	if w < 20 {
		return 20
	}
	return w
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
