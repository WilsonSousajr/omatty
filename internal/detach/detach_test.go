package detach_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/detach"
	"github.com/WilsonSousajr/omatty/internal/paths"
)

// claudeCommand is the command the supervisor hands the holder: exactly what
// omatty runs today, before any detach layer touches it.
//
// Built by hand rather than with exec.Command, which resolves the binary on
// PATH and records the failure in cmd.Err when it cannot find one. Wrap now
// refuses such a command, which is the point of the fix - so exec.Command here
// made every test below pass or fail on whether claude happened to be installed
// on the machine. It is on the author's and not on CI, so this shipped green
// locally and red on ubuntu. Path is what a successful lookup would have left
// behind; Args is what Wrap actually reads (#43).
func claudeCommand() *exec.Cmd {
	return &exec.Cmd{
		Path: "/usr/local/bin/claude",
		Args: []string{"claude", "--resume", "abc-123", "--settings", "/home/u/.omatty/hooks.json"},
		Dir:  "/w/parser-fix",
	}
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
	home := t.TempDir()
	// Built here rather than through detach.SocketPath: that helper enforces
	// the real limit, which t.TempDir() is past, and this test is about the
	// command line rather than the limit.
	sock := filepath.Join(paths.SessionDir(home), "abc-123.sock")

	out, err := testDtach(home).Wrap("abc-123", claudeCommand())

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
	out, err := testDtach(t.TempDir()).Wrap("abc-123", claudeCommand())

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
	out, err := testDtach(t.TempDir()).Wrap("abc-123", claudeCommand())

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
	out, err := testDtach(t.TempDir()).Wrap("abc-123", claudeCommand())

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
	if !strings.Contains(err.Error(), "103") {
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
	home := t.TempDir() // no .omatty/s inside it, as on a fresh machine

	if _, err := testDtach(home).Wrap("abc-123", claudeCommand()); err != nil {
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
	home := t.TempDir()

	if _, err := testDtach(home).Wrap("abc-123", claudeCommand()); err != nil {
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

// testDtach is a holder whose socket-path limit is out of the way, so a test
// about wrapping or stopping is not accidentally a test of the limit.
//
// It exists because t.TempDir() on macOS sits under /var/folders/<random>,
// itself long enough that adding "/.omatty/s/<uuid>.sock" trips the real cap -
// a property of the test environment, not of omatty. The previous answer was to
// mkdir under /tmp directly, which ignored TMPDIR, broke anywhere /tmp is
// absent or unwritable, and left directories behind when the binary was killed.
// Naming the limit is the fix at the right depth: every case below uses
// t.TempDir(), and the limit keeps its own tests in paths_test.go (#43).
func testDtach(home string) *detach.Dtach {
	return detach.NewDtachCapped(home, "dtach", 4096)
}

// Regression, issue #43: Wrap rebuilt the command around dtach and copied only
// Dir and Env, dropping cmd.Err - where exec.Command records a binary it could
// not resolve. Under Plain that field reaches Start and names the problem;
// under Dtach the launch "succeeded", dtach started, and the pane flashed
// "sh: claude: not found" before dying, with nothing in the log.
func TestDtach_WrapRefusesACommandThatCouldNotBeResolved_issue43(t *testing.T) {
	missing := exec.Command("omatty-no-such-claude-binary")

	_, err := testDtach(t.TempDir()).Wrap("abc-123", missing)

	if err == nil {
		t.Fatal("Dtach.Wrap() with an unresolvable binary returned nil, want the error exec.Command recorded")
	}
	if !strings.Contains(err.Error(), "abc-123") {
		t.Errorf("error %q does not name the session", err)
	}
}

// A directory that already exists keeps its mode through MkdirAll, so a
// ~/.omatty/s left at 0755 by an earlier build or a restored backup stayed
// world-readable and the 0700 in the fresh-creation path was a claim rather
// than a guarantee. The socket in it is a control channel into a running
// claude: anyone who can connect can type into that session.
func TestDtach_WrapTightensASessionDirectoryThatIsAlreadyTooOpen_issue43(t *testing.T) {
	home := t.TempDir()
	dir := paths.SessionDir(home)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o755); err != nil { // MkdirAll applies the umask
		t.Fatal(err)
	}

	if _, err := testDtach(home).Wrap("abc-123", claudeCommand()); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("session directory mode = %04o, want 0700: an existing directory is tightened, not accepted", perm)
	}
}

// Without the `|| exit 1` a failed redirection - a full or read-only home -
// left sh free to exec claude anyway, and that session was one Stop could never
// end: no pidfile, so archiving was a silent no-op and the claude went on
// running behind a socket with no row in state.json (#43).
func TestDtach_WrapFailsTheLaunchWhenThePidCannotBeRecorded_issue43(t *testing.T) {
	out, err := testDtach(t.TempDir()).Wrap("abc-123", claudeCommand())

	if err != nil {
		t.Fatal(err)
	}
	line := strings.Join(out.Args, " ")
	if !strings.Contains(line, "|| exit 1") {
		t.Errorf("dtach line %q execs claude even when the pid write fails, leaving a session Stop cannot end", line)
	}
}

// Without dtach omatty still runs, so this is a notice rather than a failure -
// but it has to name the fix. "sessions will not survive quit" alone leaves an
// operator who has never heard of dtach with nothing to do about it (#43).
func TestPlain_NoticeNamesTheFixForThisPlatform_issue43(t *testing.T) {
	got := (&detach.Plain{}).Notice()

	if !strings.Contains(got, "install dtach") {
		t.Errorf("notice = %q, want it to name the command that fixes it", got)
	}
	if !strings.Contains(got, "quit") {
		t.Errorf("notice = %q, want it to say what is lost", got)
	}
	// The notice shares the footer line with the exit key, which the footer
	// const guarantees stays on screen. At the default 80 columns there is no
	// room for a longer one (#28, #43).
	if len(got) > 62 {
		t.Errorf("notice is %d characters; longer than 62 pushes ctrl+o q off an 80-column footer", len(got))
	}
}

// With a holder there is nothing to say, and a permanent line saying so would
// cost the keymap its place in the footer for no reason.
func TestDtach_NoticeIsSilentWhenSessionsPersist_issue43(t *testing.T) {
	if got := testDtach(t.TempDir()).Notice(); got != "" {
		t.Errorf("notice = %q, want empty when sessions already survive quit", got)
	}
}
