// The adoption picker (#122): the same list as project discovery, over the
// claude sessions inside one registered project rather than over repositories.
//
// It is `ctrl+o A` because `ctrl+o a` is already discovery, and the shifted
// letter matches the n / N pair the new-session prompt uses.
//
// Like discovery, it only ever proposes. Nothing here writes to the registry
// without the operator marking a row and pressing enter, so state.json stays
// the single source of truth (invariant 9).

package ui

import (
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// SessionProposal is one claude session adoption offers.
//
// Dir is the working directory the transcript recorded, not the project root:
// the two differ for a session that ran in a linked worktree, and Dir is where
// the adopted session has to be started.
type SessionProposal struct {
	ID       string
	Title    string
	Dir      string
	LastUsed time.Time
}

// AdoptFunc proposes the sessions inside one project. Injected because ui may
// neither read the transcript store nor shell out to git (invariant 4).
type AdoptFunc func(projectRoot string) ([]SessionProposal, error)

// AdoptCommitFunc registers each pick and reports what became of it, one error
// per proposal in the order given, nil where it succeeded.
//
// The whole batch rather than one at a time, for the reason AddProjectFunc
// takes one: the register-report-carry-on loop lives in cmd beside the
// subcommand that shares it, so one collision does not abandon the rest (#91).
type AdoptCommitFunc func(project string, picks []SessionProposal) []error

// noAdopt is the Deps.AdoptPropose default: it names the missing wiring rather
// than proposing an empty list, which would read as "this project has no
// sessions to adopt".
func noAdopt(projectRoot string) ([]SessionProposal, error) {
	return nil, fmt.Errorf("ui: no session adoption configured for %q", projectRoot)
}

// noAdoptCommit is the Deps.AdoptCommit default, for the same reason: a silent
// success would put a row in the sidebar that state.json does not hold.
func noAdoptCommit(_ string, picks []SessionProposal) []error {
	errs := make([]error, 0, len(picks))
	for _, p := range picks {
		errs = append(errs, fmt.Errorf("ui: no session registrar configured for %q", p.ID))
	}
	return errs
}

// SessionsProposedMsg carries an adoption scan's result into Update. Exported
// so tests can send one.
//
// Token identifies the scan, for the reason ProjectsProposedMsg carries one:
// two scans can be in flight, and without it the slower would overwrite the
// faster one's list, wiping any marks already made (#91).
type SessionsProposedMsg struct {
	Token     int
	Proposals []SessionProposal
	Err       error
}

// openAdoption opens the picker on a placeholder and scans in the background.
//
// The scan reads a transcript head per session in the project, so doing it
// inline would stall the frame and doing it before opening anything would leave
// the key looking dead (#91).
func (m *Model) openAdoption() tea.Cmd {
	root := m.projectRoot(m.SelectedProject())
	if root == "" {
		m.lastErr = "no project selected; register one with " + Leader + " a first"
		return nil
	}
	m.scanToken++
	token := m.scanToken
	m.openModal(modal{
		Kind: modalAdopt,
		List: newPickList("scanning for sessions", nil, true),
		Scan: token,
	})
	propose := m.adoptPropose
	return func() tea.Msg {
		proposals, err := propose(root)
		return SessionsProposedMsg{Token: token, Proposals: proposals, Err: err}
	}
}

// onSessionsProposed fills the picker, dropping a result that arrived after the
// picker closed or from an older scan - the operator moved on, and reopening
// the box under them would be worse than losing the scan (#91).
func (m *Model) onSessionsProposed(msg SessionsProposedMsg) tea.Cmd {
	if m.modal.Kind != modalAdopt || msg.Token != m.modal.Scan {
		return nil
	}
	if msg.Err != nil {
		slog.Error("proposing sessions to adopt", "err", msg.Err)
		m.modal, m.lastErr = modal{}, msg.Err.Error()
		return nil
	}
	items := make([]pickItem, 0, len(msg.Proposals))
	for _, p := range msg.Proposals {
		items = append(items, pickItem{ID: p.ID, Label: p.Title, Detail: proposedSessionDetail(p, m.clock())})
	}
	list := newPickList(adoptionTitle(len(items)), items, true)
	// The query typed while the scan ran is kept, for the reason discovery
	// keeps it: the placeholder accepts text, so resetting it here would throw
	// away letters the operator watched themselves type (#91).
	list.SetQuery(m.modal.List.Query)
	m.modal.List = list
	m.adoptable = msg.Proposals
	return nil
}

// proposedSessionDetail is the row's second column: how long since the session
// was worked in, which is the order the list is already in.
func proposedSessionDetail(p SessionProposal, now time.Time) string {
	if age := AgeString(now, p.LastUsed); age != "" {
		return age
	}
	return p.Dir
}

// adoptionTitle says what the list is once it is known, including the empty
// case, which is a real answer rather than a failure.
func adoptionTitle(n int) string {
	if n == 0 {
		return "no unregistered sessions in this project"
	}
	return "adopt session (tab to mark)"
}

// commitAdoption registers every marked session, or the row under the cursor
// when nothing is marked, and starts each one.
func (m *Model) commitAdoption() tea.Cmd {
	picked := m.pickedProposals()
	if len(picked) == 0 {
		return nil
	}
	m.modal, m.lastErr = modal{}, ""
	project := m.SelectedProject()
	cmds := make([]tea.Cmd, 0, len(picked))
	for i, err := range m.adoptCommit(project, picked) {
		if err != nil {
			slog.Warn("adopting a session", "session", picked[i].ID, "err", err)
			m.lastErr = err.Error()
			continue
		}
		cmds = append(cmds, m.startAdopted(picked[i], project))
	}
	return tea.Batch(cmds...)
}

// pickedProposals resolves the marked rows back to the proposals they came
// from.
//
// The proposal, not a value rebuilt from the row: a pickItem carries a label
// and a detail string, and adoption needs the working directory, which the
// detail column does not always show (#122).
func (m *Model) pickedProposals() []SessionProposal {
	byID := make(map[string]SessionProposal, len(m.adoptable))
	for _, p := range m.adoptable {
		byID[p.ID] = p
	}
	picked := make([]SessionProposal, 0, len(m.adoptable))
	for _, it := range m.modal.List.Chosen() {
		if p, ok := byID[it.ID]; ok {
			picked = append(picked, p)
		}
	}
	return picked
}

// startAdopted brings an adopted session up: its terminal, its tailer, its
// sidebar row. The launcher resumes it, because its transcript exists (#36).
func (m *Model) startAdopted(p SessionProposal, project string) tea.Cmd {
	sess := registry.Session{ID: p.ID, Project: project, Title: p.Title, Dir: p.Dir}
	cmd, err := m.foldInSession(sess)
	if err != nil {
		slog.Error("starting an adopted session", "session", sess.ID, "dir", sess.Dir, "err", err)
		m.lastErr = err.Error()
		return nil
	}
	return cmd
}

// adoptFooter names the marking key, as the project picker's does.
func adoptFooter(marked int) string {
	if marked == 0 {
		return "type to filter  ctrl+j/ctrl+k move  tab mark  enter adopt  esc cancel"
	}
	return pickerFooter(marked)
}
