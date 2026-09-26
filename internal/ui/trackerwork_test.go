package ui_test

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// FakeBrowse is a named fake for ui.BrowseFunc (#398).
type FakeBrowse struct {
	Asked []string
	Err   error
}

func (f *FakeBrowse) fn(root string, number int) error {
	f.Asked = append(f.Asked, root+" "+strconv.Itoa(number))
	return f.Err
}

// workModel is a tracker open on one issue and one open pull request, with the
// session creator, the browser and the terminals all wired.
func workModel(t *testing.T) (*ui.Model, *recordCreate, *FakeBrowse, map[string]*termwrap.Fake) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	fi := &FakeIssues{Lists: map[string][]forge.Issue{}, Errs: map[string]error{}, Now: fixedNow}
	fp := &FakePRs{Lists: map[string][]forge.PR{}, Errs: map[string]error{}, Now: fixedNow}
	fi.Lists["/p/omatty"] = []forge.Issue{
		{Number: 399, Title: "Filter the tracker, and the argument!", Updated: fixedNow},
	}
	fp.Lists["/p/omatty"] = []forge.PR{{Number: 400, Title: "first slice", State: forge.Open, Updated: fixedNow}}
	create, browse := &recordCreate{}, &FakeBrowse{}
	items := &FakeItems{Issues: map[int]forge.Detail{
		399: {Number: 399, Title: "Filter the tracker, and the argument!", Body: "why"},
	}, PRs: map[int]forge.Detail{}}
	d := baseDeps(withEmptyProject(), terms)
	d.Issues, d.PRs, d.Create, d.Browse = fi.List, fp.List, create.fn, browse.fn
	d.Item = ui.ForgeItemFuncs{Issue: items.issue, PR: items.pr}
	d.Clock = func() time.Time { return fi.Now }
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	openTracker(m)
	return m, create, browse, fakes
}

// n on an issue is the whole point of showing them: a worktree session named
// and branched from the issue, without typing either.
func TestTrackerWork_NStartsAWorktreeSessionOnTheIssue_issue398(t *testing.T) {
	m, create, _, _ := workModel(t)

	pressDeliver(m, key('n'))

	if create.Calls != 1 {
		t.Fatalf("n created %d sessions, want 1", create.Calls)
	}
	if create.Project != "omatty" || !create.Worktree {
		t.Errorf("created in %q worktree=%v, want omatty on a worktree", create.Project, create.Worktree)
	}
	if !strings.Contains(create.Title, "399") {
		t.Errorf("title = %q, want the issue's number in it", create.Title)
	}
	// Slugged, because the branch becomes a directory and a ref: the issue's
	// own punctuation must not reach either (#127).
	if create.Branch != "399-filter-the-tracker-and-the-argument" {
		t.Errorf("branch = %q, want it slugged from the number and title", create.Branch)
	}
}

// The keys go to the new session, so the next thing typed is a prompt to it -
// what attachPath does after pasting (#199).
func TestTrackerWork_NHandsTheKeysToTheNewSession_issue398(t *testing.T) {
	m, _, _, _ := workModel(t)

	pressDeliver(m, key('n'))

	if m.ReviewFocused() {
		t.Error("after n the tracker still owns the keyboard; want the new session to")
	}
}

// n on a pull request row refuses and says why: it already has a branch, and
// checking one out is a different operation from starting work on an issue.
func TestTrackerWork_NOnAPullRequestSaysWhyNot_issue398(t *testing.T) {
	m, create, _, _ := workModel(t)
	pressDeliver(m, key('j')) // over the rule
	pressDeliver(m, key('j'))

	pressDeliver(m, key('n'))

	if create.Calls != 0 {
		t.Errorf("n on a pull request created %d sessions, want none", create.Calls)
	}
	if body := trackerBody(t, m); !strings.Contains(body, "already has a branch") {
		t.Errorf("the footer does not say why n refused:\n%s", body)
	}
}

// a types the reference into the selected session's composer and sends no
// carriage return: omatty types for you and never submits a turn (invariant 8).
func TestTrackerWork_AAttachesTheReferenceUnsent_issue398(t *testing.T) {
	m, _, _, fakes := workModel(t)

	pressDeliver(m, key('a'))

	sent := strings.Join(fakes["s1"].Sent, "")
	if !strings.Contains(sent, "#399") {
		t.Fatalf("the session received %q, want the issue's reference", sent)
	}
	if !strings.Contains(sent, "\x1b[200~") || !strings.Contains(sent, "\x1b[201~") {
		t.Errorf("the reference was not bracketed: %q", sent)
	}
	if strings.Contains(sent, "\r") {
		t.Errorf("a sent a carriage return: %q - that submits the prompt", sent)
	}
	if m.ReviewFocused() {
		t.Error("after a the tracker still owns the keyboard; want the session to")
	}
}

// On a project with no session there is nothing to type into, and it says so
// rather than dropping the keypress.
func TestTrackerWork_AWithNoSessionSaysSo_issue398(t *testing.T) {
	m, _, _, _ := workModel(t)
	for range len(withEmptyProject().Projects) {
		if m.SelectedProject() == "empty" {
			break
		}
		leader(m, key(']'))
	}

	pressDeliver(m, key('a'))

	if body := trackerBody(t, m); !strings.Contains(body, "no session") {
		t.Errorf("a with no session to type into does not say so:\n%s", body)
	}
}

// b hands the item to the operator's browser through their own gh.
func TestTrackerWork_BOpensItInTheBrowser_issue398(t *testing.T) {
	m, _, browse, _ := workModel(t)

	pressDeliver(m, key('b'))

	if len(browse.Asked) != 1 || browse.Asked[0] != "/p/omatty 399" {
		t.Errorf("browse asked %v, want the issue in its project's root", browse.Asked)
	}
}

// A browser that will not open is a footer line, not a silence.
func TestTrackerWork_AFailedBrowseSaysSo_issue398(t *testing.T) {
	m, _, browse, _ := workModel(t)
	browse.Err = errors.New("exec: xdg-open not found")

	pressDeliver(m, key('b'))

	if body := trackerBody(t, m); !strings.Contains(body, "xdg-open") {
		t.Errorf("a failed browse says nothing:\n%s", body)
	}
}

// The three keys work from an open item too: you read it, then act on it,
// without going back to the list first.
func TestTrackerWork_TheKeysWorkFromAnOpenItem_issue398(t *testing.T) {
	m, create, browse, _ := workModel(t)
	pressDeliver(m, special(tea.KeyEnter))
	if m.ReviewView() != ui.ViewTrackerItem {
		t.Fatalf("view = %v, want the item open", m.ReviewView())
	}

	pressDeliver(m, key('b'))
	pressDeliver(m, key('n'))

	if len(browse.Asked) != 1 || create.Calls != 1 {
		t.Errorf("from an item: browsed %v, created %d; want one of each", browse.Asked, create.Calls)
	}
}
