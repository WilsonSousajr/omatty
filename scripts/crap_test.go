// These tests guard the C.R.A.P. gate script and its wiring into CI (#262).
package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The threshold must actually be compared, not merely printed - the lesson of
// #27, where the coverage gate reported a number that meant nothing.
//
// A profile holding no records scores every function at zero coverage, so at a
// threshold of 1 every function crosses it. It is written fresh, so it is
// newer than the source and the staleness guard lets it through; that keeps
// this test to one `go run` rather than a second full race suite.
func TestCrapGate_ComparesTheThreshold(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "empty.out")
	if err := os.WriteFile(profile, []byte("mode: set\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("./scripts/check-crap.sh", "1", profile)
	cmd.Dir = repoRoot(t)

	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("gate passed at a threshold of 1, want failure:\n%s", out)
	}
	if !strings.Contains(string(out), "reach the CRAP") {
		t.Errorf("output does not explain the failure:\n%s", out)
	}
}

// A profile older than the source is refused rather than scored. Blocks carry
// positions, so scoring a stale one reports a confident wrong number - the one
// failure mode of reusing the coverage step's output.
func TestCrapGate_RefusesAStaleProfile(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "old.out")
	if err := os.WriteFile(profile, []byte("mode: set\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	stale := mustParseAncientTime(t)
	if err := os.Chtimes(profile, stale, stale); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("./scripts/check-crap.sh", "999", profile)
	cmd.Dir = repoRoot(t)

	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("gate scored a profile older than the tree:\n%s", out)
	}
	if !strings.Contains(string(out), "older than") {
		t.Errorf("output does not say the profile is stale:\n%s", out)
	}
}

// The default in the script and the number CI passes must agree, or the gate
// enforced locally is not the gate enforced on CI. The coverage threshold
// already lives in three places; this keeps the second one from doing the same.
func TestCrapGate_DefaultThresholdMatchesCI(t *testing.T) {
	script, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "check-crap.sh"))
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(script), `threshold="${1:-15}"`) {
		t.Error("check-crap.sh does not default to 15")
	}
	if !strings.Contains(ciWorkflow(t), "./scripts/check-crap.sh 15") {
		t.Error("ci.yml does not run the crap gate at 15")
	}
}

// Ordering is not cosmetic: the crap gate reads the profile the coverage gate
// writes. Run the other way round it would regenerate it, doubling the test
// suite's cost on both runners with no visible symptom.
func TestCI_RunsTheCrapGateAfterTheCoverageGate(t *testing.T) {
	workflow := ciWorkflow(t)

	coverage := strings.Index(workflow, "./scripts/check-coverage.sh")
	crap := strings.Index(workflow, "./scripts/check-crap.sh")
	if coverage < 0 || crap < 0 {
		t.Fatalf("ci.yml is missing a gate step: coverage at %d, crap at %d", coverage, crap)
	}
	if crap < coverage {
		t.Error("ci.yml runs the crap gate before the coverage gate, so it " +
			"regenerates the profile instead of reusing it")
	}
}

// mustParseAncientTime is a moment safely before any file in the checkout.
func mustParseAncientTime(t *testing.T) time.Time {
	t.Helper()
	return time.Now().Add(-100 * 24 * time.Hour)
}
