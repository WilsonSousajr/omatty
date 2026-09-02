package supervisor_test

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// Invariant 3: --settings points at omatty's own file, so the user's
// ~/.claude/settings.json is never read or written.
func TestLauncher_CommandPassesSessionIDAndOwnSettings(t *testing.T) {
	l := supervisor.NewLauncher("claude", "/home/u/.omatty/hooks.json")

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
	cmd := supervisor.NewLauncher("claude", "/home/u/.omatty/hooks.json").
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

	term, err := supervisor.NewLauncher("claude", "/h.json").Start(factory, sess, 80, 24)

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

	_, err := supervisor.NewLauncher("claude", "/h.json").
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
	l := supervisor.NewLauncher(bin, "/h.json")
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
