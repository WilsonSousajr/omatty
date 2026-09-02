package supervisor_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// Invariant 3: --settings points at omatty's own file, so the user's
// ~/.claude/settings.json is never read or written.
func TestLauncher_CommandPassesSessionIDAndOwnSettings(t *testing.T) {
	l := supervisor.NewLauncher("claude", "/home/u/.omatty/hooks.json", t.TempDir())

	cmd := l.Command("abc-123", "/w/parser-fix")

	got := strings.Join(cmd.Args, " ")
	want := "claude --session-id abc-123 --settings /home/u/.omatty/hooks.json"
	if got != want {
		t.Errorf("Args = %q, want %q", got, want)
	}
	if cmd.Dir != "/w/parser-fix" {
		t.Errorf("Dir = %q, want %q", cmd.Dir, "/w/parser-fix")
	}
}

// Invariant 3, stated as a property: nothing on the command line points at
// the user's own settings file.
func TestLauncher_CommandNeverReferencesTheUserSettings(t *testing.T) {
	cmd := supervisor.NewLauncher("claude", "/home/u/.omatty/hooks.json", t.TempDir()).
		Command("abc-123", "/w")

	for _, arg := range cmd.Args {
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

	term, err := supervisor.NewLauncher("claude", "/h.json", t.TempDir()).Start(factory, sess, 80, 24)

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

	_, err := supervisor.NewLauncher("claude", "/h.json", t.TempDir()).
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
	l := supervisor.NewLauncher(bin, "/h.json", t.TempDir())
	sess := registry.Session{ID: "smoke-uuid", Dir: t.TempDir()}

	term, err := l.Start(termwrap.Start, sess, 60, 12)
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	defer func() { _ = term.Close() }()

	if got := strings.Join(l.Command(sess.ID, sess.Dir).Args, " "); !strings.Contains(got, "smoke-uuid") {
		t.Errorf("command %q does not carry the session id", got)
	}
}

// Regression, issue #36: claude refuses `--session-id <uuid>` once a transcript
// for that id exists ("Session ID ... is already in use"), so every session the
// operator had typed into died on relaunch. There is no lock file - the
// transcript is the claim - and `--resume` is the documented way back in.
func TestLauncher_UsesSessionIDForAFreshSession_issue36(t *testing.T) {
	home := t.TempDir()
	l := supervisor.NewLauncher("claude", "/h.json", home)

	args := strings.Join(l.Command("abc-123", "/w/parser-fix").Args, " ")

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
	l := supervisor.NewLauncher("claude", "/h.json", home)

	args := strings.Join(l.Command("abc-123", "/w/parser-fix").Args, " ")

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
		args := strings.Join(supervisor.NewLauncher("claude", "/h.json", home).Command("abc-123", "/w").Args, " ")
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
