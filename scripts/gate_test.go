// Package scripts_test guards the repository's own tooling. These are
// regression tests for config-level bugs, which are just as capable of
// silently breaking the project as Go code is.
package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// Regression, issue #27: the gate ran `go test -coverpkg=./internal/...`, which
// makes every test binary instrument every package and report zero for the
// ones it does not exercise. Merging those profiles zeroes out the real counts:
// internal/registry measured 91.7% alone and 43% merged. The 90% gate was
// therefore reporting a number that meant nothing.
func TestCoverageGate_DoesNotUseCoverpkg(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "check-coverage.sh"))
	if err != nil {
		t.Fatal(err)
	}
	// Only executable lines count: the script's own comment names the flag to
	// explain why it is not used.
	for i, line := range strings.Split(string(b), "\n") {
		code := strings.TrimSpace(line)
		if code == "" || strings.HasPrefix(code, "#") {
			continue
		}
		if strings.Contains(code, "-coverpkg") {
			t.Errorf("line %d uses -coverpkg: %q\n"+
				"that zeroes counts across test binaries and makes the reported "+
				"coverage meaningless", i+1, code)
		}
	}
}

// The gate must also not hide the test run: swallowing output meant a failing
// or cached run still produced a coverage verdict.
func TestCoverageGate_DoesNotSwallowTestOutput(t *testing.T) {
	b, err := os.ReadFile(filepath.Join(repoRoot(t), "scripts", "check-coverage.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "go test") &&
			strings.Contains(line, "/dev/null") {
			t.Errorf("the go test line discards its output: %q", strings.TrimSpace(line))
		}
	}
}

// The threshold must actually be compared, not just printed.
func TestCoverageGate_FailsOnAnUnreachableThreshold(t *testing.T) {
	if testing.Short() {
		t.Skip("runs the full test suite")
	}
	cmd := exec.Command("./scripts/check-coverage.sh", "101")
	cmd.Dir = repoRoot(t)

	out, err := cmd.CombinedOutput()

	if err == nil {
		t.Fatalf("gate passed at a 101%% threshold, want failure:\n%s", out)
	}
	if !strings.Contains(string(out), "below") {
		t.Errorf("output does not explain the failure:\n%s", out)
	}
}

// Regression, issue #27: .gitignore listed a bare `omatty` to ignore the built
// binary. That pattern matches any path segment, so it also matched
// cmd/omatty/, which would have silently excluded the entire binary package
// from the repository.
func TestGitignore_DoesNotExcludeTheCommandPackage(t *testing.T) {
	for _, path := range []string{"cmd/omatty/main.go", "cmd/omatty"} {
		t.Run(path, func(t *testing.T) {
			cmd := exec.Command("git", "check-ignore", "-q", path)
			cmd.Dir = repoRoot(t)
			// check-ignore exits 0 when the path IS ignored.
			if err := cmd.Run(); err == nil {
				t.Errorf("%q is gitignored; the binary package would never be committed", path)
			}
		})
	}
}

// The built binary at the repo root must still be ignored.
func TestGitignore_StillIgnoresTheBuiltBinary(t *testing.T) {
	cmd := exec.Command("git", "check-ignore", "-q", "omatty")
	cmd.Dir = repoRoot(t)
	if err := cmd.Run(); err != nil {
		t.Error("the built binary at the repo root is not ignored")
	}
}
