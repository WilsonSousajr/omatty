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
}

// The Stable Dependencies Principle is green today, which is the whole premise
// for landing it report-only. If this starts failing, the follow-up that turns
// --sdp into the default has a decision to make, and should know before it
// makes it.
func TestDepsGate_StableDependenciesPrincipleHoldsToday(t *testing.T) {
	cmd := exec.Command("./scripts/check-deps.sh", "--sdp")
	cmd.Dir = repoRoot(t)

	out, err := cmd.CombinedOutput()

	if err != nil {
		t.Errorf("an import now runs against the direction of stability:\n%s", out)
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
