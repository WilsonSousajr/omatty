package ui

import (
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

// Model is omatty's root Bubble Tea model.
//
//	m := ui.NewModel(state, terms)
//	tea.NewProgram(m).Run()
type Model struct {
	sidebar *Sidebar
	terms   map[string]termwrap.Terminal
	router  *keys.Router
	width   int
	height  int
}

// NewModel builds the root model over a registered state and one Terminal
// per session id.
func NewModel(st registry.State, terms map[string]termwrap.Terminal) *Model {
	return &Model{
		sidebar: NewSidebar(SidebarRows(st, nil)),
		terms:   terms,
		router:  keys.NewRouter(Leader),
	}
}

// Focused returns the selected session's id, or "" when none is selected.
func (m *Model) Focused() string {
	row, ok := m.sidebar.Selected()
	if !ok {
		return ""
	}
	return row.Session.ID
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

// command runs an omatty command key, pressed after the leader.
func (m *Model) command(key string) tea.Cmd {
	switch key {
	case "j":
		m.sidebar.MoveDown()
	case "k":
		m.sidebar.MoveUp()
	case "q":
		return tea.Quit
	}
	return nil
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

func (m *Model) focusedTerminal() termwrap.Terminal {
	id := m.Focused()
	if id == "" {
		return nil
	}
	return m.terms[id]
}

// View renders the sidebar above the focused session's terminal.
func (m *Model) View() tea.View {
	var b strings.Builder
	for _, row := range m.sidebar.Rows() {
		b.WriteString(renderRow(row))
		b.WriteByte('\n')
	}
	if term := m.focusedTerminal(); term != nil {
		b.WriteString(term.View())
		return tea.NewView(b.String())
	}
	b.WriteString("no sessions - press " + Leader + " n to create one")
	return tea.NewView(b.String())
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
