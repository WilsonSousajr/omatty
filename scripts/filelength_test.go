// These tests guard the file-length gate (#609).
package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// writeLines writes a .go file of exactly n newline-terminated lines, the
// shape gofmt leaves every file in, so the count wc reports is n.
func writeLines(t *testing.T, name string, n int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(strings.Repeat("x\n", n)), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// runFileLength runs the gate from the repository root with args.
func runFileLength(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	script := filepath.Join(repoRoot(t), "scripts", "check-file-length.sh")
	cmd := exec.Command(script, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Regression, issue #609: "files under 500 lines" was written down and never
// checked, so six files passed it after #136 had already fixed the same thing
// once. The negative control: one line over the limit must fail and name the
// file.
func TestFileLengthGate_FailsOnAFileOverTheLimit_issue609(t *testing.T) {
	long := writeLines(t, "long.go", 501)

	out, err := runFileLength(t, repoRoot(t), "500", long)

	if err == nil {
		t.Fatalf("a 501-line file passed the 500-line gate:\n%s", out)
	}
	if !strings.Contains(out, "501 "+long) {
		t.Errorf("output does not name the file and its length:\n%s", out)
	}
}

// The limit is inclusive: exactly 500 lines is within it.
func TestFileLengthGate_PassesAFileExactlyAtTheLimit_issue609(t *testing.T) {
	edge := writeLines(t, "edge.go", 500)

	out, err := runFileLength(t, repoRoot(t), "500", edge)

	if err != nil {
		t.Fatalf("a 500-line file failed the 500-line gate: %v\n%s", err, out)
	}
}

// A limit that is not a number made every `[ -gt ]` an error inside an if,
// which set -e does not catch, so every file compared as "not over" and the
// gate exited 0: one typo in ci.yml and it is green forever.
func TestFileLengthGate_RefusesALimitThatIsNotANumber_issue609(t *testing.T) {
	long := writeLines(t, "long.go", 501)

	out, err := runFileLength(t, repoRoot(t), "5OO", long)

	if err == nil {
		t.Fatalf("the gate passed with a limit of 5OO:\n%s", out)
	}
	if !strings.Contains(out, "is not a number") {
		t.Errorf("the gate failed without naming the bad limit:\n%s", out)
	}
}

// Outside a checkout git ls-files lists nothing, and a gate that measured
// nothing would report green. It must say it had nothing to measure instead.
func TestFileLengthGate_RefusesAnEmptyFileList_issue609(t *testing.T) {
	out, err := runFileLength(t, t.TempDir())

	if err == nil {
		t.Fatalf("the gate passed with no files to measure:\n%s", out)
	}
	// Failing is not enough: it must fail because it refused, not because a
	// later step tripped over an empty name.
	if !strings.Contains(out, "no .go files to measure") {
		t.Errorf("the gate failed without saying it had nothing to measure:\n%s", out)
	}
}

// The repository itself must pass, so a local `go test ./...` catches a
// seventh long file without anyone remembering the script exists. The
// longest file is printed on every run, clean or not, the way check-deps
// prints its tightest margin: a gate that only says "clean" gives no warning
// before it breaks.
func TestFileLengthGate_IsGreen(t *testing.T) {
	out, err := runFileLength(t, repoRoot(t))

	if err != nil {
		t.Fatalf("check-file-length.sh failed on the repository:\n%s", out)
	}
	if !strings.Contains(out, "longest:") {
		t.Errorf("output does not report the longest file:\n%s", out)
	}
}

// The step costs a second and the test suite minutes, so a long file should
// be reported first.
func TestCI_RunsTheFileLengthGateBeforeTheTests_issue609(t *testing.T) {
	workflow := ciWorkflow(t)

	gate := strings.Index(workflow, "./scripts/check-file-length.sh 500")
	tests := strings.Index(workflow, "go test ./... -race")
	if gate < 0 || tests < 0 {
		t.Fatalf("ci.yml is missing a step: file length at %d, tests at %d", gate, tests)
	}
	if tests < gate {
		t.Error("ci.yml runs the test suite before the file-length gate")
	}
}

// AGENTS.md's gate list is what a contributor runs before claiming a change
// is ready; a step CI runs and the list omits fails only on the pull request.
func TestAgentsGateList_IncludesTheFileLengthGate_issue609(t *testing.T) {
	agents, err := os.ReadFile(filepath.Join(repoRoot(t), "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), "./scripts/check-file-length.sh 500") {
		t.Error("AGENTS.md's gate list does not include the file-length gate")
	}
}
