package ui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// shipper is a named ShipFuncs fake recording every outside effect it was asked
// to make - which is the point of the tests below: most of them assert that it
// was asked for none.
type shipper struct {
	State     review.Shippable
	StateErr  error
	Protected bool
	ProtErr   error
	CreateErr error
	MergeErr  error

	Pushed  []string
	Created []string
	Merged  []int
}

func (s *shipper) shippable(_ registry.Session, _ string) (review.Shippable, error) {
	return s.State, s.StateErr
}

func (s *shipper) push(_, branch string) error {
	s.Pushed = append(s.Pushed, branch)
	return nil
}

func (s *shipper) create(_, head, base, _ string) (int, error) {
	s.Created = append(s.Created, head+"->"+base)
	return 443, s.CreateErr
}

func (s *shipper) merge(_ string, number int) error {
	s.Merged = append(s.Merged, number)
	return s.MergeErr
}

func (s *shipper) protected(_, _ string) (bool, error) { return s.Protected, s.ProtErr }

func (s *shipper) funcs() ui.ShipFuncs {
	return ui.ShipFuncs{
		Shippable: s.shippable, Push: s.push, CreatePR: s.create,
		MergePR: s.merge, BranchProtected: s.protected,
	}
}

// shipState is one worktree session on a branch forked from develop.
func shipState() registry.State {
	return registry.State{
		Projects: []registry.Project{{Name: "omatty", Root: "/p/omatty",
			Gate: []gate.Step{{Name: "test", Run: "go test ./..."}}}},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: "parser-fix",
			Dir: "/wt/parser-fix", Branch: "feat/parser", Base: "develop", Worktree: true}},
	}
}

// modelReadyToShip is a model whose session has a green gate, a non-empty diff
// and is at rest - the three facts readyToShip is made of.
func modelReadyToShip(t *testing.T, sh *shipper, prs []forge.PR) *ui.Model {
	t.Helper()
	st := shipState()
	d := baseDeps(st, fakeTermsFor(st))
	d.Ship = sh.funcs()
	d.PRs = func(string) ([]forge.PR, error) { return prs, nil }
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.SetRepoStat("s1", review.Stat{Branch: "feat/parser", Added: 12, Removed: 3, Head: "abc123"})
	m.Update(ui.PRsLoadedMsg{Project: "omatty", PRs: prs})
	statusDeliver(m, "s1", watcher.TurnEnded, time.Now())
	deliver(m, second(m.Update(passingReport())))
	return m
}

// #331 step 1, the whole point: one keypress pushes and opens the pull request.
func TestModel_pPushesAndOpensThePullRequest_issue331(t *testing.T) {
	sh := &shipper{State: review.Shippable{Commits: 2}}
	m := modelReadyToShip(t, sh, nil)

	leaderDeliver(m, key('p'))

	if len(sh.Pushed) != 1 || sh.Pushed[0] != "feat/parser" {
		t.Fatalf("pushed %v, want the session's branch once", sh.Pushed)
	}
	if len(sh.Created) != 1 || sh.Created[0] != "feat/parser->develop" {
		t.Errorf("opened %v, want feat/parser against its own base", sh.Created)
	}
	if got := m.View().Content; !strings.Contains(got, "443") {
		t.Errorf("the footer does not name the pull request:\n%s", got)
	}
}

// A dirty worktree is refused. The gate verified a working tree and a push moves
// commits, so shipping uncommitted work would open a pull request that differs
// from what was verified - and omatty authors no commit to close the gap.
func TestModel_pRefusesAnUncommittedWorktree_issue331(t *testing.T) {
	sh := &shipper{State: review.Shippable{Uncommitted: 4, Commits: 2}}
	m := modelReadyToShip(t, sh, nil)

	leaderDeliver(m, key('p'))

	if len(sh.Pushed) != 0 {
		t.Fatalf("pushed %v with uncommitted work", sh.Pushed)
	}
	if got := m.View().Content; !strings.Contains(got, "commit them in the session") {
		t.Errorf("the refusal does not say what to do:\n%s", got)
	}
}

// Nothing to ship is not a push. A branch with no commit its base lacks would
// open an empty pull request.
func TestModel_pRefusesABranchWithNoCommits_issue331(t *testing.T) {
	sh := &shipper{State: review.Shippable{}}
	m := modelReadyToShip(t, sh, nil)

	leaderDeliver(m, key('p'))

	if len(sh.Pushed) != 0 || len(sh.Created) != 0 {
		t.Errorf("pushed %v / opened %v for a branch with nothing on it", sh.Pushed, sh.Created)
	}
}

// A red or unrun gate is refused before anything is read, and the refusal names
// the key that would fix it.
func TestModel_pRefusesWhenTheGateIsNotGreen_issue331(t *testing.T) {
	sh := &shipper{State: review.Shippable{Commits: 2}}
	st := shipState()
	d := baseDeps(st, fakeTermsFor(st))
	d.Ship = sh.funcs()
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	leaderDeliver(m, key('p'))

	if len(sh.Pushed) != 0 {
		t.Fatalf("pushed %v with no gate verdict", sh.Pushed)
	}
	if got := m.View().Content; !strings.Contains(got, "gate is not green") {
		t.Errorf("the refusal does not name the gate:\n%s", got)
	}
}

// Step 2: a pull request already open and green on both sides merges.
func TestModel_pMergesAPullRequestThatIsAlreadyGreen_issue331(t *testing.T) {
	sh := &shipper{State: review.Shippable{Commits: 2}}
	m := modelReadyToShip(t, sh, []forge.PR{
		{Number: 443, Branch: "feat/parser", State: forge.Open, CI: forge.CIPassing, Head: "abc123"},
	})

	leaderDeliver(m, key('p'))

	if len(sh.Merged) != 1 || sh.Merged[0] != 443 {
		t.Fatalf("merged %v, want #443 once", sh.Merged)
	}
	if len(sh.Created) != 0 {
		t.Errorf("opened %v as well as merging", sh.Created)
	}
	if got := m.View().Content; !strings.Contains(got, "merged #443") {
		t.Errorf("the footer does not report the merge:\n%s", got)
	}
}

// The line #331 must not cross: nothing merges on a red or pending remote
// verdict. That is auto-merge's job and it belongs on the forge.
func TestModel_pRefusesToMergeWhileTheChecksAreNotPassing_issue331(t *testing.T) {
	for _, ci := range []forge.CIState{forge.CIRunning, forge.CIFailing, forge.CINone} {
		sh := &shipper{State: review.Shippable{Commits: 2}}
		m := modelReadyToShip(t, sh, []forge.PR{
			{Number: 443, Branch: "feat/parser", State: forge.Open, CI: ci, Head: "abc123"},
		})

		leaderDeliver(m, key('p'))

		if len(sh.Merged) != 0 {
			t.Errorf("CI %v: merged %v", ci, sh.Merged)
		}
		if got := m.View().Content; !strings.Contains(got, "not passing") {
			t.Errorf("CI %v: the refusal does not name the checks:\n%s", ci, got)
		}
	}
}

// A draft is work that is not offered yet, and a conflicted one cannot merge as
// it stands.
func TestModel_pRefusesADraftAndAConflict_issue331(t *testing.T) {
	for reason, pr := range map[string]forge.PR{
		"draft":    {Number: 443, Branch: "feat/parser", State: forge.Open, CI: forge.CIPassing, Draft: true, Head: "abc123"},
		"conflict": {Number: 443, Branch: "feat/parser", State: forge.Open, CI: forge.CIPassing, Conflict: true, Head: "abc123"},
	} {
		sh := &shipper{State: review.Shippable{Commits: 2}}
		m := modelReadyToShip(t, sh, []forge.PR{pr})

		leaderDeliver(m, key('p'))

		if len(sh.Merged) != 0 {
			t.Errorf("%s: merged %v", reason, sh.Merged)
		}
	}
}

// The bound #331's own text does not carry: never into a protected branch.
// AGENTS.md moves `main` only by a promotion pull request, so a ship key able to
// merge there would route around omatty's own release gate.
func TestModel_pRefusesToMergeIntoAProtectedBranch_issue331(t *testing.T) {
	sh := &shipper{State: review.Shippable{Commits: 2}, Protected: true}
	m := modelReadyToShip(t, sh, []forge.PR{
		{Number: 443, Branch: "feat/parser", State: forge.Open, CI: forge.CIPassing, Head: "abc123"},
	})

	leaderDeliver(m, key('p'))

	if len(sh.Merged) != 0 {
		t.Fatalf("merged into a protected branch: %v", sh.Merged)
	}
	if got := m.View().Content; !strings.Contains(got, "protected") {
		t.Errorf("the refusal does not say why:\n%s", got)
	}
}

// It fails closed: a protection flag omatty could not read is not permission.
func TestModel_pRefusesWhenProtectionCannotBeRead_issue331(t *testing.T) {
	sh := &shipper{State: review.Shippable{Commits: 2}, ProtErr: errors.New("HTTP 404")}
	m := modelReadyToShip(t, sh, []forge.PR{
		{Number: 443, Branch: "feat/parser", State: forge.Open, CI: forge.CIPassing, Head: "abc123"},
	})

	leaderDeliver(m, key('p'))

	if len(sh.Merged) != 0 {
		t.Fatalf("merged without knowing whether the base was protected: %v", sh.Merged)
	}
	if got := m.View().Content; !strings.Contains(got, "refusing to merge") {
		t.Errorf("the refusal does not say it could not tell:\n%s", got)
	}
}

// A main-checkout session has no branch somebody opened for this work.
func TestModel_pRefusesAMainCheckoutSession_issue331(t *testing.T) {
	sh := &shipper{State: review.Shippable{Commits: 2}}
	st := shipState()
	st.Sessions[0].Branch, st.Sessions[0].Base, st.Sessions[0].Worktree = "", "", false
	d := baseDeps(st, fakeTermsFor(st))
	d.Ship = sh.funcs()
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.SetRepoStat("s1", review.Stat{Branch: "develop", Added: 3, Head: "abc123"})
	statusDeliver(m, "s1", watcher.TurnEnded, time.Now())
	deliver(m, second(m.Update(passingReport())))

	leaderDeliver(m, key('p'))

	if len(sh.Pushed) != 0 {
		t.Errorf("pushed %v from a main-checkout session", sh.Pushed)
	}
}

// Unwired, it refuses by name rather than doing nothing.
func TestModel_pWithNoForgeWiredSaysSo_issue331(t *testing.T) {
	st := shipState()
	d := baseDeps(st, fakeTermsFor(st))
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.SetRepoStat("s1", review.Stat{Branch: "feat/parser", Added: 12, Head: "abc123"})
	statusDeliver(m, "s1", watcher.TurnEnded, time.Now())
	deliver(m, second(m.Update(passingReport())))

	leaderDeliver(m, key('p'))

	if got := m.View().Content; !strings.Contains(got, "no forge wired") {
		t.Errorf("an unwired ship said nothing:\n%s", got)
	}
}

// leaderDeliver is leader() for a command that has to run: shipping is an
// outside effect, and a test that only Updates asserts against a model half-way
// through the keypress.
func leaderDeliver(m *ui.Model, k tea.KeyPressMsg) {
	press(m, ctrl('o'))
	_, cmd := m.Update(k)
	deliver(m, cmd)
}

// Regression, found by #331's own real-PTY run: watcher.Status is a string, so a
// session nothing has reported on holds "" rather than StatusIdle, and
// atRest("") is false. readyToShip therefore never said READY for a stopped
// session with a green gate, and `p` refused it with "the gate is not green" -
// which is every session at boot and every stopped one. No unit test caught it
// because every fixture reports a status first.
func TestModel_pShipsASessionThatHasReportedNoStatus_issue331(t *testing.T) {
	sh := &shipper{State: review.Shippable{Commits: 2}}
	st := shipState()
	d := baseDeps(st, fakeTermsFor(st))
	d.Ship = sh.funcs()
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.SetRepoStat("s1", review.Stat{Branch: "feat/parser", Added: 12, Removed: 3, Head: "abc123"})
	// A green gate, and deliberately no status at all: a stopped session.
	deliver(m, second(m.Update(passingReport())))

	leaderDeliver(m, key('p'))

	if len(sh.Pushed) != 1 {
		t.Errorf("pushed %v, want the stopped session shipped; footer says %q",
			sh.Pushed, footerOf(m))
	}
}
