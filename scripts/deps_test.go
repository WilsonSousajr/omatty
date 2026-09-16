// These tests guard the dependency-structure gate (#263).
package scripts_test

import (
	"os/exec"
	"strings"
	"testing"
)

// The measurement must actually run and come back clean, so that a local
// `go test ./...` catches an architectural break without anyone remembering
// the new script exists.
//
// Since #269 "clean" includes the Stable Dependencies Principle: the separate
// test that ran it behind --sdp was folded in here when the flag went away, because
// a rule the gate enforces needs no second opinion about whether it holds.
func TestDepsGate_IsGreen(t *testing.T) {
	cmd := exec.Command("./scripts/check-deps.sh")
	cmd.Dir = repoRoot(t)

	out, err := cmd.CombinedOutput()

	if err != nil {
		t.Fatalf("check-deps.sh failed:\n%s", out)
	}
	if !strings.Contains(string(out), "tightest edge") {
		t.Errorf("output does not report the margin:\n%s", out)
	}
	// The margin is printed on every run, clean or not, because instability is
	// a ratio of small integers: internal/config is Ca=1 Ce=1, and one new
	// importer takes it from 0.50 to 0.33. A gate that only ever says "clean"
	// gives no warning before it breaks.
	if !strings.Contains(string(out), "direction of stability") {
		t.Errorf("output does not report on the Stable Dependencies Principle:\n%s", out)
	}
}

// Ordering is not cosmetic: this step costs seconds and the test suite costs
// minutes, so a structural break should be reported first.
func TestCI_RunsTheDependencyGateBeforeTheTests(t *testing.T) {
	workflow := ciWorkflow(t)

	deps := strings.Index(workflow, "./scripts/check-deps.sh")
	tests := strings.Index(workflow, "go test ./... -race")
	if deps < 0 || tests < 0 {
		t.Fatalf("ci.yml is missing a step: deps at %d, tests at %d", deps, tests)
	}
	if tests < deps {
		t.Error("ci.yml runs the test suite before the dependency gate, so a " +
			"structural break waits four minutes to be reported")
	}
}
