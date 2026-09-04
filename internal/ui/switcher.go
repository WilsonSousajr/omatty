// The fuzzy session switcher (#42): type a few letters, jump to a session
// anywhere in the registry. j/k walks the sidebar in order, which stops
// scaling somewhere around eight sessions and cannot cross projects without
// walking every row in between.

package ui

import (
	tea "charm.land/bubbletea/v2"
)

// openSwitcher lists every session across every project, in sidebar order, so
// an empty query shows exactly what the sidebar shows.
func (m *Model) openSwitcher() tea.Cmd {
	items := make([]pickItem, 0, len(m.state.Sessions))
	for _, row := range SidebarRows(m.state, m.statusMap()) {
		if row.Session == nil {
			continue // a project header is a label, never a target
		}
		items = append(items, pickItem{
			ID:     row.Session.ID,
			Label:  row.Session.Title,
			Detail: row.Project,
		})
	}
	if len(items) == 0 {
		return nil
	}
	m.modal = modal{Kind: modalList, List: newPickList(items, false)}
	return nil
}

// onListKey drives the open list: the keys that end it, and everything else.
func (m *Model) onListKey(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.Keystroke() {
	case "esc":
		m.modal = modal{}
	case "enter":
		return m.commitList()
	case "tab":
		m.modal.List.ToggleMark()
	default:
		m.editList(msg)
	}
	return nil
}

// editList moves the cursor or edits the query.
//
// j and k cannot move here: they are filter text, and a switcher you cannot
// type "jk" into is not a switcher. Movement is the arrows plus ctrl+j and
// ctrl+k, which the modal footer says out loud because it is the one place M4
// departs from the sidebar's own keymap.
func (m *Model) editList(msg tea.KeyPressMsg) {
	l := &m.modal.List
	switch msg.Keystroke() {
	case "up", "ctrl+k":
		l.Move(-1, m.pickRows())
	case "down", "ctrl+j":
		l.Move(1, m.pickRows())
	case "backspace":
		l.SetQuery(trimLastRune(l.Query))
	default:
		l.SetQuery(l.Query + msg.Text)
	}
}

// commitList jumps to the chosen session. A query that matches nothing leaves
// the list open rather than closing on a choice that was never made.
func (m *Model) commitList() tea.Cmd {
	chosen, ok := m.modal.List.Current()
	if !ok {
		return nil
	}
	m.modal = modal{}
	if !m.sidebar.SelectByID(chosen.ID) {
		return nil
	}
	// The same pair moveCursor uses: size what we landed on and drag an open
	// review column along (#73, #95).
	return tea.Batch(m.resizeSelected(), m.followSession())
}
