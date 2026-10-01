// Folding a project in the sidebar (#505): its sessions go away behind its
// header, as a directory does in the file tree, and come back on the same key.

package app

import (
	"fmt"
	"log/slog"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// FoldFunc persists whether a project is folded. Injected so ui never reaches
// the store; cmd/omatty closes it over registry.SetCollapsed.
//
//	deps.Fold = func(project string, collapsed bool) error {
//	        return registry.SetCollapsed(store, project, collapsed)
//	}
type FoldFunc func(project string, collapsed bool) error

// noFold is the Deps.Fold default. It names the missing wiring rather than
// appearing to succeed, as noRemoveProject does.
func noFold(project string, collapsed bool) error {
	return fmt.Errorf("ui: no fold store configured for project %q (collapsed=%v)", project, collapsed)
}

// foldedHeader is the one row a folded project draws: its header, carrying
// the sessions it hides and the loudest of their statuses.
func foldedHeader(project string, sessions []Row) Row {
	h := Row{Project: project, Status: status.StatusIdle}
	for _, r := range sessions {
		h.Folded = append(h.Folded, r.Session)
		if loudness(r.Status) > loudness(h.Status) {
			h.Status = r.Status
		}
	}
	return h
}

// loudness ranks a status by how much it wants the operator: a question
// first, then a failure, then a finished turn, then work in flight, then rest.
func loudness(s status.Status) int {
	switch s {
	case status.StatusWaiting:
		return 4
	case status.StatusError:
		return 3
	case status.StatusDone:
		return 2
	case status.StatusThinking, status.StatusTool:
		return 1
	}
	return 0
}

// toggleFold folds project, or unfolds it when it is folded: ctrl+o tab on the
// project the cursor is in, and a click on its header. Folding leaves the
// cursor on the header, the one row left that names the project; unfolding
// puts it on the first session. A project with no sessions has nothing to
// fold, so it is left alone.
func (m *Model) toggleFold(project string) tea.Cmd {
	i := m.foldable(project)
	if i < 0 {
		return nil
	}
	collapsed, fold := !m.state.Projects[i].Collapsed, m.fold
	// Saved before shown, off the Update goroutine (#653): a fold that could
	// not be written must not appear to have happened.
	return m.persistCmd("saving a project fold", []any{"project", project, "collapsed", collapsed},
		func() error { return fold(project, collapsed) },
		func(m *Model) tea.Cmd { return m.folded(project, collapsed) })
}

// folded shows a fold that is on disk.
func (m *Model) folded(project string, collapsed bool) tea.Cmd {
	i := m.foldable(project)
	if i < 0 {
		return nil
	}
	m.state.Projects[i].Collapsed = collapsed
	m.sidebar.SetRows(SidebarRows(m.state, m.statusMap()))
	m.sidebar.SelectByProject(project)
	// The pair commitJump uses: size what we landed on and drag an open
	// review column along (#73, #95).
	return tea.Batch(m.resizeSelected(), m.followSession())
}

// foldable is the index of project in m.state when it has a session to fold,
// and -1 otherwise.
func (m *Model) foldable(project string) int {
	if project == "" || !m.hasSessions(project) {
		return -1
	}
	for i, p := range m.state.Projects {
		if p.Name == project {
			return i
		}
	}
	return -1
}

func (m *Model) hasSessions(project string) bool {
	for _, s := range m.state.Sessions {
		if s.Project == project {
			return true
		}
	}
	return false
}

// persistFold saves the fold before showing it, for the reason
// removeProjectRow saves first: a fold that could not be written must not
// appear to have happened, or the next launch contradicts the screen.
func (m *Model) persistFold(i int, collapsed bool) bool {
	name := m.state.Projects[i].Name
	if err := m.fold(name, collapsed); err != nil {
		slog.Error("saving a project fold", "project", name, "collapsed", collapsed, "err", err)
		m.lastErr = err.Error()
		return false
	}
	m.state.Projects[i].Collapsed = collapsed
	return true
}

// revealSession puts the cursor on a session, unfolding its project first when
// the fold hides it. Every path that lands on a named session - the switcher,
// a created or adopted one - goes through here, so reaching a folded session
// cannot be written twice and forgotten once.
func (m *Model) revealSession(id string) bool {
	if m.sidebar.SelectByID(id) {
		return true
	}
	i := m.foldable(m.projectOfSession(id))
	if i < 0 || !m.state.Projects[i].Collapsed || !m.persistFold(i, false) {
		return false
	}
	m.sidebar.SetRows(SidebarRows(m.state, m.statusMap()))
	return m.sidebar.SelectByID(id)
}

// projectOfSession is the project a registered session belongs to, or "".
func (m *Model) projectOfSession(id string) string {
	for _, s := range m.state.Sessions {
		if s.ID == id {
			return s.Project
		}
	}
	return ""
}
