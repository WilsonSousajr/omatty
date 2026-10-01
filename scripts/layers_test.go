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

// -enforce is the switch the migration's last PR flips; on findings it must
// fail, or flipping it would prove nothing.
//
// It ran against this repository's own findings until migration step 6.10
// (#653) cleared the last of them, which left the test asserting that a
// clean tree fails. The claim is unchanged; the findings now come from a
// fixture module whose domain package imports os, so it no longer depends on
// the repository being unfinished.
func TestLayerCheck_enforceFailsOnFindings_issue620(t *testing.T) {
	out, err := runLayerCheckIn(t, layerFixture(t), "-enforce")

	if err == nil {
		t.Fatalf("layercheck -enforce passed with findings present:\n%s", out)
	}
	if !strings.Contains(out, "enforced") || !strings.Contains(out, "domain may not import os") {
		t.Errorf("output does not say the run was enforced and name the finding:\n%s", out)
	}
}

// layerFixture is a module of one domain package that imports os: exactly one
// finding under ADR 0001's table.
func layerFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	pkg := filepath.Join(dir, "internal", "domain", "leaky")
	if err := os.MkdirAll(pkg, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		filepath.Join(dir, "go.mod"):   "module example.com/fixture\n\ngo 1.26\n",
		filepath.Join(pkg, "leaky.go"): "package leaky\n\nimport \"os\"\n\nvar _ = os.Getenv\n",
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// runLayerCheckIn builds layercheck from this repository and runs it in dir,
// a module of its own.
func runLayerCheckIn(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "layercheck")
	build := exec.Command("go", "build", "-o", bin, "./tools/layercheck")
	build.Dir = repoRoot(t)
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building layercheck: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
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
