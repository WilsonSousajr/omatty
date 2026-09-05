package ui_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// recordDiscover is a named fake for the two discovery dependencies.
type recordDiscover struct {
	Proposed    []ui.Proposal
	ProposeErr  error
	Registered  []string
	RegisterErr error
	// Names is the project name the registry writes for a root, where it
	// differs from the one discovery proposed (#91).
	Names map[string]string
}

func (r *recordDiscover) propose() ([]ui.Proposal, error) {
	return r.Proposed, r.ProposeErr
}

func (r *recordDiscover) register(roots []string) []registry.Registration {
	out := make([]registry.Registration, 0, len(roots))
	for _, root := range roots {
		r.Registered = append(r.Registered, root)
		name := filepath.Base(root)
		if n, ok := r.Names[root]; ok {
			name = n
		}
		out = append(out, registry.Registration{
			Root:    root,
			Project: registry.Project{Name: name, Root: root},
			Err:     r.RegisterErr,
		})
	}
	return out
}

func threeProposals() []ui.Proposal {
	return []ui.Proposal{
		{Name: "omatty", Root: "/p/omatty"},
		{Name: "api-guiaflix", Root: "/work/api-guiaflix"},
		{Name: "notes", Root: "/p/notes"},
	}
}

func modelWithDiscover(t *testing.T, r *recordDiscover) (*ui.Model, map[string]*termwrap.Fake) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Discover, d.AddProject = r.propose, r.register
	return ui.NewModel(d), fakes
}

// openPicker presses the key and delivers the scan's result, which arrives as
// a command rather than inline.
func openPicker(m *ui.Model) {
	press(m, ctrl('o'))
	_, cmd := m.Update(key('a'))
	deliver(m, cmd)
}

func TestModel_pickerListsTheProposedRepositories_issue91(t *testing.T) {
	r := &recordDiscover{Proposed: threeProposals()}
	m, _ := modelWithDiscover(t, r)

	openPicker(m)

	got := m.View().Content
	if !strings.Contains(got, "register project") {
		t.Fatalf("ctrl+o a did not open the picker:\n%s", got)
	}
	for _, name := range []string{"omatty", "api-guiaflix", "notes"} {
		if !strings.Contains(got, name) {
			t.Errorf("the picker does not list %q:\n%s", name, got)
		}
	}
}

// The scan is a git call per slug directory, so it runs as a command. The
// picker must still appear at once, or the key looks dead.
func TestModel_pickerOpensBeforeTheScanReturns_issue91(t *testing.T) {
	m, _ := modelWithDiscover(t, &recordDiscover{Proposed: threeProposals()})

	press(m, ctrl('o'))
	press(m, key('a')) // the command is deliberately not delivered

	if got := m.View().Content; !strings.Contains(got, "scanning") {
		t.Errorf("the picker does not show that it is scanning:\n%s", got)
	}
}

// Discovery proposes; it never registers on its own (invariant 9).
func TestModel_pickerRegistersNothingUntilYouCommit_issue91(t *testing.T) {
	r := &recordDiscover{Proposed: threeProposals()}
	m, _ := modelWithDiscover(t, r)

	openPicker(m)
	press(m, special(tea.KeyEscape))

	if len(r.Registered) != 0 {
		t.Errorf("registered %v without being asked, want nothing", r.Registered)
	}
}

func TestModel_pickerRegistersTheRowUnderTheCursor_issue91(t *testing.T) {
	r := &recordDiscover{Proposed: threeProposals()}
	m, _ := modelWithDiscover(t, r)
	openPicker(m)

	press(m, special(tea.KeyEnter))

	if len(r.Registered) != 1 || r.Registered[0] != "/p/omatty" {
		t.Errorf("registered %v, want just /p/omatty", r.Registered)
	}
	if got := m.View().Content; !strings.Contains(got, "omatty") {
		t.Errorf("the sidebar does not show the registered project:\n%s", got)
	}
}

// Marking several and committing once is the whole reason the picker is
// multi-select: a long history is registered in one pass.
func TestModel_pickerRegistersEveryMarkedRepository_issue91(t *testing.T) {
	r := &recordDiscover{Proposed: threeProposals()}
	m, _ := modelWithDiscover(t, r)
	openPicker(m)

	press(m, special(tea.KeyTab)) // mark omatty
	press(m, ctrl('j'))
	press(m, special(tea.KeyTab)) // mark api-guiaflix
	press(m, special(tea.KeyEnter))

	if len(r.Registered) != 2 {
		t.Fatalf("registered %v, want two marked repositories", r.Registered)
	}
	if r.Registered[0] != "/p/omatty" || r.Registered[1] != "/work/api-guiaflix" {
		t.Errorf("registered %v, want /p/omatty and /work/api-guiaflix", r.Registered)
	}
}

// AddProject refuses a duplicate name even when the roots differ. One at a
// time that is a rare annoyance; in bulk it must not abort the run (#91).
func TestModel_pickerCarriesOnPastACollision_issue91(t *testing.T) {
	r := &recordDiscover{
		Proposed:    threeProposals(),
		RegisterErr: errors.New(`project "api" is already registered at "/work/api"`),
	}
	m, _ := modelWithDiscover(t, r)
	openPicker(m)

	press(m, special(tea.KeyTab))
	press(m, ctrl('j'))
	press(m, special(tea.KeyTab))
	press(m, ctrl('j'))
	press(m, special(tea.KeyTab))
	press(m, special(tea.KeyEnter))

	if len(r.Registered) != 3 {
		t.Errorf("registered %v, want all three attempted despite the collision", r.Registered)
	}
	if got := m.View().Content; !strings.Contains(got, "already registered") {
		t.Errorf("the collision was not reported:\n%s", got)
	}
}

func TestModel_pickerSurfacesAFailedScan_issue91(t *testing.T) {
	r := &recordDiscover{ProposeErr: errors.New("cannot read the transcript store")}
	m, _ := modelWithDiscover(t, r)

	openPicker(m)

	got := m.View().Content
	if !strings.Contains(got, "cannot read the transcript store") {
		t.Errorf("View() does not surface the failed scan:\n%s", got)
	}
	if strings.Contains(got, "register project") {
		t.Errorf("the picker stayed open after a failed scan:\n%s", got)
	}
}

// An empty store is a real answer, not a failure.
func TestModel_pickerSaysWhenThereIsNothingToRegister_issue91(t *testing.T) {
	m, _ := modelWithDiscover(t, &recordDiscover{})

	openPicker(m)

	if got := m.View().Content; !strings.Contains(got, "no repositories found") {
		t.Errorf("the picker does not say the store was empty:\n%s", got)
	}
}

// A scan finishing after the operator moved on must not reopen the box.
func TestModel_pickerDropsAScanThatArrivesAfterItClosed_issue91(t *testing.T) {
	m, _ := modelWithDiscover(t, &recordDiscover{Proposed: threeProposals()})
	press(m, ctrl('o'))
	press(m, key('a'))
	press(m, special(tea.KeyEscape))

	m.Update(ui.ProjectsProposedMsg{Token: 1, Proposals: threeProposals()})

	if got := m.View().Content; strings.Contains(got, "register project") {
		t.Errorf("a late scan reopened the picker:\n%s", got)
	}
}

// Regression, issue #91: two scans can be in flight - ctrl+o a, esc, ctrl+o a -
// and the result carried no way to tell which picker it answered. Whichever
// landed second overwrote the list, so a slow first scan wiped the rows and any
// marks the operator had already made on the second.
func TestModel_pickerIgnoresAnOlderScansResult_issue91(t *testing.T) {
	m, _ := modelWithDiscover(t, &recordDiscover{Proposed: threeProposals()})
	press(m, ctrl('o'))
	press(m, key('a'))
	press(m, special(tea.KeyEscape))
	openPicker(m) // the second scan fills the list

	m.Update(ui.ProjectsProposedMsg{Token: 1, Proposals: []ui.Proposal{
		{Name: "stale", Root: "/p/stale"},
	}})

	if got := m.View().Content; strings.Contains(got, "stale") {
		t.Errorf("the first scan's result overwrote the second scan's list:\n%s", got)
	}
}

// Regression, issue #91: the placeholder renders a live query line and accepts
// keystrokes into it, so a query typed while the scan ran was thrown away when
// the result replaced the list wholesale.
func TestModel_pickerKeepsAQueryTypedWhileScanning_issue91(t *testing.T) {
	m, _ := modelWithDiscover(t, &recordDiscover{Proposed: threeProposals()})
	press(m, ctrl('o'))
	_, cmd := m.Update(key('a'))
	press(m, key('n'))
	press(m, key('o'))
	deliver(m, cmd)

	got := m.View().Content
	if !strings.Contains(got, "no") {
		t.Errorf("the query typed during the scan was lost:\n%s", got)
	}
	if !strings.Contains(got, "1 of 3") {
		t.Errorf("the query was kept but not applied to the list:\n%s", got)
	}
}

// Regression, issue #91: the sidebar row was built from the picked list item
// rather than from the project the registry actually wrote. Discovery names a
// candidate after MainCheckout's directory and AddProject after RepoRoot's, so
// where those disagree the sidebar showed a name state.json did not have -
// ctrl+o n on that row failed, and a restart silently renamed it.
func TestModel_pickerUsesTheNameTheRegistryWrote_issue91(t *testing.T) {
	r := &recordDiscover{
		Proposed: []ui.Proposal{{Name: "proposed-name", Root: "/p/omatty"}},
		Names:    map[string]string{"/p/omatty": "registry-name"},
	}
	m, _ := modelWithDiscover(t, r)
	openPicker(m)

	press(m, special(tea.KeyEnter))

	got := m.View().Content
	if !strings.Contains(got, "registry-name") {
		t.Errorf("the sidebar does not show the name the registry wrote:\n%s", got)
	}
	if strings.Contains(got, "proposed-name") {
		t.Errorf("the sidebar shows the proposed name, which state.json does not have:\n%s", got)
	}
}

// Issue #28, for the picker.
func TestModel_ctrlCQuitsWhileThePickerIsOpen_issue28(t *testing.T) {
	m, _ := modelWithDiscover(t, &recordDiscover{Proposed: threeProposals()})
	openPicker(m)

	_, cmd := m.Update(ctrl('c'))

	if !isQuit(cmd) {
		t.Error("ctrl+c while the picker is open did not quit")
	}
}
