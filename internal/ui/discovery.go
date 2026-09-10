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
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// Proposal is one repository discovery offers, with when it was last worked
// in. LastUsed is what orders the list, so the picker shows it rather than
// leaving the operator to wonder why six near-identical names are in that
// order (#91).
type Proposal struct {
	Name     string
	Root     string
	LastUsed time.Time
}

// DiscoverFunc proposes repositories to register, newest first. Injected
// because ui may neither read the transcript store nor shell out to git.
type DiscoverFunc func() ([]Proposal, error)

// AddProjectFunc registers repositories by root and reports what became of
// each. It takes the whole batch rather than one root at a time so the
// register-report-carry-on loop lives in registry, where `omatty discover`
// shares it (#91).
type AddProjectFunc func(roots []string) []registry.Registration

// noDiscover is the Deps.Discover default: it names the missing wiring rather
// than proposing an empty list, which would read as "you have never used
// claude anywhere".
func noDiscover() ([]Proposal, error) {
	return nil, fmt.Errorf("ui: no project discovery configured")
}

// noAddProject is the Deps.AddProject default, for the same reason.
func noAddProject(roots []string) []registry.Registration {
	out := make([]registry.Registration, 0, len(roots))
	for _, root := range roots {
		out = append(out, registry.Registration{
			Root: root,
			Err:  fmt.Errorf("ui: no project registrar configured for %q", root),
		})
	}
	return out
}

// ProjectsProposedMsg carries discovery's result into Update. Exported so
// tests can send one.
//
// Token identifies the scan that produced it. Two scans can be in flight -
// ctrl+o a, esc, ctrl+o a - and without this the slower one overwrote the
// faster one's list, wiping any marks the operator had already made (#91).
type ProjectsProposedMsg struct {
	Token     int
	Proposals []Proposal
	Err       error
}

// openDiscovery opens the picker straight away, on a placeholder, and scans in
// the background. The scan is a git call per slug directory - some thirty on a
// well-used machine - so doing it inline would stall the frame, and doing it
// before opening anything would leave the key looking dead for a second.
func (m *Model) openDiscovery() tea.Cmd {
	m.scanToken++
	token := m.scanToken
	m.openModal(modal{
		Kind: modalPicker,
		List: newPickList("scanning for repositories", nil, true),
		Scan: token,
	})
	propose := m.discover
	return func() tea.Msg {
		proposals, err := propose()
		return ProjectsProposedMsg{Token: token, Proposals: proposals, Err: err}
	}
}

// onProjectsProposed fills the picker. A result arriving after the picker
// closed is dropped: the operator moved on, and reopening the box under them
// would be worse than losing the scan. A result from an older scan is dropped
// for the same reason - it would overwrite a newer list, marks and all.
func (m *Model) onProjectsProposed(msg ProjectsProposedMsg) tea.Cmd {
	if m.modal.Kind != modalPicker || msg.Token != m.modal.Scan {
		return nil
	}
	if msg.Err != nil {
		slog.Error("discovering projects", "err", msg.Err)
		m.modal, m.lastErr = modal{}, msg.Err.Error()
		return nil
	}
	items := make([]pickItem, 0, len(msg.Proposals))
	for _, p := range msg.Proposals {
		items = append(items, pickItem{ID: p.Root, Label: p.Name, Detail: proposalDetail(p, m.clock())})
	}
	list := newPickList(discoveryTitle(len(items)), items, true)
	// The query typed while the scan ran is kept: the placeholder renders a
	// live query line and editList accepts into it, so resetting it here threw
	// away letters the operator watched themselves type (#91).
	list.SetQuery(m.modal.List.Query)
	m.modal.List = list
	return nil
}

// proposalDetail is the row's second column: the root, and how long since the
// repository was worked in - which is the order the list is already in, so
// without it the ordering is unexplained.
func proposalDetail(p Proposal, now time.Time) string {
	age := AgeString(now, p.LastUsed)
	if age == "" {
		return p.Root
	}
	return p.Root + "  " + age
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
	roots := make([]string, 0, len(picked))
	for _, it := range picked {
		roots = append(roots, it.ID)
	}
	// The Project that was actually written, never one rebuilt from the picked
	// row: discovery names a candidate after MainCheckout's directory and
	// AddProject after RepoRoot's, and where those disagree the sidebar showed
	// a project name state.json did not have - so ctrl+o n on it failed, and a
	// restart silently renamed the row (#91).
	for _, r := range m.registerProjects(roots) {
		if r.Err != nil {
			slog.Warn("registering a discovered project", "root", r.Root, "err", r.Err)
			m.lastErr = r.Err.Error()
			continue
		}
		m.state.Projects = append(m.state.Projects, r.Project)
	}
	// A new project has no sessions yet, so the cursor does not move; the
	// operator's next act is ctrl+o n.
	m.sidebar.SetRows(SidebarRows(m.state, m.statusMap()))
	return nil
}

// pickerFooter names the marking key, which is the picker's one difference
// from the switcher.
func pickerFooter(marked int) string { return markFooter(marked, "register") }

// markFooter is both pickers' footer, with the verb the enter key performs
// passed in. One function rather than two near-copies, so the two cannot say
// different things about the same key (#122).
func markFooter(marked int, verb string) string {
	if marked == 0 {
		return "type to filter  ctrl+j/ctrl+k move  tab mark  enter " + verb + "  esc cancel"
	}
	return "marked " + strconv.Itoa(marked) + "  tab mark  enter " + verb + "  esc cancel"
}
