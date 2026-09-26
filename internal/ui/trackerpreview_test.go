package ui_test

import (
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// Zoomed and wide, the tracker shows the list and the item under the cursor
// side by side, as gh-dash's preview does (#434).
func TestTracker_ZoomedPreviewsTheItemBesideTheList_issue434(t *testing.T) {
	m, _ := itemModel(t)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 32})
	openTracker(m)
	openTracker(m)
	leader(m, key('z'))

	row := rowContaining(t, m, "#397")
	if !strings.Contains(stripSGR(m.View().Content), "the body") || !strings.Contains(row, "│") {
		t.Errorf("the zoomed tracker does not show #397's body beside the list:\n%s", stripSGR(m.View().Content))
	}
}

// previewModel is a zoomed, wide tracker over four issues and a pull request,
// every one readable, with the cursor on the first issue already read.
func previewModel(t *testing.T) (*ui.Model, *FakeItems) {
	t.Helper()
	terms, _ := fakeTerms(t)
	fi := &FakeIssues{Lists: map[string][]forge.Issue{}, Errs: map[string]error{}, Now: fixedNow}
	fp := &FakePRs{Lists: map[string][]forge.PR{}, Errs: map[string]error{}, Now: fixedNow}
	items := &FakeItems{Issues: map[int]forge.Detail{}, PRs: map[int]forge.Detail{}}
	for _, n := range []int{391, 392, 393, 394} {
		fi.Lists["/p/omatty"] = append(fi.Lists["/p/omatty"], forge.Issue{Number: n, Title: "issue", Updated: fixedNow})
		items.Issues[n] = forge.Detail{Number: n, Title: "issue", Body: "body of " + strconv.Itoa(n)}
	}
	fp.Lists["/p/omatty"] = []forge.PR{{Number: 400, Title: "pr", State: forge.Open, Updated: fixedNow}}
	d := baseDeps(withEmptyProject(), terms)
	d.Issues, d.PRs = fi.List, fp.List
	d.Item = ui.ForgeItemFuncs{Issue: items.issue, PR: items.pr}
	d.Clock = func() time.Time { return fi.Now }
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 32})
	openTracker(m)
	leader(m, key('z'))
	return m, items
}

// Moving through the list reads an item only once the cursor has rested on
// it, not once per row passed: a burst of j over three rows is one gh call,
// for the row it stopped on (#358's rate-limit lesson).
func TestTracker_ThePreviewReadsOnlyWhereTheCursorRests_issue434(t *testing.T) {
	m, items := previewModel(t)
	asked := len(items.Asked)

	var pending []tea.Cmd
	for range 3 { // #392, #393, #394 - the burst stops on #394
		_, cmd := m.Update(key('j'))
		pending = append(pending, cmd)
	}
	for _, cmd := range pending {
		deliver(m, cmd)
	}

	if got := items.Asked[asked:]; len(got) != 1 || !strings.HasSuffix(got[0], " 394") {
		t.Errorf("a burst of j read %v, want one read, of #394 where it stopped", got)
	}
	if !strings.Contains(stripSGR(m.View().Content), "body of 394") {
		t.Errorf("the preview does not show where the cursor rests:\n%s", stripSGR(m.View().Content))
	}
}

// Unzoomed, or not wide enough, the tracker is the list alone, as before.
func TestTracker_NoPreviewUnzoomed_issue434(t *testing.T) {
	m, items := itemModel(t)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 32})
	openTracker(m)
	openTracker(m)
	asked := len(items.Asked)

	if strings.Contains(stripSGR(m.View().Content), "the body") || len(items.Asked) != asked {
		t.Errorf("an unzoomed tracker previewed or read an item")
	}
}
