// Removing a project (#159): the confirmation and the registry edit. It is
// reachable only from an empty project's header, which is the only header
// the cursor can rest on (#158) - so the "no sessions" rule the registry
// enforces is already true on the way in, and the key is x because x is
// "be rid of the selected row" for a session too.

package app

import (
	"fmt"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"strconv"

	tea "charm.land/bubbletea/v2"
)

// RemoveProjectFunc forgets a project. Injected so ui never reaches the store;
// cmd/omatty closes it over sessions.RemoveProject.
//
//	deps.RemoveProject = func(name string) (session.Project, error) {
//	        return sessions.RemoveProject(store, name)
//	}
type RemoveProjectFunc func(name string) (session.Project, error)

// noRemoveProject is the Deps.RemoveProject default. It names the missing
// wiring rather than appearing to succeed, as noArchive does.
func noRemoveProject(name string) (session.Project, error) {
	return session.Project{}, fmt.Errorf("ui: no project remover configured for project %q", name)
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
	remove := m.removeProject
	write := func() error {
		_, err := remove(name)
		return err
	}
	return m.persistCmd("removing project", []any{"project", name}, write, func(m *Model) tea.Cmd {
		m.forgetProject(name)
		m.sidebar.SetRows(SidebarRows(m.state, m.statusMap()))
		return tea.Batch(m.resizeSelected(), m.followSession())
	})
}

// forgetProject drops the project from the in-memory state. A fresh slice,
// for the reason forgetSession builds one: the sidebar's rows alias the
// backing arrays of m.state.
func (m *Model) forgetProject(name string) {
	kept := make([]session.Project, 0, len(m.state.Projects))
	for _, p := range m.state.Projects {
		if p.Name != name {
			kept = append(kept, p)
		}
	}
	m.state.Projects = kept
	delete(m.prs, name) // the project's pull requests (#310)
	delete(m.prPending, name)
	delete(m.prFailed, name)
	delete(m.forgeStopped, name)
	delete(m.forgeRetry, name)
	delete(m.prAsked, name)
	m.forgetProjectIssues(name) // and its issues (#394)
	m.forgetProjectItems(name)  // and any item read in full (#397)
}
