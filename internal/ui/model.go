package ui

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

// NewModel builds the root model over a registered state and one Terminal
// per session id.
func NewModel(
	st registry.State, terms map[string]termwrap.Terminal,
	create CreateFunc, start StartFunc,
) *Model {
	return &Model{
		state:     st,
		sidebar:   NewSidebar(SidebarRows(st, nil)),
		terms:     terms,
		router:    keys.NewRouter(Leader),
		create:    create,
		start:     start,
		status:    map[string]watcher.SessionState{},
		clock:     time.Now,
		notified:  map[string]time.Time{},
		startedAt: time.Now(),
		hasFocus:  true,
		// Until the first WindowSizeMsg arrives the frame is laid out for a
		// conventional terminal rather than a 0x0 one, which would floor every
		// pane and truncate the text in it.
		width:  defaultWidth,
		height: defaultHeight,
	}
}

// Prompt returns the pending new-session input, if any.
func (m *Model) Prompt() Prompt { return m.prompt }

// StatusMsg carries one watcher event into the model's Update loop.
type StatusMsg watcher.Event

// TickMsg is the once-a-second heartbeat that re-renders the frame, so a
// quiet session's age keeps counting (issue #71). Exported so tests can send
// one.
type TickMsg time.Time

// tickEvery is the age column's resolution; finer buys nothing.
const tickEvery = time.Second

func scheduleTick() tea.Cmd {
	return tea.Tick(tickEvery, func(t time.Time) tea.Msg { return TickMsg(t) })
}

// SetNotifier sets the desktop notifier used when a backgrounded session needs
// attention. Absent, notifications are simply not sent.
func (m *Model) SetNotifier(n notify.Notifier) { m.notifier = n }

// SetTailStarter registers a callback that starts a transcript tailer for a
// session created at runtime, so a new session gets status and tokens too.
func (m *Model) SetTailStarter(start func(registry.Session)) { m.tailStart = start }

// SetEvents attaches the live status stream and the clock the sidebar ages
// against. Called by Run; tests that exercise status call it too.
func (m *Model) SetEvents(events <-chan watcher.Event, clock func() time.Time) {
	m.events = events
	if clock != nil {
		m.clock = clock
	}
	m.startedAt = m.clock()
}

// waitForEvent blocks on the next status event and delivers it as a StatusMsg.
// nil events (a model built without a watcher) simply never fires.
func (m *Model) waitForEvent() tea.Cmd {
	if m.events == nil {
		return nil
	}
	return func() tea.Msg { return StatusMsg(<-m.events) }
}

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
	cmds := make([]tea.Cmd, 0, len(m.terms)+1)
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

// onStatus folds a watcher event into the session's state and re-arms the
// wait. Newer-wins lives in watcher.Apply; the model just stores the result.
// A hook can name any session id; only registered ones may grow the status
// map or reach the operator's notifications (issue #69).
func (m *Model) onStatus(ev StatusMsg) tea.Cmd {
	e := watcher.Event(ev)
	if !m.knownSession(e.SessionID) {
		return m.waitForEvent()
	}
	before := m.status[e.SessionID].Status
	after := watcher.Apply(m.status[e.SessionID], e)
	m.status[e.SessionID] = after
	m.sidebar.SetRows(SidebarRows(m.state, m.statusMap()))
	return tea.Batch(m.waitForEvent(), m.maybeNotify(e, before, after.Status))
}

func (m *Model) knownSession(id string) bool {
	for i := range m.state.Sessions {
		if m.state.Sessions[i].ID == id {
			return true
		}
	}
	return false
}

// notifyCooldown is the least time between two notifications for one
// session, so a permission loop cannot storm the desktop (issue #69).
const notifyCooldown = 5 * time.Second

// maybeNotify returns a command that posts a desktop notification when a
// session enters a state that needs the operator while omatty is
// backgrounded. It is a command, off the Update goroutine, because osascript
// takes tens of milliseconds (issue #69). Suppressed: a repeated state, a
// transition older than this run (issue #70), and a second notification for
// the same session within notifyCooldown.
func (m *Model) maybeNotify(e watcher.Event, before, after registry.Status) tea.Cmd {
	if m.notifier == nil || m.hasFocus || before == after || e.At.Before(m.startedAt) {
		return nil
	}
	body, ok := needsYou(m.sessionTitle(e.SessionID), after)
	if !ok || !m.cooldownElapsed(e.SessionID) {
		return nil
	}
	return notifyCmd(m.notifier, body)
}

func (m *Model) cooldownElapsed(id string) bool {
	now := m.clock()
	if last, ok := m.notified[id]; ok && now.Sub(last) < notifyCooldown {
		return false
	}
	m.notified[id] = now
	return true
}

func notifyCmd(n notify.Notifier, body string) tea.Cmd {
	return func() tea.Msg {
		if err := n.Notify("omatty", body); err != nil {
			slog.Warn("desktop notification failed", "body", body, "err", err)
		}
		return nil
	}
}

func (m *Model) sessionTitle(id string) string {
	for i := range m.state.Sessions {
		if m.state.Sessions[i].ID == id {
			return m.state.Sessions[i].Title
		}
	}
	return id
}

// needsYou returns the notification body for a status that wants attention.
func needsYou(title string, status registry.Status) (string, bool) {
	switch status {
	case registry.StatusWaiting:
		return title + " needs you", true
	case registry.StatusDone:
		return title + " finished", true
	default:
		return "", false
	}
}

// statusMap projects the per-session state down to the status the sidebar
// needs.
func (m *Model) statusMap() map[string]registry.Status {
	out := make(map[string]registry.Status, len(m.status))
	for id, st := range m.status {
		out[id] = st.Status
	}
	return out
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
		m.sidebar.MoveDown()
	case "k":
		m.sidebar.MoveUp()
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
	term, err := m.start(sess)
	if err != nil {
		m.lastErr = fmt.Sprintf("restarting %s: %v", sess.Title, err)
		return nil
	}
	if old := m.terms[sess.ID]; old != nil {
		_ = old.Close()
	}
	m.terms[sess.ID] = term
	return tea.Batch(term.Init(), term.Resize(PaneSize(m.width, m.height)))
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
func (m *Model) onResize(msg tea.WindowSizeMsg) tea.Cmd {
	m.width, m.height = msg.Width, msg.Height
	term := m.focusedTerminal()
	if term == nil {
		return nil
	}
	return term.Resize(PaneSize(msg.Width, msg.Height))
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

// footer is the keymap, rendered on every frame. It stays visible while a
// session fills the pane because that is exactly the state where ctrl+c
// belongs to Claude and `ctrl+o q` is the only exit (issues #28, #30).
const footer = Leader + " j/k switch  " + Leader + " n new  " +
	Leader + " N worktree  " + Leader + " r restart  " + Leader + " q quit"

// emptyStateHint names the next useful action. With no projects registered,
// creating a session can only fail, so it points at `omatty add` instead.
func (m *Model) emptyStateHint() string {
	if len(m.sidebar.Rows()) == 0 {
		return "no projects - run `omatty add <dir>` to register one"
	}
	return "no sessions - press " + Leader + " n to create one"
}

// promptLine renders the open new-session prompt.
func (m *Model) promptLine() string {
	label := "new session title"
	if m.prompt.Worktree {
		label = "new branch (worktree)"
	}
	return label + ": " + m.prompt.Buffer + "_"
}

// View lays the sidebar beside the focused session's terminal, with the
// keymap underneath (issue #35). Both boxes are sized exactly before the
// border is applied, so lipgloss adds precisely one column and row per side
// and the frame never exceeds the window.
func (m *Model) View() tea.View {
	termW, termH := PaneSize(m.width, m.height)
	now := m.clock() // once per frame, so every row ages against the same instant
	panes := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderSidebar(termH, now),
		m.renderTerminal(termW, termH, now))
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, panes, m.renderFooter()))
	v.AltScreen = true
	v.ReportFocus = true // so FocusMsg/BlurMsg drive notifications
	return v
}

// renderSidebar boxes the project/session rows at exactly SidebarWidth.
func (m *Model) renderSidebar(rows int, now time.Time) string {
	inner := SidebarWidth - 2
	lines := make([]string, 0, rows)
	lines = append(lines, headerStyle.Render(padRight("projects", inner)))
	for _, row := range m.sidebar.Rows() {
		lines = append(lines, m.renderRow(row, inner, now))
	}
	return paneBox(false).Render(fitBlock(lines, inner, rows))
}

// renderTerminal boxes the focused terminal, or the empty-state guidance.
func (m *Model) renderTerminal(w, h int, now time.Time) string {
	if term := m.focusedTerminal(); term != nil {
		body := fitBlock(strings.Split(term.View(), "\n"), w, h-1)
		return paneBox(true).Render(m.terminalTitle(w, now) + "\n" + body)
	}
	lines := []string{""}
	if m.prompt.Active {
		lines = append(lines, m.promptLine())
	} else {
		lines = append(lines, m.emptyStateHint())
	}
	// With no session focused ctrl+c also quits, which is worth saying because
	// it is the reflex an operator reaches for first (issue #28).
	lines = append(lines, "", "ctrl+c or "+Leader+" q to quit")
	return paneBox(m.prompt.Active).Render(fitBlock(lines, w, h))
}

// terminalTitle is the header line inside the focused session's box: its
// title, coloured status, age and cumulative tokens.
func (m *Model) terminalTitle(w int, now time.Time) string {
	row, ok := m.sidebar.Selected()
	if !ok {
		return padRight("", w)
	}
	st := m.status[row.Session.ID]
	parts := row.Session.Title
	if st.Status != "" {
		parts += " · " + glyphStyle(st.Status).Render(statusGlyph(st.Status)+" "+string(st.Status))
	}
	if age := AgeString(now, st.At); age != "" {
		parts += " " + age
	}
	if st.Tokens.In+st.Tokens.Out > 0 {
		parts += " · " + mutedStyle.Render(KString(st.Tokens.In)+" in / "+KString(st.Tokens.Out)+" out")
	}
	return headerStyle.Render(fitLine(parts, w))
}

// renderFooter shows the keymap, or the last error until the next keypress.
// Errors live here rather than in a pane so they are visible whether or not
// a session has focus.
func (m *Model) renderFooter() string {
	if m.lastErr != "" {
		return errorStyle.Render(fitLine(" error: "+m.lastErr, m.width))
	}
	return footerStyle.Render(padRight(" "+footer, m.width))
}

// renderRow draws one sidebar line: a project header, or a session with its
// focus marker and coloured status glyph.
func (m *Model) renderRow(row Row, width int, now time.Time) string {
	if row.Session == nil {
		return mutedStyle.Render(padRight("> "+row.Project, width))
	}
	marker := "  "
	if sel, ok := m.sidebar.Selected(); ok && sel.Session.ID == row.Session.ID {
		marker = "» "
	}
	glyph := glyphStyle(row.Status).Render(statusGlyph(row.Status))
	age := ""
	if st, ok := m.status[row.Session.ID]; ok {
		age = AgeString(now, st.At)
	}
	title := padRight(row.Session.Title, width-4-len(age)-1)
	return marker + glyph + " " + title + " " + mutedStyle.Render(age)
}

// fitBlock forces lines to exactly width x height so a border lands
// precisely: short lines are padded, long ones cut, missing rows added.
func fitBlock(lines []string, width, height int) string {
	out := make([]string, height)
	for i := range out {
		if i < len(lines) {
			out[i] = fitLine(lines[i], width)
			continue
		}
		out[i] = strings.Repeat(" ", width)
	}
	return strings.Join(out, "\n")
}

func fitLine(s string, width int) string {
	if lipgloss.Width(s) > width {
		return lipgloss.NewStyle().MaxWidth(width).Render(s)
	}
	return padRight(s, width)
}

// padRight is ANSI-aware: it measures visible width, not bytes.
func padRight(s string, width int) string {
	if n := width - lipgloss.Width(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
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
	case registry.StatusExited:
		return "∅"
	default:
		return "-"
	}
}
