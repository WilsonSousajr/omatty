package gate

import (
	"context"
	"errors"
	"os/exec"
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
		{"a shell builtin", "cd internal && go test", ""},
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
	if name, absent := absentTool("CGO_ENABLED=0 omatty-no-such-tool-xyz"); absent {
		t.Errorf("absentTool() = %q, true; an env assignment must be run, not pre-judged", name)
	}
}
