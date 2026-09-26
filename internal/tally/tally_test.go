package tally_test

import (
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/tally"
)

var start = time.Date(2026, 9, 26, 9, 0, 0, 0, time.UTC)

// #332's first number: session start to the pull request merging. omatty holds
// every timestamp it needs already - it had just never measured anything.
func TestOf_LeadTimeIsSessionStartToTheMergedPullRequest_issue332(t *testing.T) {
	project := registry.Project{Name: "omatty", Root: "/p/omatty"}
	sessions := []registry.Session{
		{ID: "s1", Project: "omatty", Branch: "feat/a", Started: start},
		{ID: "s2", Project: "omatty", Branch: "feat/b", Started: start},
	}
	prs := []forge.PR{
		{Number: 1, Branch: "feat/a", State: forge.Merged, MergedAt: start.Add(2 * time.Hour)},
		{Number: 2, Branch: "feat/b", State: forge.Merged, MergedAt: start.Add(4 * time.Hour)},
	}

	got := tally.Of(project, sessions, prs)

	if got.Merged != 2 {
		t.Fatalf("Merged = %d, want the two merged sessions", got.Merged)
	}
	if want := 3 * time.Hour; got.LeadTime != want {
		t.Errorf("LeadTime = %v, want the mean %v", got.LeadTime, want)
	}
}

// An open pull request is not a lead time yet, and a session with no start time
// - every one written before #332 - cannot contribute one.
func TestOf_CountsOnlyWhatItCanMeasure_issue332(t *testing.T) {
	project := registry.Project{Name: "omatty"}
	sessions := []registry.Session{
		{ID: "s1", Project: "omatty", Branch: "feat/a", Started: start},
		{ID: "s2", Project: "omatty", Branch: "feat/open", Started: start},
		{ID: "s3", Project: "omatty", Branch: "feat/old"}, // no Started
	}
	prs := []forge.PR{
		{Number: 1, Branch: "feat/a", State: forge.Merged, MergedAt: start.Add(time.Hour)},
		{Number: 2, Branch: "feat/open", State: forge.Open},
		{Number: 3, Branch: "feat/old", State: forge.Merged, MergedAt: start.Add(9 * time.Hour)},
	}

	got := tally.Of(project, sessions, prs)

	if got.Merged != 1 {
		t.Errorf("Merged = %d, want only the one that is both merged and timed", got.Merged)
	}
	if got.LeadTime != time.Hour {
		t.Errorf("LeadTime = %v, want 1h", got.LeadTime)
	}
}

// Another project's sessions are not this project's measurement.
func TestOf_IgnoresOtherProjectsSessions_issue332(t *testing.T) {
	project := registry.Project{Name: "omatty"}
	sessions := []registry.Session{
		{ID: "s1", Project: "omatty", Branch: "feat/a", Started: start},
		{ID: "s2", Project: "api-svc", Branch: "feat/a", Started: start},
	}
	prs := []forge.PR{{Number: 1, Branch: "feat/a", State: forge.Merged, MergedAt: start.Add(time.Hour)}}

	if got := tally.Of(project, sessions, prs).Merged; got != 1 {
		t.Errorf("Merged = %d, want 1", got)
	}
}

// #332's second number, straight off the counters the UI keeps.
func TestOf_FirstPassRateIsTheShareThatPassed_issue332(t *testing.T) {
	project := registry.Project{Name: "omatty", GateRuns: 8, GateFirstPass: 6}

	got := tally.Of(project, nil, nil)

	if got.GateRuns != 8 {
		t.Errorf("GateRuns = %d, want 8", got.GateRuns)
	}
	if want := 0.75; got.FirstPass != want {
		t.Errorf("FirstPass = %v, want %v", got.FirstPass, want)
	}
}

// Nothing measured is not zero percent. A rate over no runs is not a number,
// and reporting 0% would read as "the gate never passes".
func TestOf_NoRunsIsNotAZeroRate_issue332(t *testing.T) {
	got := tally.Of(registry.Project{Name: "omatty"}, nil, nil)

	if got.Measured() {
		t.Error("Measured() is true with nothing measured")
	}
	if got.FirstPass != 0 || got.GateRuns != 0 {
		t.Errorf("got %+v, want the zero value", got)
	}
}

// A merged pull request whose branch no session has is somebody else's work,
// or work from before omatty. It is not this project's lead time.
func TestOf_APullRequestWithNoSessionIsNotMeasured_issue332(t *testing.T) {
	prs := []forge.PR{{Number: 1, Branch: "someone-else", State: forge.Merged, MergedAt: start}}

	if got := tally.Of(registry.Project{Name: "omatty"}, nil, prs).Merged; got != 0 {
		t.Errorf("Merged = %d, want 0", got)
	}
}

// A clock that went backwards - a merge recorded before the session started,
// which a wrong system clock or a re-used branch name can produce - is not a
// negative lead time. It is not a measurement.
func TestOf_RefusesANegativeLeadTime_issue332(t *testing.T) {
	sessions := []registry.Session{{ID: "s1", Project: "omatty", Branch: "feat/a", Started: start}}
	prs := []forge.PR{{Number: 1, Branch: "feat/a", State: forge.Merged, MergedAt: start.Add(-time.Hour)}}

	if got := tally.Of(registry.Project{Name: "omatty"}, sessions, prs); got.Merged != 0 {
		t.Errorf("Merged = %d with a merge before the start, want 0", got.Merged)
	}
}
