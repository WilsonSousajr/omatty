// Removing a project (#159): the confirmation and the registry edit. It is
// reachable only from an empty project's header, which is the only header
// the cursor can rest on (#158) - so the "no sessions" rule the registry
// enforces is already true on the way in, and the key is x because x is
// "be rid of the selected row" for a session too.

package ui

import (
	"fmt"
	"log/slog"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// RemoveProjectFunc forgets a project. Injected so ui never reaches the store;
// cmd/omatty closes it over registry.RemoveProject.
//
//	deps.RemoveProject = func(name string) (registry.Project, error) {
//	        return registry.RemoveProject(store, name)
//	}
type RemoveProjectFunc func(name string) (registry.Project, error)

// noRemoveProject is the Deps.RemoveProject default. It names the missing
// wiring rather than appearing to succeed, as noArchive does.
func noRemoveProject(name string) (registry.Project, error) {
	return registry.Project{}, fmt.Errorf("ui: no project remover configured for project %q", name)
}

// openRemoveProject asks before forgetting the empty project under the cursor.
func (m *Model) openRemoveProject(name string) {
	m.openModal(modal{Kind: modalConfirm, Confirm: confirmBox{
		Project:  name,
		Question: "remove project " + strconv.Quote(name) + " from omatty?",
		Note:     "the repository stays on disk, untouched; only omatty forgets it",
		Choices:  []confirmChoice{{Key: "y", Label: "remove"}},
	}})
}

// removeProjectRow forgets the project, then drops its header. The registry
// edit comes first for the reason archiveSession's does: a failed save must
// leave the row where it was.
func (m *Model) removeProjectRow() tea.Cmd {
	name := m.modal.Confirm.Project
	m.modal, m.lastErr = modal{}, ""
	if _, err := m.removeProject(name); err != nil {
		slog.Error("removing project", "project", name, "err", err)
		m.lastErr = err.Error()
		return nil
	}
	m.forgetProject(name)
	m.sidebar.SetRows(SidebarRows(m.state, m.statusMap()))
	return tea.Batch(m.resizeSelected(), m.followSession())
}

// forgetProject drops the project from the in-memory state. A fresh slice,
// for the reason forgetSession builds one: the sidebar's rows alias the
// backing arrays of m.state.
func (m *Model) forgetProject(name string) {
	kept := make([]registry.Project, 0, len(m.state.Projects))
	for _, p := range m.state.Projects {
		if p.Name != name {
			kept = append(kept, p)
		}
	}
	m.state.Projects = kept
}
