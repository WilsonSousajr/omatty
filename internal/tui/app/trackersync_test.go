package app_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// refused is a token the forge turned away: an expired gh login, a variable
// missing at launch. It used to stop the project for the rest of the run.
var refused error = &forge.AuthError{Host: "github.com", TokenEnv: "GH_TOKEN", Status: 401}

// stoppedThenFixed is a tracker on omatty whose first read was refused and
// whose forge now answers: one issue, one pull request.
func stoppedThenFixed(t *testing.T) (*app.Model, *FakeIssues, *FakePRs) {
	t.Helper()
	m, fi, fp := modelWithBothLists(t)
	size(m)
	fi.Errs["/p/omatty"], fp.Errs["/p/omatty"] = refused, refused
	openTracker(m)
	delete(fi.Errs, "/p/omatty")
	delete(fp.Errs, "/p/omatty")
	fi.Lists["/p/omatty"] = []forge.Issue{{Number: 658, Title: "not synced", Updated: fixedNow}}
	fp.Lists["/p/omatty"] = []forge.PR{{Number: 700, Title: "a new one", State: forge.Open, Updated: fixedNow}}
	fi.Asked, fp.Asked = nil, nil
	return m, fi, fp
}

// One refused read stopped both lists until omatty restarted (#658): r in the
// tracker reads again, and the lists come back.
func TestTracker_rRecoversARefusedForge_issue658(t *testing.T) {
	m, _, _ := stoppedThenFixed(t)

	pressDeliver(m, key('r'))

	body := stripSGR(trackerBody(t, m))
	if !strings.Contains(body, "#658") || !strings.Contains(body, "#700") {
		t.Errorf("r after the forge recovered did not bring the lists back:\n%s", body)
	}
}

// Without r, the issue tick tries a stopped project again, so a token fixed
// in another terminal is picked up within one period (#658).
func TestModel_theIssueTickRetriesARefusedForge_issue658(t *testing.T) {
	m, fi, _ := stoppedThenFixed(t)
	fi.Now = fi.Now.Add(5 * time.Minute)

	deliver(m, m.IssueTickPolls())

	if got := m.IssuesOf("omatty"); len(got) != 1 {
		t.Errorf("IssuesOf(omatty) = %+v after a tick, want the recovered list", got)
	}
	if got := m.PRsOf("omatty"); len(got) != 1 {
		t.Errorf("PRsOf(omatty) = %+v after a tick, want the recovered list", got)
	}
}

// A retry that is refused again keeps the stop and its note, and says nothing
// new in the log: the stop is the same fact as before.
func TestModel_aRetryRefusedAgainKeepsTheStop_issue658(t *testing.T) {
	m, fi, fp := stoppedThenFixed(t)
	fi.Errs["/p/omatty"], fp.Errs["/p/omatty"] = refused, refused

	pressDeliver(m, key('r'))

	if body := stripSGR(trackerBody(t, m)); !strings.Contains(body, "refused GH_TOKEN") {
		t.Errorf("a second refusal lost the stopped note:\n%s", body)
	}
}

// A checkout on no forge is a fact about the checkout: r does not hammer it.
func TestTracker_rDoesNotRetryACheckoutOnNoForge_issue658(t *testing.T) {
	m, fi, fp := modelWithBothLists(t)
	size(m)
	fi.Errs["/p/omatty"], fp.Errs["/p/omatty"] = forge.ErrNoForge, forge.ErrNoForge
	openTracker(m)
	fi.Asked, fp.Asked = nil, nil

	pressDeliver(m, key('r'))

	if len(fi.Asked)+len(fp.Asked) != 0 {
		t.Errorf("r asked %v %v of a checkout on no forge", fi.Asked, fp.Asked)
	}
}

// noTracker was as sticky: one ErrNoTracker and the issues were never asked
// again, even on r (#658). r asks again.
func TestTracker_rAsksForIssuesAfterNoTracker_issue658(t *testing.T) {
	m, fi, _ := modelWithBothLists(t)
	size(m)
	fi.Errs["/p/omatty"] = forge.ErrNoTracker
	openTracker(m)
	delete(fi.Errs, "/p/omatty")
	fi.Lists["/p/omatty"] = []forge.Issue{{Number: 658, Title: "issues are back", Updated: fixedNow}}

	pressDeliver(m, key('r'))

	if body := stripSGR(trackerBody(t, m)); !strings.Contains(body, "#658") {
		t.Errorf("r did not read the issues again:\n%s", body)
	}
}

// r inside the thirty-second floor was refused without a word, which reads as
// "not syncing" (#658). An explicit r goes past the floor.
func TestTracker_rReadsInsideTheFloor_issue658(t *testing.T) {
	m, fi, fp := modelWithBothLists(t)
	size(m)
	openTracker(m)
	fi.Lists["/p/omatty"] = []forge.Issue{{Number: 658, Title: "opened a moment ago", Updated: fixedNow}}
	fi.Asked, fp.Asked = nil, nil

	pressDeliver(m, key('r'))

	if body := stripSGR(trackerBody(t, m)); !strings.Contains(body, "#658") {
		t.Errorf("r inside the floor did not read again:\n%s", body)
	}
}

// r while a read is still in flight cannot ask twice, and says so.
func TestTracker_rWhileReadingSaysSo_issue658(t *testing.T) {
	m, _, _ := modelWithBothLists(t)
	size(m)
	m.Update(ctrl('o'))
	m.Update(key('i')) // opens the tracker; its reads are left in flight

	m.Update(key('r'))

	if footer := stripSGR(footerOf(m)); !strings.Contains(footer, "still reading") {
		t.Errorf("footer = %q, want it to say the read is still in flight", footer)
	}
}

// A project holding no session never had its pull requests polled on a tick
// (#658): the issue tick reads them too.
func TestModel_theIssueTickReadsAnIdleProjectsPRs_issue658(t *testing.T) {
	m, _, fp := modelWithBothLists(t)
	fp.Lists["/p/empty"] = []forge.PR{{Number: 7, Title: "idle", State: forge.Open, Updated: fixedNow}}

	deliver(m, m.IssueTickPolls())

	if got := m.PRsOf("empty"); len(got) != 1 {
		t.Errorf("PRsOf(empty) = %+v after an issue tick, want its pull requests", got)
	}
}

// A list that fills its window says so: the hundred-and-first issue is not
// read, and "100 issues" read as all of them (#658).
func TestTracker_aFullListSaysItIsCut_issue658(t *testing.T) {
	m, fi, _ := modelWithBothLists(t)
	size(m)
	fi.Lists["/p/omatty"] = openIssues(forge.ListWindow)

	openTracker(m)

	if body := stripSGR(trackerBody(t, m)); !strings.Contains(body, "100+ issues") {
		t.Errorf("the title does not say the list was cut:\n%s", body)
	}
	if header := stripSGR(omattyHeader(t, m)); !strings.Contains(header, "100+i") {
		t.Errorf("header = %q, want 100+i", header)
	}
}

// A terminal that sends focus-out and loses the focus-in left polling off for
// good (#658). A key reaches only the focused window, so a key is focus.
func TestModel_aKeyAfterALostFocusInResumesPolling_issue658(t *testing.T) {
	m, f := modelWithIssues(t)
	m.Update(tea.BlurMsg{})

	_, cmd := m.Update(key('x'))
	deliver(m, cmd)

	if len(f.Asked) == 0 {
		t.Error("no poll after a key; the lost focus-in still holds polling off")
	}
}
