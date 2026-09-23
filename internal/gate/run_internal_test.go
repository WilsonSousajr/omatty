package gate

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// leadingWord decides whether a step gets the LookPath pre-flight at all.
// It is deliberately conservative: every case it declines to read is run, so
// the cost of a wrong "" is a step that executes normally, while the cost of a
// wrong name is a real failure hidden behind a bogus Missing.
func TestLeadingWord(t *testing.T) {
	cases := []struct {
		name string
		run  string
		want string
	}{
		{"a plain command", "golangci-lint run", "golangci-lint"},
		{"a path", "./scripts/check-coverage.sh 90", "./scripts/check-coverage.sh"},
		{"no arguments", "true", "true"},
		{"leading whitespace", "   gofmt -l .", "gofmt"},
		{"an env assignment", "CGO_ENABLED=0 go build", ""},
		// leadingWord is purely syntactic since #248: it reads the word, and
		// absentTool asks the shell whether it resolves. A builtin therefore
		// comes back named here and is cleared by the probe, not by a list.
		{"a shell builtin", "cd internal && go test", "cd"},
		{"a subshell", "(go vet ./...)", ""},
		{"a variable", "$LINTER run", ""},
		{"a pipeline that starts with one", "go test | tee out", "go"},
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := leadingWord(c.run); got != c.want {
				t.Errorf("leadingWord(%q) = %q, want %q", c.run, got, c.want)
			}
		})
	}
}

// Invariant 12: the exit status is the whole of the verdict.
func TestClassify(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	t.Run("no error is Pass", func(t *testing.T) {
		if v, code := classify(context.Background(), nil); v != Pass || code != 0 {
			t.Errorf("classify(nil) = %v, %d, want pass, 0", v, code)
		}
	})

	t.Run("an exit status is Fail carrying the code", func(t *testing.T) {
		err := exec.Command("sh", "-c", "exit 7").Run()
		if v, code := classify(context.Background(), err); v != Fail || code != 7 {
			t.Errorf("classify(exit 7) = %v, %d, want fail, 7", v, code)
		}
	})

	// A command that could not start at all is still not a Pass, and it has no
	// exit status to report.
	t.Run("a non-exit error is Fail with no code", func(t *testing.T) {
		if v, code := classify(context.Background(), errors.New("fork/exec: no such file")); v != Fail || code != -1 {
			t.Errorf("classify(start error) = %v, %d, want fail, -1", v, code)
		}
	})

	// A cancelled context outranks whatever the killed child reported: the
	// operator stopped it, so it did not fail.
	t.Run("a cancelled context outranks the child's status", func(t *testing.T) {
		if v, _ := classify(cancelled, errors.New("signal: killed")); v != Cancelled {
			t.Errorf("classify(cancelled) = %v, want cancelled", v)
		}
	})
}

// absentTool must decline to judge anything leadingWord declined to read.
func TestAbsentTool_leavesUnreadableLinesAlone(t *testing.T) {
	if name, absent := absentTool("CGO_ENABLED=0 omatty-no-such-tool-xyz", t.TempDir()); absent {
		t.Errorf("absentTool() = %q, true; an env assignment must be run, not pre-judged", name)
	}
}

// The probe asks the shell, so a builtin with no binary anywhere resolves
// (#248). exec.LookPath answered a different question and got these wrong.
func TestAbsentTool_resolvesBuiltinsThatHaveNoBinary(t *testing.T) {
	dir := t.TempDir()
	for _, builtin := range []string{"exit 1", "return 0", "local x=1", "break", "continue", "readonly x=1"} {
		if name, absent := absentTool(builtin, dir); absent {
			t.Errorf("absentTool(%q) = %q, true; a shell builtin is not an absent tool", builtin, name)
		}
	}
}

// And a tool that really is not there is still caught.
func TestAbsentTool_stillCatchesARealAbsence(t *testing.T) {
	name, absent := absentTool("omatty-no-such-tool-xyz --check", t.TempDir())
	if !absent {
		t.Fatal("absentTool() = false for a tool that does not exist")
	}
	if name != "omatty-no-such-tool-xyz" {
		t.Errorf("absentTool() named %q, want the tool itself", name)
	}
}

// THE regression (#289). A step's command is resolved where the step will run,
// not where omatty happens to have been launched from. gate.Detect proposes
// `./scripts/check-coverage.sh` for any project that ships one, so a probe run
// in the wrong directory makes omatty refuse the gate it just proposed - and
// Missing stops the gate, so every step after it reports Pending and the
// operator sees no coverage, no overlay and no verdict.
//
// The fixture puts the script somewhere the test process's own working
// directory cannot see, which is exactly the operator's case.
func TestAbsentTool_aRelativePathIsResolvedInTheStepsDirectory_issue289(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, filepath.Join(dir, "scripts", "check-coverage.sh"))

	name, absent := absentTool("./scripts/check-coverage.sh 90", dir)

	if absent {
		t.Errorf("absentTool() = %q, true; the script is right there in the step's own directory", name)
	}
}

// The step itself then runs, and passes - the pre-flight and the run agree
// about where they are.
func TestRunStep_aRelativePathStepRuns_issue289(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, filepath.Join(dir, "scripts", "check-coverage.sh"))

	got := runStep(context.Background(), dir, Step{Name: "cov", Run: "./scripts/check-coverage.sh"})

	if got.Verdict != Pass {
		t.Errorf("verdict = %v, want pass; output:\n%s", got.Verdict, got.Output)
	}
}

// The negative control: the fix must not amount to switching the pre-flight
// off. A tool that is genuinely nowhere is still Missing, which is the whole
// point of the pre-flight - telling a session its lint is failing when
// golangci-lint is merely absent sends it off to fix code that was never
// broken.
func TestAbsentTool_aGenuinelyAbsentToolIsStillMissing_issue289(t *testing.T) {
	name, absent := absentTool("omatty-no-such-tool-xyz --check", t.TempDir())

	if !absent || name != "omatty-no-such-tool-xyz" {
		t.Errorf("absentTool() = %q, %v; want the absent tool named", name, absent)
	}
}

// A relative path that is not there either is still Missing: the fix moves
// where the question is asked, not whether it is asked.
func TestAbsentTool_aRelativePathThatIsNotThereIsMissing_issue289(t *testing.T) {
	name, absent := absentTool("./scripts/check-coverage.sh", t.TempDir())

	if !absent || name != "./scripts/check-coverage.sh" {
		t.Errorf("absentTool() = %q, %v; want the missing script named", name, absent)
	}
}

// writeScript puts an executable no-op at path, creating its directory.
func writeScript(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil { //nolint:gosec // a test fixture that must be executable
		t.Fatal(err)
	}
}
