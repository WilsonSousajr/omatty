package ui_test

import (
	"errors"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// cardMiddle is line two's sixteen columns between the rail's indent and the
// lane - where the branch, or now the pull request, and the diffstat live.
func cardMiddle(t *testing.T, m *ui.Model, id string) string {
	t.Helper()
	line := m.CardOf(id)[1]
	if w := lipgloss.Width(line); w != ui.SidebarWidth-1 {
		t.Errorf("line two is %d cells, want %d", w, ui.SidebarWidth-1)
	}
	return string([]rune(stripSGR(line))[3 : 3+16])
}

// modelWithCard has s2 on a worktree (branch parser-fix) and s1 on the main
// checkout, each with a +12 −3 diffstat, and the given pull requests loaded
// for project omatty.
func modelWithCard(t *testing.T, s1Branch string, prs ...forge.PR) *ui.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	m := ui.NewModel(baseDeps(worktreeState(), terms))
	m.SetRepoStat("s1", review.Stat{Branch: s1Branch, Added: 12, Removed: 3})
	m.SetRepoStat("s2", review.Stat{Branch: "parser-fix", Added: 12, Removed: 3, Head: "h1"})
	m.Update(ui.PRsLoadedMsg{Project: "omatty", PRs: prs})
	return m
}

func open(n int, branch string, ci forge.CIState) forge.PR {
	return forge.PR{Number: n, Branch: branch, State: forge.Open, CI: ci}
}

func TestCard_lineTwoNamesThePullRequestAndItsCI_issue310(t *testing.T) {
	conflicted := open(349, "parser-fix", forge.CIPassing)
	conflicted.Conflict = true
	failingAndConflicted := open(349, "parser-fix", forge.CIFailing)
	failingAndConflicted.Conflict = true
	for _, tt := range []struct {
		name string
		prs  []forge.PR
		want string
	}{
		{"no pull request: the branch, as today", nil, "parser-fi +12 −3"},
		{"passing", []forge.PR{open(349, "parser-fix", forge.CIPassing)}, "#349 ✓    +12 −3"},
		{"failing", []forge.PR{open(349, "parser-fix", forge.CIFailing)}, "#349 ✗    +12 −3"},
		{"running", []forge.PR{open(349, "parser-fix", forge.CIRunning)}, "#349 ◍    +12 −3"},
		{"conflict", []forge.PR{conflicted}, "#349 ⚠    +12 −3"},
		{"failing outranks conflict", []forge.PR{failingAndConflicted}, "#349 ✗    +12 −3"},
		{"no checks", []forge.PR{open(349, "parser-fix", forge.CINone)}, "#349      +12 −3"},
		{"merged gives the diffstat way", []forge.PR{{Number: 349, Branch: "parser-fix", State: forge.Merged, Head: "h1"}}, "#349 merged     "},
		{"closed", []forge.PR{{Number: 349, Branch: "parser-fix", State: forge.Closed, Head: "h1"}}, "#349 closed     "},
		{"another branch's PR", []forge.PR{open(7, "other", forge.CIPassing)}, "parser-fi +12 −3"},
	} {
		m := modelWithCard(t, "main", tt.prs...)
		if got := cardMiddle(t, m, "s2"); got != tt.want {
			t.Errorf("%s: line two middle = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// Review Focus 1: a branch reused across pull requests shows the newest.
func TestCard_theNewestPullRequestForABranchWins_issue310(t *testing.T) {
	m := modelWithCard(t, "main",
		forge.PR{Number: 12, Branch: "parser-fix", State: forge.Closed, Head: "h1"},
		open(349, "parser-fix", forge.CIPassing))

	if got := cardMiddle(t, m, "s2"); got != "#349 ✓    +12 −3" {
		t.Errorf("line two middle = %q, want the newest pull request, #349", got)
	}
}

// Review Focus 2: a main checkout sitting on develop is not the promotion PR
// that merged develop into main. Only an open one speaks for it.
func TestCard_aMainCheckoutTakesOpenPullRequestsOnly_issue310(t *testing.T) {
	merged := forge.PR{Number: 343, Branch: "develop", State: forge.Merged}

	if got := cardMiddle(t, modelWithCard(t, "develop", merged), "s1"); got != "develop   +12 −3" {
		t.Errorf("with a merged PR from develop, line two middle = %q, want the branch", got)
	}
	withOpen := modelWithCard(t, "develop", merged, open(350, "develop", forge.CIRunning))
	if got := cardMiddle(t, withOpen, "s1"); got != "#350 ◍    +12 −3" {
		t.Errorf("with an open PR from develop, line two middle = %q, want #350", got)
	}
}

// Review Focus 3: when the last poll failed the card does not know, and says
// so rather than showing the old verdict as current (Orca #18484).
func TestCard_aFailedPollMarksThePullRequestUnknown_issue310(t *testing.T) {
	m := modelWithCard(t, "main", open(349, "parser-fix", forge.CIPassing))

	m.Update(ui.PRsLoadedMsg{Project: "omatty", Err: errors.New("HTTP 502")})

	if got := cardMiddle(t, m, "s2"); got != "#349 ?    +12 −3" {
		t.Errorf("line two middle = %q, want #349 ?", got)
	}
}

// Final review, 1: a fork's pull request is someone else's branch that shares
// a name - a contributor's "main", or a common slug - never this session's.
func TestCard_aForkPullRequestIsNeverThisSessions_issue310(t *testing.T) {
	fork := func(branch string) forge.PR {
		pr := open(812, branch, forge.CIFailing)
		pr.Fork = true
		return pr
	}
	m := modelWithCard(t, "main", fork("main"), fork("parser-fix"))

	if got := cardMiddle(t, m, "s1"); got != "main      +12 −3" {
		t.Errorf("main checkout: line two middle = %q, want the branch", got)
	}
	if got := cardMiddle(t, m, "s2"); got != "parser-fi +12 −3" {
		t.Errorf("worktree: line two middle = %q, want the branch", got)
	}
}

// Final review, 2: a merged or closed pull request whose head is not this
// checkout's HEAD was an earlier use of a reused branch name, not this work.
func TestCard_aFinishedPullRequestOnAnotherCommitIsNotThisWork_issue310(t *testing.T) {
	old := forge.PR{Number: 240, Branch: "parser-fix", State: forge.Merged, Head: "old"}

	if got := cardMiddle(t, modelWithCard(t, "main", old), "s2"); got != "parser-fi +12 −3" {
		t.Errorf("line two middle = %q, want the branch, not the old #240", got)
	}
}
