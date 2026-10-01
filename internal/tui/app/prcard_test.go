package app_test

import (
	"errors"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
	"github.com/WilsonSousajr/omatty/internal/service/review"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// cardMiddle is line two's columns between the rail's indent and the blank -
// where the branch, or now the pull request, and the diffstat live. Sixteen
// until #410 took the activity lane out and gave its seven to this.
func cardMiddle(t *testing.T, m *app.Model, id string) string {
	t.Helper()
	line := m.CardOf(id)[1]
	if w := lipgloss.Width(line); w != app.SidebarWidth-1 {
		t.Errorf("line two is %d cells, want %d", w, app.SidebarWidth-1)
	}
	// Past the rail, the gutter (#498) and the two-space indent.
	start := 1 + app.GutterCols() + 2
	return string([]rune(stripSGR(line))[start : start+app.MetaCols()])
}

// modelWithCard has s2 on a worktree (branch parser-fix) and s1 on the main
// checkout, each with a +12 −3 diffstat, and the given pull requests loaded
// for project omatty.
func modelWithCard(t *testing.T, s1Branch string, prs ...forge.PR) *app.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	m := app.NewModel(baseDeps(worktreeState(), terms))
	m.SetRepoStat("s1", review.Stat{Branch: s1Branch, Added: 12, Removed: 3})
	m.SetRepoStat("s2", review.Stat{Branch: "parser-fix", Added: 12, Removed: 3, Head: "h1"})
	m.Update(app.PRsLoadedMsg{Project: "omatty", PRs: prs})
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
		{"no pull request: the branch, as today", nil, "parser-fix       +12 −3"},
		{"passing", []forge.PR{open(349, "parser-fix", forge.CIPassing)}, "#349 ✓           +12 −3"},
		{"failing", []forge.PR{open(349, "parser-fix", forge.CIFailing)}, "#349 ✗           +12 −3"},
		{"running", []forge.PR{open(349, "parser-fix", forge.CIRunning)}, "#349 ◍           +12 −3"},
		{"conflict", []forge.PR{conflicted}, "#349 ⚠           +12 −3"},
		{"failing outranks conflict", []forge.PR{failingAndConflicted}, "#349 ✗           +12 −3"},
		{"no checks", []forge.PR{open(349, "parser-fix", forge.CINone)}, "#349             +12 −3"},
		{"merged, with room, keeps the diffstat (#410)", []forge.PR{{Number: 349, Branch: "parser-fix", State: forge.Merged, Head: "h1"}}, "#349 merged      +12 −3"},
		{"closed, with room, keeps the diffstat (#410)", []forge.PR{{Number: 349, Branch: "parser-fix", State: forge.Closed, Head: "h1"}}, "#349 closed      +12 −3"},
		{"another branch's PR", []forge.PR{open(7, "other", forge.CIPassing)}, "parser-fix       +12 −3"},
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

	if got := cardMiddle(t, m, "s2"); got != "#349 ✓           +12 −3" {
		t.Errorf("line two middle = %q, want the newest pull request, #349", got)
	}
}

// Review Focus 2: a main checkout sitting on develop is not the promotion PR
// that merged develop into main. Only an open one speaks for it.
func TestCard_aMainCheckoutTakesOpenPullRequestsOnly_issue310(t *testing.T) {
	merged := forge.PR{Number: 343, Branch: "develop", State: forge.Merged}

	if got := cardMiddle(t, modelWithCard(t, "develop", merged), "s1"); got != "develop          +12 −3" {
		t.Errorf("with a merged PR from develop, line two middle = %q, want the branch", got)
	}
	withOpen := modelWithCard(t, "develop", merged, open(350, "develop", forge.CIRunning))
	if got := cardMiddle(t, withOpen, "s1"); got != "#350 ◍           +12 −3" {
		t.Errorf("with an open PR from develop, line two middle = %q, want #350", got)
	}
}

// Review Focus 3: when the last poll failed the card does not know, and says
// so rather than showing the old verdict as current (Orca #18484).
func TestCard_aFailedPollMarksThePullRequestUnknown_issue310(t *testing.T) {
	m := modelWithCard(t, "main", open(349, "parser-fix", forge.CIPassing))

	m.Update(app.PRsLoadedMsg{Project: "omatty", Err: errors.New("HTTP 502")})

	if got := cardMiddle(t, m, "s2"); got != "#349 ?           +12 −3" {
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

	if got := cardMiddle(t, m, "s1"); got != "main             +12 −3" {
		t.Errorf("main checkout: line two middle = %q, want the branch", got)
	}
	if got := cardMiddle(t, m, "s2"); got != "parser-fix       +12 −3" {
		t.Errorf("worktree: line two middle = %q, want the branch", got)
	}
}

// Final review, 2: a merged or closed pull request whose head is not this
// checkout's HEAD was an earlier use of a reused branch name, not this work.
func TestCard_aFinishedPullRequestOnAnotherCommitIsNotThisWork_issue310(t *testing.T) {
	old := forge.PR{Number: 240, Branch: "parser-fix", State: forge.Merged, Head: "old"}

	if got := cardMiddle(t, modelWithCard(t, "main", old), "s2"); got != "parser-fix       +12 −3" {
		t.Errorf("line two middle = %q, want the branch, not the old #240", got)
	}
}

// An open pull request keeps its diffstat (#357). The give-way #310 wrote was
// meant for "merged" and "closed", where the label says more than the line
// counts; on an active card a four-digit number and a large diffstat used to
// hide the stat entirely. Now the label drops the space before its mark, then
// the stat sheds whole parts - never a number cut mid-way, which would read
// as a different count - and the CI verdict is never cut. #410 widened the
// middle to 23 columns, where "#1234 ✓ +312 −1.2k" fits whole, so the table
// reaches each step with five- and six-digit numbers and six-figure counts.
func TestCard_anOpenFourDigitPullRequestKeepsItsDiffstat_issue357(t *testing.T) {
	passing := []forge.PR{open(1234, "parser-fix", forge.CIPassing)}
	passing5 := []forge.PR{open(12345, "parser-fix", forge.CIPassing)}
	passing6 := []forge.PR{open(123456, "parser-fix", forge.CIPassing)}
	merged := []forge.PR{{Number: 1234, Branch: "parser-fix", State: forge.Merged, Head: "h1"}}
	for _, tt := range []struct {
		name           string
		prs            []forge.PR
		added, removed int
		want           string
	}{
		{"both fit as they are", passing, 12, 3, "#1234 ✓          +12 −3"},
		{"exactly twenty-three", passing, 123_456, 123_456, "#1234 ✓ +123.5k −123.5k"},
		{"the gap goes", passing5, 123_456, 123_456, "#12345✓ +123.5k −123.5k"},
		{"then the removed half", passing6, 123_456, 123_456, "#123456✓        +123.5k"},
		{"merged still gives the diffstat way", merged, 1200, 1200, "#1234 merged           "},
	} {
		m := modelWithCard(t, "main", tt.prs...)
		m.SetRepoStat("s2", review.Stat{Branch: "parser-fix", Added: tt.added, Removed: tt.removed, Head: "h1"})
		if got := cardMiddle(t, m, "s2"); got != tt.want {
			t.Errorf("%s: line two middle = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// Bitbucket names a pull request's head by its first twelve characters, and
// git names the session's by all forty: the same commit, so a merged one
// still speaks for its session. Shorter than twelve is too short to be sure
// of, and a different commit never matches (#460's review).
func TestCard_AShortHeadIsTheSameCommit_issue460(t *testing.T) {
	full := "31b8ff8dad0a4c6e8f1a2b3c4d5e6f708192a3b4"
	for head, want := range map[string]string{
		full[:12]:      "#349 merged      +12 −3",
		full[:7]:       "parser-fix       +12 −3",
		"31b8ff8dad0b": "parser-fix       +12 −3",
	} {
		terms, _ := fakeTerms(t)
		m := app.NewModel(baseDeps(worktreeState(), terms))
		m.SetRepoStat("s2", review.Stat{Branch: "parser-fix", Added: 12, Removed: 3, Head: full})
		m.Update(app.PRsLoadedMsg{Project: "omatty", PRs: []forge.PR{{Number: 349, Branch: "parser-fix", State: forge.Merged, Head: head}}})

		if got := cardMiddle(t, m, "s2"); got != want {
			t.Errorf("head %q: line two middle = %q, want %q", head, got, want)
		}
	}
}
