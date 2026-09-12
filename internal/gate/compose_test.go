package gate_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/gate"
)

func TestCompose_namesTheFailingStepAndItsCommand(t *testing.T) {
	results := []gate.StepResult{
		{Step: gate.Step{Name: "fmt", Run: "gofmt -l ."}, Verdict: gate.Pass},
		{Step: gate.Step{Name: "test", Run: "go test ./... -race"}, Verdict: gate.Fail, ExitCode: 1,
			Output: "--- FAIL: TestThing\n    thing_test.go:12: got 1, want 2\n"},
	}

	got := gate.Compose(results)

	for _, want := range []string{"test", "go test ./... -race", "exit 1", "TestThing", "thing_test.go:12"} {
		if !strings.Contains(got, want) {
			t.Errorf("Compose() missing %q:\n%s", want, got)
		}
	}
}

// A passing run has nothing to say. Sending "everything is fine" into a live
// session spends a turn to tell it what it already knows.
func TestCompose_everythingPassed_isEmpty(t *testing.T) {
	results := []gate.StepResult{{Step: gate.Step{Name: "fmt"}, Verdict: gate.Pass}}

	if got := gate.Compose(results); got != "" {
		t.Errorf("Compose() = %q, want empty for a green run", got)
	}
}

// A Missing step is a message about the machine, not about the code, and it
// must not read as "your lint is failing".
func TestCompose_missingTool_saysItIsNotInstalled(t *testing.T) {
	results := []gate.StepResult{
		{Step: gate.Step{Name: "lint", Run: "golangci-lint run"}, Verdict: gate.Missing,
			Output: `gate: "golangci-lint" is not on PATH`},
	}

	got := gate.Compose(results)

	if !strings.Contains(got, "not on PATH") {
		t.Errorf("Compose() = %q, want it to say the tool is absent", got)
	}
	if strings.Contains(strings.ToLower(got), "failed") {
		t.Errorf("Compose() = %q, want it not to read as a code failure", got)
	}
}

// Invariant 8's sibling: what reaches a session is bounded, whatever the step
// wrote. MaxOutputLines is the per-step cap; the message caps again.
func TestCompose_boundsTheMessageBelowThePerStepCap(t *testing.T) {
	var big strings.Builder
	for i := 0; i < gate.MaxOutputLines; i++ {
		big.WriteString("a line of failure output\n")
	}
	results := []gate.StepResult{
		{Step: gate.Step{Name: "test", Run: "go test"}, Verdict: gate.Fail, ExitCode: 1, Output: big.String()},
	}

	got := gate.Compose(results)

	if lines := strings.Count(got, "\n"); lines > gate.MaxComposeLines+6 {
		t.Errorf("Compose() wrote %d lines, want at most %d plus its header", lines, gate.MaxComposeLines)
	}
}
