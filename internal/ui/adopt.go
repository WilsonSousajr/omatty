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

// AdoptCommitFunc registers each pick and reports what became of it, one result
// per proposal in the order given.
//
// The whole batch rather than one at a time, for the reason AddProjectFunc
// takes one: the register-report-carry-on loop lives in the registry beside the
// subcommand that shares it, so one collision does not abandon the rest (#91).
//
// It returns the Session that was written and not merely an error, for the
// reason AddProjectFunc returns Registration: the picker has to use the row
// state.json actually holds. Rebuilding one from the pick matched only while
// AdoptSession stored the pick's fields verbatim, and it no longer does - it
// fills in the branch a worktree session is on (#91, #122).
type AdoptCommitFunc func(project string, picks []SessionProposal) []registry.Adoption

// noAdopt is the Deps.AdoptPropose default: it names the missing wiring rather
// than proposing an empty list, which would read as "this project has no
// sessions to adopt".
func noAdopt(projectRoot string) ([]SessionProposal, error) {
	return nil, fmt.Errorf("ui: no session adoption configured for %q", projectRoot)
}

// noAdoptCommit is the Deps.AdoptCommit default, for the same reason: a silent
// success would put a row in the sidebar that state.json does not hold.
func noAdoptCommit(_ string, picks []SessionProposal) []registry.Adoption {
	out := make([]registry.Adoption, 0, len(picks))
	for _, p := range picks {
		out = append(out, registry.Adoption{
			Err: fmt.Errorf("ui: no session registrar configured for %q", p.ID),
		})
	}
	return out
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
	// Cleared with the scan that replaces it. Left standing, the previous
	// project's proposals stayed reachable for the life of the process and -
	// worse as state - pickedProposals resolved the open picker's rows against
	// a scan that no longer described it (#122).
	m.adoptable = nil
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

// liveWindow is how recently a transcript must have been written for the picker
// to say the session may still be running somewhere else.
//
// Adopting a session that is still open in a plain terminal starts a second
// `claude --resume` against the same transcript: both processes append to one
// JSONL, the tailer reads interleaved turns, the status glyph flaps and
// whichever process wrote last wins. registry.refuseKnownSession guards the
// case where both rows are omatty's; nothing here can see a process omatty did
// not start, and the README sells adoption as the way to pick up a session
// begun in a plain terminal - which is exactly the terminal likely to still be
// open (#122).
//
// Two minutes, because this is the transcript's mtime and not a liveness check:
// a session idle longer than that is indistinguishable from one that ended, so
// the picker says what it can rather than claiming more than it knows.
const liveWindow = 2 * time.Minute

// proposedSessionDetail is the row's second column: how long since the session
// was worked in, which is the order the list is already in, and a warning when
// that is recent enough that something else may still be attached to it.
func proposedSessionDetail(p SessionProposal, now time.Time) string {
	detail := p.Dir
	if age := AgeString(now, p.LastUsed); age != "" {
		detail = age
	}
	if now.Sub(p.LastUsed) < liveWindow {
		return detail + " · may still be running elsewhere"
	}
	return detail
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
	for i, a := range m.adoptCommit(project, picked) {
		if a.Err != nil {
			slog.Warn("adopting a session", "session", picked[i].ID, "err", a.Err)
			m.lastErr = a.Err.Error()
			continue
		}
		cmds = append(cmds, m.startAdopted(a.Session))
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
//
// It takes the Session the registry wrote, never one rebuilt from the picked
// row. Where the two disagreed, the sidebar showed a value state.json did not
// have, `ctrl+o n` on it failed and a restart silently renamed the row (#91) -
// and they do disagree now, because AdoptSession fills in the branch.
func (m *Model) startAdopted(sess registry.Session) tea.Cmd {
	cmd, err := m.foldInSession(sess)
	if err != nil {
		slog.Error("starting an adopted session", "session", sess.ID, "dir", sess.Dir, "err", err)
		m.lastErr = err.Error()
		return nil
	}
	return cmd
}

// adoptFooter names the marking key, as the project picker's does.
//
// The verb is passed down rather than left to pickerFooter: delegating to it
// once a row was marked flipped "enter adopt" to "enter register" at the exact
// moment the operator was about to commit (#122).
func adoptFooter(marked int) string { return markFooter(marked, "adopt") }
