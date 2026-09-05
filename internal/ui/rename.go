// Renaming a session in place (#41): the editor's commit and the dependency
// that persists it. The title is display-only, so this is a state.json edit
// and a sidebar rebuild - nothing here touches what relaunching a session
// depends on (invariant 9).

package ui

import (
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"
)

// RenameFunc persists a session's new title. Injected so ui never reaches the
// registry store itself; cmd/omatty closes it over the store.
//
//	deps.Rename = func(id, title string) error {
//	        return registry.RenameSession(store, id, title)
//	}
type RenameFunc func(sessionID, title string) error

// noRename is the Deps.Rename default. It names the missing wiring rather than
// appearing to succeed, which would leave the sidebar showing a title that
// state.json does not have - the same reasoning as noDiff and noFiles.
func noRename(sessionID, title string) error {
	return fmt.Errorf("ui: no rename source configured for session %s (title %q)", sessionID, title)
}

// openRename starts editing the selected session's title, pre-filled so that
// correcting a typo is a small edit rather than a retype.
func (m *Model) openRename() {
	row, ok := m.sidebar.Selected()
	if !ok {
		return
	}
	m.modal = modal{
		Kind:   modalRename,
		Editor: lineEditor{Target: row.Session.ID, Buffer: row.Session.Title},
	}
}

// commitRename persists the new title and rebuilds the sidebar. The editor
// closes first, so a failed rename surfaces in the footer against the pane the
// operator came from rather than behind a box still asking for input.
func (m *Model) commitRename() tea.Cmd {
	id, title := m.modal.Editor.Target, m.modal.Editor.Buffer
	m.modal = modal{}
	m.lastErr = ""
	if err := m.rename(id, title); err != nil {
		slog.Error("renaming session", "session", id, "title", title, "err", err)
		m.lastErr = err.Error()
		return nil
	}
	if !m.retitle(id, title) {
		slog.Error("renaming session", "session", id, "title", title, "err", "renamed on disk but not in memory")
		m.lastErr = fmt.Sprintf("session %s was renamed on disk but is not in the sidebar; reload to see it", id)
		return nil
	}
	// SetRows, not NewSidebar: it re-finds the selection by id, so the row you
	// just renamed is still the row you are on.
	m.sidebar.SetRows(SidebarRows(m.state, m.statusMap()))
	return nil
}

// retitle updates the in-memory state the sidebar is rebuilt from, so the new
// name shows without waiting for a reload.
//
// A miss is reported rather than swallowed: the rename has already succeeded on
// disk at this point, so a silent return would leave the sidebar showing the
// old title with no error anywhere (#41).
func (m *Model) retitle(id, title string) bool {
	i, ok := m.sessionIndex(id)
	if !ok {
		return false
	}
	m.state.Sessions[i].Title = title
	return true
}
