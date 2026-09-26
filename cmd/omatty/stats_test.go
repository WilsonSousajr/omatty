package main

import (
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/tally"
)

// #332's two numbers, rendered. A CLI line rather than a card: the sidebar's
// 27 columns already fight over the branch name, and a number read
// occasionally does not belong on something watched continuously.
func TestStatsLines_ReportsBothNumbers_issue332(t *testing.T) {
	lines := strings.Join(statsLines(tally.Numbers{
		LeadTime: 3*time.Hour + 30*time.Minute, Merged: 4, GateRuns: 8, FirstPass: 0.75,
	}), "\n")

	for _, want := range []string{"lead time", "3h30m", "4", "first-pass", "75", "8"} {
		if !strings.Contains(lines, want) {
			t.Errorf("%q missing from:\n%s", want, lines)
		}
	}
}

// Nothing measured says so. A rate of 0% over no runs would read as "the gate
// never passes", and a lead time of 0s as "everything ships instantly".
func TestStatsLines_SaysWhenNothingIsMeasuredYet_issue332(t *testing.T) {
	lines := strings.Join(statsLines(tally.Numbers{}), "\n")

	if strings.Contains(lines, "0%") || strings.Contains(lines, "0s") {
		t.Errorf("an unmeasured project reports a number:\n%s", lines)
	}
	if !strings.Contains(lines, "nothing measured") {
		t.Errorf("it does not say why there are no numbers:\n%s", lines)
	}
}

// Half measured is reported half: the counters exist without a merged pull
// request, and one number missing must not hide the other.
func TestStatsLines_ReportsTheGateRateWithNoMergedWorkYet_issue332(t *testing.T) {
	lines := strings.Join(statsLines(tally.Numbers{GateRuns: 4, FirstPass: 0.5}), "\n")

	if !strings.Contains(lines, "50") {
		t.Errorf("the gate rate went missing:\n%s", lines)
	}
	if !strings.Contains(lines, "no merged") {
		t.Errorf("it does not say why there is no lead time:\n%s", lines)
	}
}

// --stats reads and prints; it must never write. The flag sits beside
// --detect, which has the same contract.
func TestGateCommand_statsWritesNothing_issue332(t *testing.T) {
	store, _ := gateFixture(t)

	if err := gateCommand(store, []string{"omatty", "--stats"}, strings.NewReader("")); err != nil {
		t.Fatalf("gateCommand(--stats) error = %v", err)
	}

	if got := configuredGate(t, store); got != nil {
		t.Errorf("gate = %v, want --stats to have written nothing", got)
	}
}

// A project that is not registered is a usage error naming it, as it is for
// every other form of this command.
func TestGateCommand_statsNeedsAKnownProject_issue332(t *testing.T) {
	store, _ := gateFixture(t)

	if err := gateCommand(store, []string{"ghost", "--stats"}, strings.NewReader("")); err == nil {
		t.Error("--stats on an unknown project should fail")
	}
}

var _ = registry.Project{}

// A lead time is hours and minutes. Duration.String leaves "0s" on a value
// rounded to the minute, which spends two cells saying nothing.
func TestStatsLines_LeadTimeDropsTheSeconds_issue332(t *testing.T) {
	lines := strings.Join(statsLines(tally.Numbers{
		LeadTime: 3*time.Hour + 54*time.Minute + 12*time.Second, Merged: 2,
	}), "\n")

	if !strings.Contains(lines, "3h54m") {
		t.Errorf("lead time is not rounded to the minute:\n%s", lines)
	}
	if strings.Contains(lines, "3h54m0s") {
		t.Errorf("the trailing 0s survived:\n%s", lines)
	}
}
