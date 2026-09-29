// These tests guard the report-only layer check (#620).
package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runLayerCheck(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("./scripts/check-layers.sh", args...)
	cmd.Dir = repoRoot(t)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// The check is report-only until the migration's last PR (ADR 0001): the
// repository has findings today, and a report that failed CI on them would
// block every pull request that is not the migration.
func TestLayerCheck_reportsButPassesByDefault_issue620(t *testing.T) {
	out, err := runLayerCheck(t)

	if err != nil {
		t.Fatalf("check-layers.sh failed in report mode: %v\n%s", err, out)
	}
	if !strings.Contains(out, "layer findings:") || !strings.Contains(out, "report only") {
		t.Errorf("output does not report the findings count as report-only:\n%s", out)
	}
}

// -enforce is the switch the migration's last PR flips; with today's findings
// it must fail, or flipping it would prove nothing.
func TestLayerCheck_enforceFailsOnFindings_issue620(t *testing.T) {
	out, err := runLayerCheck(t, "-enforce")

	if err == nil {
		t.Fatalf("check-layers.sh -enforce passed with findings present:\n%s", out)
	}
	if !strings.Contains(out, "enforced") {
		t.Errorf("output does not say the run was enforced:\n%s", out)
	}
}

func TestCI_runsTheLayerReport_issue620(t *testing.T) {
	if !strings.Contains(ciWorkflow(t), "./scripts/check-layers.sh") {
		t.Error("ci.yml does not run the layer report")
	}
	agents, err := os.ReadFile(filepath.Join(repoRoot(t), "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), "./scripts/check-layers.sh") {
		t.Error("AGENTS.md's gate list does not include the layer report")
	}
}
