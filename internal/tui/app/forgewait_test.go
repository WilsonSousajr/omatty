package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// onHosts is a model whose omatty and api-svc are on bitbucket.org and whose
// empty project is on gitlab.com, both lists wired, its clock in the fakes.
func onHosts(t *testing.T) (*app.Model, *FakeIssues, *FakePRs, *time.Time) {
	t.Helper()
	terms, _ := fakeTerms(t)
	clock := fixedNow
	fi := &FakeIssues{Lists: map[string][]forge.Issue{}, Errs: map[string]error{}, Now: fixedNow}
	fp := &FakePRs{Lists: map[string][]forge.PR{}, Errs: map[string]error{}, Now: fixedNow}
	hosts := map[string]string{"/p/omatty": "bitbucket.org", "/p/api-svc": "bitbucket.org", "/p/empty": "gitlab.com"}
	d := baseDeps(withEmptyProject(), terms)
	d.Issues, d.PRs = fi.List, fp.List
	d.Host = func(root string) string { return hosts[root] }
	d.Clock = func() time.Time { return clock }
	return app.NewModel(d), fi, fp, &clock
}

// limited is a 429 from bitbucket.org, with the time it named or none.
func limited(until time.Time) error {
	return &forge.RateLimitError{Host: "bitbucket.org", Until: until}
}

// asks reports whether root was asked since the fake's log was last cleared.
func asks(asked []string, root string) bool {
	for _, a := range asked {
		if a == root {
			return true
		}
	}
	return false
}

// #594: a 429 waits for the time the forge named, rather than asking again on
// the next tick and lengthening the outage.
func TestPRPoll_WaitsForRetryAfter_issue594(t *testing.T) {
	m, _, fp, clock := onHosts(t)
	fp.Errs["/p/omatty"] = limited(fixedNow.Add(10 * time.Minute))
	deliver(m, m.PollPRs())
	fp.Errs["/p/omatty"], fp.Asked = nil, nil

	*clock = fixedNow.Add(9 * time.Minute)
	deliver(m, m.PollPRs())
	if asks(fp.Asked, "/p/omatty") {
		t.Fatalf("asked %v at 9 minutes, want omatty left until 10", fp.Asked)
	}
	*clock = fixedNow.Add(10 * time.Minute)
	deliver(m, m.PollPRs())
	if !asks(fp.Asked, "/p/omatty") {
		t.Errorf("asked %v at 10 minutes, want omatty asked again", fp.Asked)
	}
}

// The quota is the account's: a project on the same host that never saw the
// 429 waits too, for both lists - and an answer it got for a request sent
// before the 429 does not lift the wait.
func TestPRPoll_SiblingOnSameHostWaits_issue594(t *testing.T) {
	m, fi, fp, clock := onHosts(t)
	fp.Errs["/p/omatty"] = limited(fixedNow.Add(10 * time.Minute))
	deliver(m, m.PollPRs())
	fp.Asked, fi.Asked = nil, nil

	*clock = fixedNow.Add(time.Minute)
	deliver(m, m.PollPRs())
	deliver(m, m.PollIssues())
	if asks(fp.Asked, "/p/api-svc") || asks(fi.Asked, "/p/api-svc") || asks(fi.Asked, "/p/omatty") {
		t.Errorf("asked PRs %v and issues %v, want nothing on bitbucket.org", fp.Asked, fi.Asked)
	}
	if !asks(fi.Asked, "/p/empty") {
		t.Errorf("asked issues %v, want gitlab.com's project asked: its host is not waiting", fi.Asked)
	}
}

// No time named is a backoff: five minutes, doubling to an hour.
func TestPRPoll_BackoffDoublesToAnHour_issue594(t *testing.T) {
	m, _, fp, clock := onHosts(t)
	fp.Errs["/p/omatty"] = limited(time.Time{})
	for _, want := range []time.Duration{5, 10, 20, 40, 60, 60} {
		start := *clock
		deliver(m, m.PollPRs())
		if got := m.HostWaitUntil("omatty").Sub(start); got != want*time.Minute {
			t.Fatalf("wait = %v, want %v", got, want*time.Minute)
		}
		*clock = m.HostWaitUntil("omatty")
	}
}

// An answer once the wait is over lifts it and resets the backoff.
func TestPRPoll_AnswerClearsTheWait_issue594(t *testing.T) {
	m, _, fp, clock := onHosts(t)
	fp.Errs["/p/omatty"] = limited(time.Time{})
	deliver(m, m.PollPRs())
	*clock = m.HostWaitUntil("omatty")
	deliver(m, m.PollPRs())
	*clock = m.HostWaitUntil("omatty")
	fp.Errs["/p/omatty"] = nil
	deliver(m, m.PollPRs())
	if !m.HostWaitUntil("api-svc").IsZero() {
		t.Fatalf("wait = %v after an answer, want none", m.HostWaitUntil("api-svc"))
	}
	fp.Errs["/p/omatty"] = limited(time.Time{})
	*clock = clock.Add(time.Minute)
	start := *clock
	deliver(m, m.PollPRs())
	if got := m.HostWaitUntil("omatty").Sub(start); got != 5*time.Minute {
		t.Errorf("wait after a reset = %v, want 5m", got)
	}
}

// A rate limit is not a stop: the last list stays, marked unknown, and
// nothing about the project is dropped.
func TestPRPoll_RateLimitIsNotAStop_issue594(t *testing.T) {
	m, _, fp, clock := onHosts(t)
	fp.Lists["/p/omatty"] = []forge.PR{{Number: 7, Branch: "feat-x"}}
	deliver(m, m.PollPRs())
	fp.Errs["/p/omatty"] = limited(fixedNow.Add(10 * time.Minute))
	*clock = fixedNow.Add(time.Minute)
	deliver(m, m.PollPRs())
	if prs := m.PRsOf("omatty"); len(prs) != 1 || !m.PRFailed("omatty") {
		t.Errorf("PRsOf = %+v, failed = %v; want the last list kept and marked unknown", prs, m.PRFailed("omatty"))
	}
}

// The tracker says why nothing new is coming, and until when.
func TestTracker_SaysRateLimitedUntil_issue594(t *testing.T) {
	m, fi, fp, _ := onHosts(t)
	size(m)
	fi.Errs["/p/omatty"] = limited(fixedNow.Add(10 * time.Minute))
	fp.Errs["/p/omatty"] = fi.Errs["/p/omatty"]
	deliver(m, m.PollIssues())
	openTracker(m)
	body := stripSGR(trackerBody(t, m))
	// The pane wraps the line, so its two halves are looked for apart.
	if !strings.Contains(body, "rate-limited by bitbucket.org") || !strings.Contains(body, "until 12:10") {
		t.Errorf("the tracker does not say it is rate-limited:\n%s", body)
	}
}

// A wait keeps the rows already read - it is not a stop - and the title's
// unknown mark says until when.
func TestTracker_AWaitKeepsTheRowsAndTitlesTheTime_issue594(t *testing.T) {
	m, fi, fp, clock := onHosts(t)
	size(m)
	fi.Lists["/p/omatty"] = []forge.Issue{{Number: 399, Title: "filter the tracker"}}
	deliver(m, m.PollIssues())
	fi.Errs["/p/omatty"] = limited(fixedNow.Add(10 * time.Minute))
	fp.Errs["/p/omatty"] = fi.Errs["/p/omatty"]
	*clock = fixedNow.Add(time.Minute)
	deliver(m, m.PollIssues())
	openTracker(m)
	body := stripSGR(trackerBody(t, m))
	if !strings.Contains(body, "#399") || !strings.Contains(body, "? until 12:10") {
		t.Errorf("want #399 kept and the title saying \"? until 12:10\":\n%s", body)
	}
}
