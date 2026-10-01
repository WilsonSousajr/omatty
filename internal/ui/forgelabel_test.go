package ui_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/infra/forge"
	"github.com/WilsonSousajr/omatty/internal/service/review"
	"github.com/WilsonSousajr/omatty/internal/service/status"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// gitLab is a forge whose words are not GitHub's, so a copy site still
// hard-coding "pull request" or "#" shows up against it (#449).
var gitLab = forge.Label{Forge: "GitLab", Change: "merge request", Short: "MR", Sigil: "!"}

// labelOf answers every project with one label, the way a Router answers a
// project it has already resolved.
func labelOf(l forge.Label) ui.LabelFunc { return func(string) forge.Label { return l } }

// gitLabTracker is workModel's tracker - one issue, one open change, a session
// to type into - on a project whose forge is GitLab.
func gitLabTracker(t *testing.T, itemErr error) (*ui.Model, map[string]*termwrap.Fake) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	fi := &FakeIssues{Lists: map[string][]forge.Issue{}, Errs: map[string]error{}, Now: fixedNow}
	fp := &FakePRs{Lists: map[string][]forge.PR{}, Errs: map[string]error{}, Now: fixedNow}
	fi.Lists["/p/omatty"] = []forge.Issue{{Number: 399, Title: "filter the tracker", Updated: fixedNow}}
	fp.Lists["/p/omatty"] = []forge.PR{{Number: 400, Title: "first slice", State: forge.Open, Updated: fixedNow}}
	items := &FakeItems{Issues: map[int]forge.Detail{}, PRs: map[int]forge.Detail{
		400: {Number: 400, Title: "first slice", Author: "someone"},
	}, Err: itemErr}
	d := baseDeps(withEmptyProject(), terms)
	d.Issues, d.PRs, d.Label = fi.List, fp.List, labelOf(gitLab)
	d.Item = ui.ForgeItemFuncs{Issue: items.issue, PR: items.pr}
	d.Clock = func() time.Time { return fi.Now }
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 122, Height: 32})
	openTracker(m)
	return m, fakes
}

// onTheChange moves the cursor over the rule onto the open change.
func onTheChange(m *ui.Model) {
	pressDeliver(m, key('j'))
	pressDeliver(m, key('j'))
}

// The list, its rule, its count and the header speak the forge's own nouns:
// GitLab's "!400" and "merge requests", while the issue stays "#399".
func TestTracker_SpeaksTheForgesOwnNouns_issue449(t *testing.T) {
	m, _ := gitLabTracker(t, nil)

	body := trackerBody(t, m)
	for _, want := range []string{"!400", "merge requests", "1 mr", "#399"} {
		if !strings.Contains(body, want) {
			t.Errorf("the tracker does not say %q:\n%s", want, body)
		}
	}
	for _, not := range []string{"#400", "pull requests"} {
		if strings.Contains(body, not) {
			t.Errorf("the tracker says %q on GitLab:\n%s", not, body)
		}
	}
	if header := stripSGR(omattyHeader(t, m)); !strings.Contains(header, "1m") {
		t.Errorf("header = %q, want the change count as 1m", header)
	}
}

// An item's heading writes the number the way its forge does.
func TestTrackerItem_AChangeIsHeadedWithItsSigil_issue449(t *testing.T) {
	m, _ := gitLabTracker(t, nil)
	onTheChange(m)

	pressDeliver(m, special(tea.KeyEnter))

	if body := trackerBody(t, m); !strings.Contains(body, "!400  first slice") {
		t.Errorf("the item is not headed !400:\n%s", body)
	}
}

// A read that finds the tool missing says which tool, and writes the number
// the forge's way.
func TestTrackerItem_AMissingToolSaysWhichTool_issue449(t *testing.T) {
	m, _ := gitLabTracker(t, &forge.MissingToolError{Tool: "glab", TokenEnv: "GITLAB_TOKEN"})
	onTheChange(m)

	pressDeliver(m, special(tea.KeyEnter))

	want := "glab is not installed and GITLAB_TOKEN is unset, so !400 cannot be read."
	if got := columnText(m); !strings.Contains(got, want) {
		t.Errorf("the item view reads %q, want %q", got, want)
	}
}

// Regression: the preview headed the row under the cursor with the kind of the
// last item opened in full, not its own - so on GitLab a merge request previewed
// before anything was opened read "#400". Latent while every forge wrote "#".
func TestTracker_ThePreviewHeadsAChangeWithItsOwnSigil_issue574(t *testing.T) {
	m, _ := gitLabTracker(t, nil)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 32})
	leader(m, key('z'))

	onTheChange(m)

	view := stripSGR(m.View().Content)
	if !strings.Contains(view, "!400  first slice") || strings.Contains(view, "#400  first slice") {
		t.Errorf("the preview does not head the merge request !400:\n%s", view)
	}
}

// a types the change into the composer in the forge's own words: claude reads
// "merge request !400" correctly where "pull request #400" names nothing.
func TestTrackerWork_AAttachesAChangeInItsForgesWords_issue449(t *testing.T) {
	m, fakes := gitLabTracker(t, nil)
	onTheChange(m)

	pressDeliver(m, key('a'))

	if sent := strings.Join(fakes["s1"].Sent, ""); !strings.Contains(sent, "merge request !400") {
		t.Errorf("the session received %q, want merge request !400", sent)
	}
}

// n on a change refuses in the forge's words too.
func TestTrackerWork_NOnAChangeNamesItWithItsSigil_issue449(t *testing.T) {
	m, _ := gitLabTracker(t, nil)
	onTheChange(m)

	pressDeliver(m, key('n'))

	if body := trackerBody(t, m); !strings.Contains(body, "!400 already has a branch") {
		t.Errorf("the refusal does not name !400:\n%s", body)
	}
}

// The note names the tool and the variable: "install X or set Y" is the whole
// of the fix, and "gh is not installed" was only ever half of it.
func TestTracker_NamesTheToolAndTheVariableItIsMissing_issue449(t *testing.T) {
	m, fi, fp := modelWithBothLists(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	missing := &forge.MissingToolError{Tool: "glab", TokenEnv: "GITLAB_TOKEN"}
	for _, root := range []string{"/p/omatty", "/p/api-svc", "/p/empty"} {
		fi.Errs[root], fp.Errs[root] = missing, missing
	}

	openTracker(m)

	if got := columnText(m); !strings.Contains(got, "glab is not installed and GITLAB_TOKEN is unset") {
		t.Errorf("the tracker does not name glab and GITLAB_TOKEN: %q", got)
	}
}

// A project off its forge names the forge it is not on, and one whose forge is
// not known says so without naming one.
func TestTracker_AProjectOffItsForgeSaysWhichForge_issue449(t *testing.T) {
	for _, tt := range []struct {
		label forge.Label
		want  string
	}{
		{gitLab, "not on GitLab"},
		{forge.Neutral, "not on a forge omatty reads; name its host in [forge.hosts]."},
	} {
		terms, _ := fakeTerms(t)
		off := fmt.Errorf("forge: no git remotes found: %w", forge.ErrNoForge)
		fi := &FakeIssues{Lists: map[string][]forge.Issue{}, Errs: map[string]error{"/p/omatty": off}, Now: fixedNow}
		fp := &FakePRs{Lists: map[string][]forge.PR{}, Errs: map[string]error{"/p/omatty": off}, Now: fixedNow}
		d := baseDeps(withEmptyProject(), terms)
		d.Issues, d.PRs, d.Label = fi.List, fp.List, labelOf(tt.label)
		m := ui.NewModel(d)
		m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})

		openTracker(m)

		if got := columnText(m); !strings.Contains(got, tt.want) {
			t.Errorf("%s: the tracker does not say %q: %q", tt.label.Forge, tt.want, got)
		}
	}
}

// A missing tool is a fact about one forge on this machine, not about every
// project: a project whose forge answered keeps being asked.
func TestModel_aMissingToolStopsOnlyItsOwnProject_issue449(t *testing.T) {
	m, f := modelWithPRs(t)
	f.Errs["/p/omatty"] = &forge.MissingToolError{Tool: "glab", TokenEnv: "GITLAB_TOKEN"}
	deliver(m, m.PollPRs())
	f.Asked = nil
	f.later()

	deliver(m, m.PollPRs())

	if got := f.asked(); got != "/p/api-svc" {
		t.Errorf("asked %q, want only the project whose forge answered", got)
	}
}

// The card writes its change the forge's way.
func TestCard_WritesTheChangeWithItsForgesSigil_issue449(t *testing.T) {
	terms, _ := fakeTerms(t)
	d := baseDeps(worktreeState(), terms)
	d.Label = labelOf(gitLab)
	m := ui.NewModel(d)
	m.SetRepoStat("s2", review.Stat{Branch: "parser-fix", Added: 12, Removed: 3, Head: "h1"})
	m.Update(ui.PRsLoadedMsg{Project: "omatty", PRs: []forge.PR{open(349, "parser-fix", forge.CIPassing)}})

	if got := cardMiddle(t, m, "s2"); !strings.HasPrefix(got, "!349 ✓") {
		t.Errorf("line two = %q, want it to start !349 ✓", got)
	}
}

// Shipping reports the merge the forge's way.
func TestModel_pReportsTheMergeWithTheForgesSigil_issue449(t *testing.T) {
	sh := &shipper{State: review.Shippable{Commits: 2}}
	prs := []forge.PR{{Number: 443, Branch: "feat/parser", State: forge.Open, CI: forge.CIPassing, Head: "abc123"}}
	st := shipState()
	d := baseDeps(st, fakeTermsFor(st))
	d.Ship, d.Label = sh.funcs(), labelOf(gitLab)
	d.PRs = func(string) ([]forge.PR, error) { return prs, nil }
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.SetRepoStat("s1", review.Stat{Branch: "feat/parser", Added: 12, Removed: 3, Head: "abc123"})
	m.Update(ui.PRsLoadedMsg{Project: "omatty", PRs: prs})
	statusDeliver(m, "s1", status.TurnEnded, time.Now())
	deliver(m, second(m.Update(passingReport())))

	leaderDeliver(m, key('p'))

	if got := m.View().Content; !strings.Contains(got, "merged !443") {
		t.Errorf("the footer does not report the merge as !443:\n%s", got)
	}
}

// noGH is what every forge call answers on a machine without gh, as ErrNoGH
// did before #449 named a missing tool for every forge.
var noGH error = &forge.MissingToolError{Tool: "gh"}

// Regression: the note was drawn as fixed lines and cut at the column's edge,
// so at a 120-column window "gh is not installed, so omatty cann" was all the
// operator read of it. It wraps now, and every word of it is on screen.
func TestTracker_TheNoteWrapsToTheColumn_issue572(t *testing.T) {
	m, fi, fp := modelWithBothLists(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	for _, root := range []string{"/p/omatty", "/p/api-svc", "/p/empty"} {
		fi.Errs[root], fp.Errs[root] = noGH, noGH
	}

	openTracker(m)

	want := "gh is not installed, so omatty cannot read this project's issues or pull requests."
	if got := columnText(m); !strings.Contains(got, want) {
		t.Errorf("the note reads %q, want all of %q", got, want)
	}
}

// columnText is the review column's words in reading order: each frame line
// past its last border, joined by single spaces, so an assertion holds however
// the column wraps.
func columnText(m *ui.Model) string {
	var words []string
	for _, line := range frameLines(m) {
		plain := stripSGR(line)
		if i := strings.LastIndex(plain, "│"); i >= 0 {
			words = append(words, strings.Fields(plain[i+len("│"):])...)
		}
	}
	return strings.Join(words, " ")
}
