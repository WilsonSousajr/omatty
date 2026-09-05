// The project picker (#91): the same list as the switcher, over the
// repositories claude has been used in rather than over sessions.
//
// Discovery only ever proposes. Nothing here writes to the registry without
// the operator marking a row and pressing enter, so state.json stays the
// single source of truth (invariant 9).

package ui

import (
	"fmt"
	"log/slog"
	"strconv"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// DiscoverFunc proposes repositories to register, newest first. Injected
// because ui may neither read the transcript store nor shell out to git.
type DiscoverFunc func() ([]registry.Project, error)

// AddProjectFunc registers one repository by its root.
type AddProjectFunc func(root string) error

// noDiscover is the Deps.Discover default: it names the missing wiring rather
// than proposing an empty list, which would read as "you have never used
// claude anywhere".
func noDiscover() ([]registry.Project, error) {
	return nil, fmt.Errorf("ui: no project discovery configured")
}

// noAddProject is the Deps.AddProject default, for the same reason.
func noAddProject(root string) error {
	return fmt.Errorf("ui: no project registrar configured for %q", root)
}

// ProjectsProposedMsg carries discovery's result into Update. Exported so
// tests can send one.
type ProjectsProposedMsg struct {
	Projects []registry.Project
	Err      error
}

// openDiscovery opens the picker straight away, on a placeholder, and scans in
// the background. The scan is a git call per slug directory - some thirty on a
// well-used machine - so doing it inline would stall the frame, and doing it
// before opening anything would leave the key looking dead for a second.
func (m *Model) openDiscovery() tea.Cmd {
	m.modal = modal{
		Kind: modalPicker,
		List: newPickList("scanning for repositories", nil, true),
	}
	propose := m.discover
	return func() tea.Msg {
		projects, err := propose()
		return ProjectsProposedMsg{Projects: projects, Err: err}
	}
}

// onProjectsProposed fills the picker. A result arriving after the picker
// closed is dropped: the operator moved on, and reopening the box under them
// would be worse than losing the scan.
func (m *Model) onProjectsProposed(msg ProjectsProposedMsg) tea.Cmd {
	if m.modal.Kind != modalPicker {
		return nil
	}
	if msg.Err != nil {
		slog.Error("discovering projects", "err", msg.Err)
		m.modal, m.lastErr = modal{}, msg.Err.Error()
		return nil
	}
	items := make([]pickItem, 0, len(msg.Projects))
	for _, p := range msg.Projects {
		items = append(items, pickItem{ID: p.Root, Label: p.Name, Detail: p.Root})
	}
	m.modal.List = newPickList(discoveryTitle(len(items)), items, true)
	return nil
}

// discoveryTitle says what the list is once it is known, including the empty
// case, which is a real answer rather than a failure.
func discoveryTitle(n int) string {
	if n == 0 {
		return "no repositories found"
	}
	return "register project (tab to mark)"
}

// commitDiscovery registers every marked repository, or the row under the
// cursor when nothing is marked.
//
// A collision is reported against the one candidate it belongs to and the rest
// carry on: AddProject refuses a duplicate *name* even when the roots differ,
// which one repository at a time is a rare annoyance and in bulk is not (#91).
func (m *Model) commitDiscovery() tea.Cmd {
	picked := m.modal.List.Chosen()
	if len(picked) == 0 {
		return nil
	}
	m.modal, m.lastErr = modal{}, ""
	// The picked item already carries the name and root discovery resolved,
	// and both AddProject and discovery name a project after its root's own
	// directory, so the two agree by construction. Re-scanning to find out
	// would put a git call per slug directory back on the Update goroutine.
	for _, it := range picked {
		if err := m.addProject(it.ID); err != nil {
			slog.Warn("registering a discovered project", "root", it.ID, "err", err)
			m.lastErr = err.Error()
			continue
		}
		m.state.Projects = append(m.state.Projects, registry.Project{Name: it.Label, Root: it.ID})
	}
	// A new project has no sessions yet, so the cursor does not move; the
	// operator's next act is ctrl+o n.
	m.sidebar.SetRows(SidebarRows(m.state, m.statusMap()))
	return nil
}

// pickerFooter names the marking key, which is the picker's one difference
// from the switcher.
func pickerFooter(marked int) string {
	if marked == 0 {
		return "type to filter  ctrl+j/ctrl+k move  tab mark  enter register  esc cancel"
	}
	return "marked " + strconv.Itoa(marked) + "  tab mark  enter register  esc cancel"
}
