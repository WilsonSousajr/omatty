package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// A forge that keeps no issues for the project - Bitbucket, whose issues are
// in Jira - still shows the project's pull requests, and says quietly where
// its issues are not, rather than an error or a count of zero (#460).
func TestTracker_AProjectWithNoTrackerShowsItsChanges_issue460(t *testing.T) {
	m, fi, fp := modelWithBothLists(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	fi.Errs["/p/omatty"] = forge.ErrNoTracker
	fp.Lists["/p/omatty"] = []forge.PR{{Number: 1115, Title: "remote code", State: forge.Open, Updated: fixedNow}}

	openTracker(m)

	body := stripSGR(trackerBody(t, m))
	if !strings.Contains(body, "#1115") || !strings.Contains(body, "issues elsewhere") {
		t.Errorf("the tracker does not show the change and the quiet note:\n%s", body)
	}
	if header := stripSGR(omattyHeader(t, m)); strings.Contains(header, "0i") || !strings.Contains(header, "1p") {
		t.Errorf("header = %q, want the change count and no issue count", header)
	}
}

// Its issues are not asked for again: the answer will not change.
func TestModel_ANoTrackerProjectIsNotAskedForIssuesAgain_issue460(t *testing.T) {
	m, f := modelWithIssues(t)
	f.Errs["/p/omatty"] = forge.ErrNoTracker
	deliver(m, m.PollIssues())
	f.Asked = nil
	f.later()

	deliver(m, m.PollIssues())

	if strings.Contains(f.asked(), "/p/omatty") {
		t.Errorf("asked %q again for a project with no tracker", f.asked())
	}
}
