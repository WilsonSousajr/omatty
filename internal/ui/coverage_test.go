package ui_test

import (
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// coveredRepo is a checkout holding a module and the profile a gate would have
// just written into it, so the load runs against a real file rather than a
// fake reader - the wiring is the thing under test (#254).
func coveredRepo(t *testing.T, profile string) string {
	t.Helper()
	root := t.TempDir()
	write(t, filepath.Join(root, "go.mod"), "module example.com/m\n\ngo 1.26\n")
	if profile != "" {
		write(t, filepath.Join(root, profile), "mode: set\n"+
			"example.com/m/a.go:3.10,5.4 2 1\n"+
			"example.com/m/a.go:9.10,10.4 1 0\n")
	}
	return root
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("setup %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("setup %s: %v", path, err)
	}
}

// modelWithProfile is a one-session model whose project's gate declares the
// given profile, with the session's directory the checkout that holds it.
func modelWithProfile(t *testing.T, dir, declared string) *ui.Model {
	t.Helper()
	steps := []gate.Step{
		{Name: "test", Run: "go test ./..."},
		{Name: "cov", Run: "./cov.sh", Kind: gate.KindCoverage, Profile: declared},
	}
	st := registry.State{
		Projects: []registry.Project{{Name: "omatty", Root: dir, Gate: steps}},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: "one", Dir: dir, Branch: "main"}},
	}
	m := ui.NewModel(baseDeps(st, fakeTermsFor(st)))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

func passingReport() ui.GateMsg {
	return ui.GateMsg(gate.Report{ID: "s1", Results: []gate.StepResult{
		{Step: gate.Step{Name: "test"}, Verdict: gate.Pass},
		{Step: gate.Step{Name: "cov", Kind: gate.KindCoverage}, Verdict: gate.Pass},
	}})
}

// The moment a profile becomes true is the moment the gate that wrote it
// finishes, so that is when it is read - out of the session's own directory,
// which for a worktree session is that worktree, so two sessions never read
// each other's numbers.
func TestModel_aFinishedGateLoadsTheProfileItsProjectDeclares_issue254(t *testing.T) {
	dir := coveredRepo(t, "cover.out")
	m := modelWithProfile(t, dir, "cover.out")

	settle(m, second(m.Update(passingReport())))

	lines, ok := m.CoverageOf("s1")
	if !ok {
		t.Fatal("no overlay was loaded for the session")
	}
	if covered, known := lines["a.go"][3]; !known || !covered {
		t.Errorf("a.go:3 = (%v, %v), want a covered line", covered, known)
	}
	if covered, known := lines["a.go"][9]; !known || covered {
		t.Errorf("a.go:9 = (%v, %v), want an uncovered line", covered, known)
	}
}

// A project that declares no profile reads nothing at all. Most gates have no
// coverage step, and the overlay is the only thing that wants one.
func TestModel_aProjectWithNoProfile_readsNothing_issue254(t *testing.T) {
	m := modelWithProfile(t, coveredRepo(t, "cover.out"), "")

	settle(m, second(m.Update(passingReport())))

	if _, ok := m.CoverageOf("s1"); ok {
		t.Error("an overlay was loaded for a project that declares no profile")
	}
}

// A profile that will not parse leaves the previous overlay rather than
// blanking it: the markers go stale, which is the same freshness the diff
// already has, while blanking would silently claim everything is covered.
func TestModel_aMalformedProfileKeepsThePreviousOverlay_issue254(t *testing.T) {
	dir := coveredRepo(t, "cover.out")
	m := modelWithProfile(t, dir, "cover.out")
	settle(m, second(m.Update(passingReport())))

	write(t, filepath.Join(dir, "cover.out"), "this is not a coverage profile\n")
	settle(m, second(m.Update(passingReport())))

	lines, ok := m.CoverageOf("s1")
	if !ok {
		t.Fatal("the overlay was dropped rather than kept")
	}
	if _, known := lines["a.go"][9]; !known {
		t.Error("the overlay was blanked by a profile that would not parse")
	}
}

// A profile the gate never wrote is the same case: nothing to read, nothing
// claimed. A gate that fails before its coverage step leaves exactly this.
func TestModel_aMissingProfileLoadsNothing_issue254(t *testing.T) {
	m := modelWithProfile(t, coveredRepo(t, ""), "cover.out")

	settle(m, second(m.Update(passingReport())))

	if _, ok := m.CoverageOf("s1"); ok {
		t.Error("an overlay appeared for a profile that does not exist")
	}
}

// A report for a session omatty does not hold must not send it reading files
// in a directory it does not own, which is the rule onGate already follows for
// the report itself (#69).
func TestModel_aReportForAnUnknownSessionLoadsNoProfile_issue254(t *testing.T) {
	m := modelWithProfile(t, coveredRepo(t, "cover.out"), "cover.out")

	settle(m, second(m.Update(ui.GateMsg(gate.Report{ID: "not-a-session"}))))

	if _, ok := m.CoverageOf("s1"); ok {
		t.Error("a foreign report loaded an overlay")
	}
}

// second is the command half of an Update, so a test can settle it without
// naming the model it already holds.
func second(_ tea.Model, cmd tea.Cmd) tea.Cmd { return cmd }
