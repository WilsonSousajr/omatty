package detach_test

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/detach"
	"github.com/WilsonSousajr/omatty/internal/paths"
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
	home := shortTempHome(t)
	sock, err := detach.SocketPath(home, "abc-123")
	if err != nil {
		t.Fatal(err)
	}

	out, err := detach.NewDtach(home, "dtach").Wrap("abc-123", claudeCommand())

	if err != nil {
		t.Fatalf("Dtach.Wrap() error = %v, want nil", err)
	}
	line := strings.Join(out.Args, " ")
	for _, want := range []string{
		"dtach -A",
		sock,
		"-r winch",
		detach.PidPath(home, "abc-123"),
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
	out, err := detach.NewDtach(shortTempHome(t), "dtach").Wrap("abc-123", claudeCommand())

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
	out, err := detach.NewDtach(shortTempHome(t), "dtach").Wrap("abc-123", claudeCommand())

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
	out, err := detach.NewDtach(shortTempHome(t), "dtach").Wrap("abc-123", claudeCommand())

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

// Regression, issue #43: nothing created ~/.omatty/s, so dtach exited 1 with
// "No such file or directory" and every session failed to start on a machine
// that had never run this before. Every unit test passed, because they all
// assert the command line rather than run it; the smoke test found it.
func TestDtach_WrapCreatesTheSessionDirectory_issue43(t *testing.T) {
	home := shortTempHome(t) // no .omatty/s inside it, as on a fresh machine

	if _, err := detach.NewDtach(home, "dtach").Wrap("abc-123", claudeCommand()); err != nil {
		t.Fatalf("Wrap() error = %v, want nil", err)
	}

	info, err := os.Stat(paths.SessionDir(home))
	if err != nil {
		t.Fatalf("session directory not created: %v; dtach cannot bind a socket in it", err)
	}
	if !info.IsDir() {
		t.Errorf("session path is a %v, want a directory", info.Mode().Type())
	}
}

// The socket lives beside the pidfile and both are omatty's own, so the
// directory is private: 0700, like every other directory omatty creates.
func TestDtach_WrapCreatesThatDirectoryPrivate_issue43(t *testing.T) {
	home := shortTempHome(t)

	if _, err := detach.NewDtach(home, "dtach").Wrap("abc-123", claudeCommand()); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(paths.SessionDir(home))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("session directory mode = %04o, want 0700", perm)
	}
}

// shortTempHome is a temporary home short enough for a session socket to fit
// under the 104-byte limit.
//
// t.TempDir cannot be used here: on macOS it sits under /var/folders/<random>,
// which is itself long enough that adding ".omatty/s/<uuid>.sock" trips the
// very guard these tests are not about. A real home is short, so this is a
// property of the test environment rather than of omatty (#43).
func shortTempHome(t *testing.T) string {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "om")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	return home
}
