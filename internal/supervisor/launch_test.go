package supervisor_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/detach"
	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// Invariant 3: --settings points at omatty's own file, so the user's
// ~/.claude/settings.json is never read or written.
func TestLauncher_CommandPassesSessionIDAndOwnSettings(t *testing.T) {
	l := supervisor.NewLauncher("claude", "/home/u/.omatty/hooks.json", t.TempDir(), &detach.Plain{})

	cmd, err := l.Command("abc-123", "/w/parser-fix")

	if err != nil {
		t.Fatalf("Command() error = %v, want nil", err)
	}
	got := strings.Join(cmd.Args, " ")
	want := "claude --session-id abc-123 --settings /home/u/.omatty/hooks.json"
	if got != want {
		t.Errorf("Args = %q, want %q", got, want)
	}
	if cmd.Dir != "/w/parser-fix" {
		t.Errorf("Dir = %q, want %q", cmd.Dir, "/w/parser-fix")
	}
}

// commandArgs is the launcher's command line as one string. The error is
// unwrapped here rather than at each call site: Command grew an error return
// when the holder arrived (#43), and the assertions below are about the
// arguments, not about that.
func commandArgs(t *testing.T, l *supervisor.Launcher, sessionID, dir string) string {
	t.Helper()
	cmd, err := l.Command(sessionID, dir)
	if err != nil {
		t.Fatalf("Command(%q, %q) error = %v, want nil", sessionID, dir, err)
	}
	return strings.Join(cmd.Args, " ")
}

// Invariant 3, stated as a property: nothing on the command line points at
// the user's own settings file.
func TestLauncher_CommandNeverReferencesTheUserSettings(t *testing.T) {
	l := supervisor.NewLauncher("claude", "/home/u/.omatty/hooks.json", t.TempDir(), &detach.Plain{})

	for _, arg := range strings.Fields(commandArgs(t, l, "abc-123", "/w")) {
		if strings.Contains(arg, ".claude/settings") {
			t.Errorf("argument %q points at the user's settings; invariant 3 forbids it", arg)
		}
	}
}

func TestLauncher_StartHandsTheCommandToTheFactory(t *testing.T) {
	var gotW, gotH int
	var gotDir string
	fake := termwrap.NewFake("")
	factory := func(w, h int, cmd *exec.Cmd) (termwrap.Terminal, error) {
		gotW, gotH, gotDir = w, h, cmd.Dir
		return fake, nil
	}
	sess := registry.Session{ID: "abc-123", Dir: "/w/parser-fix"}

	term, err := supervisor.NewLauncher("claude", "/h.json", t.TempDir(), &detach.Plain{}).Start(factory, sess, 80, 24)

	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	if term != fake {
		t.Error("Start() returned a different Terminal than the factory produced")
	}
	if gotW != 80 || gotH != 24 || gotDir != "/w/parser-fix" {
		t.Errorf("factory got (%d, %d, %q), want (80, 24, %q)", gotW, gotH, gotDir, "/w/parser-fix")
	}
}

func TestLauncher_StartFailureNamesTheSession(t *testing.T) {
	factory := func(int, int, *exec.Cmd) (termwrap.Terminal, error) {
		return nil, errors.New("pty exhausted")
	}

	_, err := supervisor.NewLauncher("claude", "/h.json", t.TempDir(), &detach.Plain{}).
		Start(factory, registry.Session{ID: "abc-123", Dir: "/w"}, 80, 24)

	if err == nil {
		t.Fatal("Start() returned nil after a factory failure, want an error")
	}
	if !strings.Contains(err.Error(), "abc-123") {
		t.Errorf("error %q does not name the offending session %q", err, "abc-123")
	}
}

// The fake claude stands in for the real binary everywhere, so it must
// actually launch and echo the session id it was given.
func TestLauncher_StartRunsTheFakeClaude(t *testing.T) {
	// Absolute, because Go resolves a relative cmd.Path against cmd.Dir - the
	// session directory - not against the caller's cwd. A bare "claude" goes
	// through PATH and is unaffected.
	bin, err := filepath.Abs("../../testdata/fake-claude")
	if err != nil {
		t.Fatal(err)
	}
	l := supervisor.NewLauncher(bin, "/h.json", t.TempDir(), &detach.Plain{})
	sess := registry.Session{ID: "smoke-uuid", Dir: t.TempDir()}

	term, err := l.Start(termwrap.Start, sess, 60, 12)
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	defer func() { _ = term.Close() }()

	if got := commandArgs(t, l, sess.ID, sess.Dir); !strings.Contains(got, "smoke-uuid") {
		t.Errorf("command %q does not carry the session id", got)
	}
}

// Regression, issue #36: claude refuses `--session-id <uuid>` once a transcript
// for that id exists ("Session ID ... is already in use"), so every session the
// operator had typed into died on relaunch. There is no lock file - the
// transcript is the claim - and `--resume` is the documented way back in.
func TestLauncher_UsesSessionIDForAFreshSession_issue36(t *testing.T) {
	home := t.TempDir()
	l := supervisor.NewLauncher("claude", "/h.json", home, &detach.Plain{})

	args := commandArgs(t, l, "abc-123", "/w/parser-fix")

	if !strings.Contains(args, "--session-id abc-123") {
		t.Errorf("args %q lack --session-id for a session with no transcript", args)
	}
	if strings.Contains(args, "--resume") {
		t.Errorf("args %q use --resume for a session with no transcript", args)
	}
}

func TestLauncher_UsesResumeWhenTheTranscriptExists_issue36(t *testing.T) {
	home := t.TempDir()
	transcript := paths.Transcript(home, "/w/parser-fix", "abc-123")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := supervisor.NewLauncher("claude", "/h.json", home, &detach.Plain{})

	args := commandArgs(t, l, "abc-123", "/w/parser-fix")

	if !strings.Contains(args, "--resume abc-123") {
		t.Errorf("args %q lack --resume for a session whose transcript exists", args)
	}
	if strings.Contains(args, "--session-id") {
		t.Errorf("args %q use --session-id, which claude refuses once a transcript exists", args)
	}
}

// Invariant 3 on both paths: omatty's own settings file is always passed.
func TestLauncher_SettingsIsPassedOnBothPaths_issue36(t *testing.T) {
	for _, withTranscript := range []bool{false, true} {
		home := t.TempDir()
		if withTranscript {
			p := paths.Transcript(home, "/w", "abc-123")
			_ = os.MkdirAll(filepath.Dir(p), 0o700)
			_ = os.WriteFile(p, []byte("{}\n"), 0o600)
		}
		args := commandArgs(t, supervisor.NewLauncher("claude", "/h.json", home, &detach.Plain{}), "abc-123", "/w")
		if !strings.Contains(args, "--settings /h.json") {
			t.Errorf("transcript=%v: args %q lack --settings", withTranscript, args)
		}
	}
}

func TestHasTranscript_issue36(t *testing.T) {
	home := t.TempDir()
	if supervisor.HasTranscript(home, "/w", "none") {
		t.Error("HasTranscript() = true for a session that has never spoken")
	}
	p := paths.Transcript(home, "/w", "spoke")
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	_ = os.WriteFile(p, []byte("{}\n"), 0o600)
	if !supervisor.HasTranscript(home, "/w", "spoke") {
		t.Error("HasTranscript() = false for a session with a transcript on disk")
	}
	// A directory at the path is not a transcript.
	_ = os.MkdirAll(paths.Transcript(home, "/w", "dir"), 0o700)
	if supervisor.HasTranscript(home, "/w", "dir") {
		t.Error("HasTranscript() = true for a directory")
	}
}

// The launcher no longer starts claude directly: it starts whatever the holder
// says to, which under dtach is a client attaching to a master that outlives
// omatty. The claude command itself is unchanged and is what the holder is
// handed, so the --session-id / --resume decision above is untouched (#43).
func TestLauncher_CommandWrapsThroughTheHolder_issue43(t *testing.T) {
	h := &fakeHolder{Wrapped: exec.Command("dtach", "-A", "/s.sock")}
	l := supervisor.NewLauncher("claude", "/h.json", t.TempDir(), h)

	cmd, err := l.Command("abc-123", "/w/parser-fix")

	if err != nil {
		t.Fatalf("Command() error = %v, want nil", err)
	}
	if h.GotID != "abc-123" {
		t.Errorf("holder was given id %q, want %q", h.GotID, "abc-123")
	}
	if got := strings.Join(h.GotArgs, " "); !strings.Contains(got, "claude --session-id abc-123") {
		t.Errorf("holder was handed %q, want the unwrapped claude command", got)
	}
	if cmd.Args[0] != "dtach" {
		t.Errorf("Command() = %v, want the command the holder returned", cmd.Args)
	}
}

// An unusable socket path must stop the session from starting and say so,
// rather than launching a claude the holder cannot later stop (#43).
func TestLauncher_CommandSurfacesAHolderFailure_issue43(t *testing.T) {
	h := &fakeHolder{WrapErr: errors.New("socket path is 130 bytes, over the 104-byte limit")}
	l := supervisor.NewLauncher("claude", "/h.json", t.TempDir(), h)

	_, err := l.Command("abc-123", "/w")

	if err == nil {
		t.Fatal("Command() returned nil after the holder failed, want an error")
	}
	if !strings.Contains(err.Error(), "104") {
		t.Errorf("error %q does not carry the holder's reason", err)
	}
}

func TestLauncher_StartSurfacesAHolderFailure_issue43(t *testing.T) {
	h := &fakeHolder{WrapErr: errors.New("socket path too long")}
	factory := func(int, int, *exec.Cmd) (termwrap.Terminal, error) {
		t.Error("the factory was called despite the holder failing")
		return nil, nil
	}

	_, err := supervisor.NewLauncher("claude", "/h.json", t.TempDir(), h).
		Start(factory, registry.Session{ID: "abc-123", Dir: "/w"}, 80, 24)

	if err == nil {
		t.Fatal("Start() returned nil after the holder failed, want an error")
	}
}
