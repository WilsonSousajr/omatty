package ui_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func failingReport() gate.Report {
	return gate.Report{
		ID: "s1",
		Results: []gate.StepResult{
			{Step: gate.Step{Name: "fmt", Run: "gofmt -l ."}, Verdict: gate.Pass},
			{
				Step:     gate.Step{Name: "test", Run: "go test ./... -race"},
				Verdict:  gate.Fail,
				ExitCode: 1,
				Output:   "--- FAIL: TestThing\n    thing_test.go:12: got 1, want 2\n",
			},
		},
	}
}

// The payoff of the milestone: one keystroke from "the gate failed" to
// "claude is fixing it", with the operator still the one who decided it
// should happen.
func TestModel_SSendsTheGateFailureIntoTheSession_issue232(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	m.SetGateReport("s1", failingReport())
	leader(m, key('g'))

	press(m, key('S'))

	sent := strings.Join(fakes["s1"].Sent, "")
	for _, want := range []string{"test", "go test ./... -race", "TestThing", "thing_test.go:12"} {
		if !strings.Contains(sent, want) {
			t.Errorf("the message does not carry %q:\n%s", want, sent)
		}
	}
}

// Invariant 8. A multi-line message written raw submits at every newline, so
// claude would receive one prompt per line of test output.
func TestModel_gateFeedbackIsOneBracketedPaste_issue232(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	m.SetGateReport("s1", failingReport())
	leader(m, key('g'))

	press(m, key('S'))

	if n := len(fakes["s1"].Sent); n != 1 {
		t.Fatalf("sent %d messages, want exactly 1 (invariant 8)", n)
	}
	body := fakes["s1"].Sent[0]
	if !strings.HasPrefix(body, "\x1b[200~") {
		t.Errorf("message does not open with the paste start:\n%q", body)
	}
	if !strings.HasSuffix(body, "\x1b[201~\r") || strings.Count(body, "\r") != 1 {
		t.Errorf("message does not end with one paste end and one CR:\n%q", body)
	}
}

// A green gate has nothing to send. Spending a turn to tell a session
// everything is fine costs more than saying nothing.
func TestModel_SOnAGreenGateSendsNothing_issue232(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	m.SetGateReport("s1", gate.Report{
		ID:      "s1",
		Results: []gate.StepResult{{Step: gate.Step{Name: "fmt"}, Verdict: gate.Pass}},
	})
	leader(m, key('g'))

	press(m, key('S'))

	if n := len(fakes["s1"].Sent); n != 0 {
		t.Errorf("sent %d messages for a passing gate, want none", n)
	}
	if !strings.Contains(m.View().Content, "passed") {
		t.Errorf("the operator is not told why nothing was sent:\n%s", m.View().Content)
	}
}

// Nothing to send before a gate has run either, and saying so beats silence.
func TestModel_SBeforeAnyRunSaysSo_issue232(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('g'))

	press(m, key('S'))

	if n := len(fakes["s1"].Sent); n != 0 {
		t.Errorf("sent %d messages with no report, want none", n)
	}
}

// Sending hands the keys back, the way submitting review comments does: the
// next thing the operator wants is to watch the session work.
func TestModel_sendingGateFeedbackHandsTheKeysBack_issue232(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", failingReport())
	leader(m, key('g'))

	press(m, key('S'))

	if m.ReviewFocused() {
		t.Error("the gate pane kept the keyboard after sending")
	}
}

// A Missing step is about the machine, not the code, and the message must not
// read as a code failure - it would send the session off to fix something
// that was never broken (invariant 12).
func TestModel_gateFeedbackForAMissingTool_saysItIsAbsent_issue232(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	m.SetGateReport("s1", gate.Report{
		ID: "s1",
		Results: []gate.StepResult{{
			Step:    gate.Step{Name: "lint", Run: "golangci-lint run"},
			Verdict: gate.Missing,
			Output:  `gate: "golangci-lint" is not on PATH, so this step was not run`,
		}},
	})
	leader(m, key('g'))

	press(m, key('S'))

	sent := strings.Join(fakes["s1"].Sent, "")
	if !strings.Contains(sent, "not on PATH") {
		t.Errorf("the message does not say the tool is absent:\n%s", sent)
	}
	if strings.Contains(strings.ToLower(sent), "failed") {
		t.Errorf("the message reads as a code failure:\n%s", sent)
	}
}

// The footer advertises the key, or nobody finds it.
func TestModel_gateFooterOffersSubmit_issue232(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", failingReport())
	leader(m, key('g'))

	if !strings.Contains(m.View().Content, "S send") {
		t.Errorf("the gate footer does not offer the send key:\n%s", m.View().Content)
	}
	_ = ui.SidebarWidth
}
