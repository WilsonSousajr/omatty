package ui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// recordAdopt is a named fake for the two adoption dependencies, recording what
// the picker scanned and what it committed.
type recordAdopt struct {
	Proposed   []ui.SessionProposal
	ProposeErr error
	// ScannedRoot is the project root the picker asked about, which is what
	// scopes adoption to the session under the cursor (#122).
	ScannedRoot string
	Adopted     []string
	AdoptErr    error
	// Project is the project name the picks were registered under.
	Project string
	// Started and Tailed are the ids whose terminal and tailer were begun, so
	// one test can assert an adopted session is actually running.
	Started []string
	Tailed  []string
}

func (r *recordAdopt) propose(projectRoot string) ([]ui.SessionProposal, error) {
	r.ScannedRoot = projectRoot
	return r.Proposed, r.ProposeErr
}

func (r *recordAdopt) adopt(project string, picks []ui.SessionProposal) []error {
	r.Project = project
	errs := make([]error, 0, len(picks))
	for _, p := range picks {
		r.Adopted = append(r.Adopted, p.ID)
		errs = append(errs, r.AdoptErr)
	}
	return errs
}

func (r *recordAdopt) start(sess registry.Session, _, _ int) (termwrap.Terminal, error) {
	r.Started = append(r.Started, sess.ID)
	return termwrap.NewFake("adopted"), nil
}

func (r *recordAdopt) tail(sess registry.Session) { r.Tailed = append(r.Tailed, sess.ID) }

func twoProposals() []ui.SessionProposal {
	now := time.Now()
	return []ui.SessionProposal{
		{ID: "a1", Title: "fix the parser", Dir: "/p/omatty", LastUsed: now},
		{ID: "a2", Title: "chase a flake", Dir: "/p/omatty", LastUsed: now.Add(-48 * time.Hour)},
	}
}

func modelWithAdopt(t *testing.T, r *recordAdopt) *ui.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.AdoptPropose, d.AdoptCommit = r.propose, r.adopt
	d.Start, d.TailStart = r.start, r.tail
	return ui.NewModel(d)
}

// openAdoption presses the key and delivers the scan, which arrives as a
// command rather than inline.
func openAdoption(m *ui.Model) {
	press(m, ctrl('o'))
	_, cmd := m.Update(key('A'))
	deliver(m, cmd)
}

func TestModel_adoptionPickerListsTheProjectsSessions_issue122(t *testing.T) {
	r := &recordAdopt{Proposed: twoProposals()}
	m := modelWithAdopt(t, r)

	openAdoption(m)

	got := m.View().Content
	for _, want := range []string{"adopt session", "fix the parser", "chase a flake"} {
		if !strings.Contains(got, want) {
			t.Errorf("the picker does not show %q:\n%s", want, got)
		}
	}
}

// Adoption is per project: the sessions offered are the ones in the repository
// the cursor is in, not every session on the machine.
func TestModel_adoptionScansTheSelectedProject_issue122(t *testing.T) {
	r := &recordAdopt{Proposed: twoProposals()}
	m := modelWithAdopt(t, r)

	openAdoption(m)

	if r.ScannedRoot != "/p/omatty" {
		t.Errorf("scanned %q, want the root of the project under the cursor", r.ScannedRoot)
	}
}

// Committing has to do more than write a row: an adopted session that is not
// started is a sidebar entry with a dead pane, and one whose tailer never
// starts shows no status ever (#33, #40).
func TestModel_adoptionRegistersStartsAndTailsThePick_issue122(t *testing.T) {
	r := &recordAdopt{Proposed: twoProposals()}
	m := modelWithAdopt(t, r)
	openAdoption(m)

	deliver(m, press2(m, special(tea.KeyEnter)))

	if len(r.Adopted) != 1 || r.Adopted[0] != "a1" {
		t.Fatalf("adopted = %v, want the row under the cursor", r.Adopted)
	}
	if len(r.Started) != 1 || r.Started[0] != "a1" {
		t.Errorf("started = %v, want the adopted session's terminal", r.Started)
	}
	if len(r.Tailed) != 1 || r.Tailed[0] != "a1" {
		t.Errorf("tailed = %v, want the adopted session's status tailer", r.Tailed)
	}
}

// tab marks, as it does in the project picker, so several sessions can be
// adopted in one pass.
func TestModel_adoptionTakesEveryMarkedSession_issue122(t *testing.T) {
	r := &recordAdopt{Proposed: twoProposals()}
	m := modelWithAdopt(t, r)
	openAdoption(m)

	press(m, special(tea.KeyTab))                          // mark a1
	press(m, tea.KeyPressMsg{Code: 'j', Mod: tea.ModCtrl}) // move to a2
	press(m, special(tea.KeyTab))                          // mark a2
	deliver(m, press2(m, special(tea.KeyEnter)))

	if len(r.Adopted) != 2 {
		t.Errorf("adopted = %v, want both marked sessions", r.Adopted)
	}
}

// The adopted session must appear in the sidebar, or the operator has no way to
// reach the pane that was just started.
func TestModel_anAdoptedSessionJoinsTheSidebar_issue122(t *testing.T) {
	r := &recordAdopt{Proposed: twoProposals()}
	m := modelWithAdopt(t, r)
	openAdoption(m)

	deliver(m, press2(m, special(tea.KeyEnter)))

	if !strings.Contains(m.View().Content, "fix the parser") {
		t.Errorf("the adopted session is not in the sidebar:\n%s", m.View().Content)
	}
}

// A failed adoption must say so rather than closing on a row that was never
// registered - the picker's own version of discovery's collision report (#91).
func TestModel_adoptionReportsAFailure_issue122(t *testing.T) {
	r := &recordAdopt{Proposed: twoProposals(), AdoptErr: errors.New("session \"a1\" is already registered")}
	m := modelWithAdopt(t, r)
	openAdoption(m)

	deliver(m, press2(m, special(tea.KeyEnter)))

	if !strings.Contains(m.View().Content, "already registered") {
		t.Errorf("the failure is not on screen:\n%s", m.View().Content)
	}
}

// A scan that fails must name itself rather than leaving an empty picker, which
// would read as "you have no sessions here".
func TestModel_adoptionSurfacesAScanFailure_issue122(t *testing.T) {
	r := &recordAdopt{ProposeErr: errors.New("cannot read the transcript store")}
	m := modelWithAdopt(t, r)

	openAdoption(m)

	if !strings.Contains(m.View().Content, "transcript store") {
		t.Errorf("the scan failure is not on screen:\n%s", m.View().Content)
	}
}

// Regression, issue #103: a key that exists in the router and in no keymap the
// operator can see is unreachable in practice. The help modal is the one place
// the full keymap is written down, so a new binding earns a row there.
func TestLeaderKeys_documentAdoption_issue122(t *testing.T) {
	for _, k := range ui.LeaderKeys() {
		if k == "A" {
			return
		}
	}
	t.Errorf("leader keys %v do not document A; ctrl+o A would be undiscoverable", ui.LeaderKeys())
}

// press2 returns the command a keypress produced, for a flow whose work
// continues in one.
func press2(m *ui.Model, k tea.KeyPressMsg) tea.Cmd {
	_, cmd := m.Update(k)
	return cmd
}

// The session is registered under the project the cursor is in. Getting this
// wrong would file a session under another repository, whose diff and file tree
// know nothing about its directory (#122).
func TestModel_adoptionRegistersUnderTheSelectedProject_issue122(t *testing.T) {
	r := &recordAdopt{Proposed: twoProposals()}
	m := modelWithAdopt(t, r)
	openAdoption(m)

	deliver(m, press2(m, special(tea.KeyEnter)))

	if r.Project != "omatty" {
		t.Errorf("registered under %q, want the project under the cursor", r.Project)
	}
}
