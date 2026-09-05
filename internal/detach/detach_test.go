package detach_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/detach"
)

// claudeCommand is the command the supervisor hands the holder: exactly what
// omatty runs today, before any detach layer touches it.
func claudeCommand() *exec.Cmd {
	cmd := exec.Command("claude", "--resume", "abc-123", "--settings", "/home/u/.omatty/hooks.json")
	cmd.Dir = "/w/parser-fix"
	return cmd
}

func TestPlain_WrapReturnsTheCommandUnchanged(t *testing.T) {
	in := claudeCommand()

	out, err := (&detach.Plain{}).Wrap("abc-123", in)

	if err != nil {
		t.Fatalf("Plain.Wrap() error = %v, want nil", err)
	}
	if out != in {
		t.Errorf("Plain.Wrap() = %v, want the very command it was given", out.Args)
	}
}

func TestPlain_StopsNothingAndDoesNotPersist(t *testing.T) {
	p := &detach.Plain{}

	if err := p.Stop("abc-123"); err != nil {
		t.Errorf("Plain.Stop() = %v, want nil: there is no held process to stop", err)
	}
	if p.Persists() {
		t.Error("Plain.Persists() = true, want false")
	}
}

func TestDtach_WrapBuildsTheAttachOrCreateLine_issue43(t *testing.T) {
	d := detach.NewDtach("/home/u", "dtach")

	out, err := d.Wrap("abc-123", claudeCommand())

	if err != nil {
		t.Fatalf("Dtach.Wrap() error = %v, want nil", err)
	}
	line := strings.Join(out.Args, " ")
	for _, want := range []string{
		"dtach -A",
		"/home/u/.omatty/s/abc-123.sock",
		"-r winch",
		"/home/u/.omatty/s/abc-123.pid",
		"claude --resume abc-123 --settings /home/u/.omatty/hooks.json",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("dtach line %q is missing %q", line, want)
		}
	}
}

// Invariant 1: every keystroke reaches Claude except omatty's own leader. dtach
// binds ctrl+\ to detach and ctrl+z to suspend, so without -E and -z it would
// silently steal two keys from Claude - and nothing on screen would say so.
func TestDtach_DisablesItsOwnDetachAndSuspendKeys_invariant1(t *testing.T) {
	out, err := detach.NewDtach("/home/u", "dtach").Wrap("abc-123", claudeCommand())

	if err != nil {
		t.Fatal(err)
	}
	for _, flag := range []string{"-E", "-z"} {
		if !hasArg(out.Args, flag) {
			t.Errorf("dtach args %v omit %s; invariant 1 requires every key reach Claude", out.Args, flag)
		}
	}
}

// The session still has to start in its own worktree, so the wrapped command
// keeps the directory the supervisor set (#43).
func TestDtach_WrapKeepsTheWorkingDirectory(t *testing.T) {
	out, err := detach.NewDtach("/home/u", "dtach").Wrap("abc-123", claudeCommand())

	if err != nil {
		t.Fatal(err)
	}
	if out.Dir != "/w/parser-fix" {
		t.Errorf("Dir = %q, want %q", out.Dir, "/w/parser-fix")
	}
}

// The pid wrapper is what makes archiving able to stop a claude: dtach exposes
// neither its own pid nor its child's. exec replaces the shell, so the $$ it
// wrote is claude's own pid rather than a shell that is already gone (#43).
func TestDtach_WrapRecordsClaudesPidThroughAnExecWrapper_issue43(t *testing.T) {
	out, err := detach.NewDtach("/home/u", "dtach").Wrap("abc-123", claudeCommand())

	if err != nil {
		t.Fatal(err)
	}
	line := strings.Join(out.Args, " ")
	if !strings.Contains(line, "echo $$") || !strings.Contains(line, `exec "$@"`) {
		t.Errorf("dtach line %q does not record the pid and exec claude in its place", line)
	}
}

func TestDtach_Persists(t *testing.T) {
	if !detach.NewDtach("/home/u", "dtach").Persists() {
		t.Error("Dtach.Persists() = false, want true")
	}
}

// An unusable socket path must surface here rather than as a dtach failure the
// operator cannot read.
func TestDtach_WrapSurfacesAnOverLongSocketPath_issue43(t *testing.T) {
	deep := "/" + strings.Repeat("d", 200)

	_, err := detach.NewDtach(deep, "dtach").Wrap("abc-123", claudeCommand())

	if err == nil {
		t.Fatal("Dtach.Wrap() with a 200-character home returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "104") {
		t.Errorf("error %q does not name the byte limit", err)
	}
}

// New picks the implementation by whether dtach is on PATH. A name that cannot
// exist stands in for "not installed", so the test does not depend on the
// machine it runs on.
func TestNew_FallsBackToPlainWhenTheBinaryIsMissing_issue43(t *testing.T) {
	h := detach.NewFor("/home/u", "omatty-no-such-dtach-binary")

	if h.Persists() {
		t.Error("NewFor() with a missing binary returned a persisting holder, want the Plain fallback")
	}
}

func TestNewFor_UsesDtachWhenTheBinaryIsOnPath_issue43(t *testing.T) {
	// "sh" stands in for dtach: the point under test is the PATH lookup, not
	// the program. Every machine running these tests has a shell.
	h := detach.NewFor("/home/u", "sh")

	if !h.Persists() {
		t.Error("NewFor() with a binary on PATH returned the Plain fallback, want Dtach")
	}
}

func hasArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
