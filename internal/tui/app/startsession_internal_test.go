package app

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// Starting a session's terminal was supervisor's Launcher.Start until
// migration step 5.5 (#653) moved the launcher into the session service, which
// may not name a terminal. These are its tests, against startSession.

// startProfile is claude's command template with the transcript slugged as
// given: all starting a terminal reads from a profile.
func startProfile() agent.Profile {
	return agent.Profile{Name: "claude", Command: agent.ClaudeCommand, TranscriptPath: paths.Transcript}
}

// failingHolder is a holder whose Wrap always fails, standing in for an
// unusable socket path (#43).
type failingHolder struct{ err error }

func (h failingHolder) Wrap(string, []string) ([]string, error) { return nil, h.err }
func (failingHolder) Stop(string) error                         { return nil }
func (failingHolder) Persists() bool                            { return true }
func (failingHolder) Held(string) (bool, error)                 { return false, nil }

func TestStartSession_HandsTheLaunchToTheFactory(t *testing.T) {
	var gotW, gotH int
	var gotDir string
	fake := terminal.NewFake("")
	factory := func(w, h int, cmd session.Launch) (terminal.Terminal, error) {
		gotW, gotH, gotDir = w, h, cmd.Dir
		return fake, nil
	}
	sess := session.Session{ID: "abc-123", Dir: "/w/parser-fix"}

	l := sessions.NewLauncher(startProfile(), "claude", "/h.json", t.TempDir(), &detach.Plain{})
	term, err := startSession(l, factory, sess, 80, 24)

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

func TestStartSession_FailureNamesTheSession(t *testing.T) {
	factory := func(int, int, session.Launch) (terminal.Terminal, error) {
		return nil, errors.New("pty exhausted")
	}

	_, err := startSession(sessions.NewLauncher(startProfile(), "claude", "/h.json", t.TempDir(), &detach.Plain{}), factory, session.Session{ID: "abc-123", Dir: "/w"}, 80, 24)

	if err == nil {
		t.Fatal("Start() returned nil after a factory failure, want an error")
	}
	if !strings.Contains(err.Error(), "abc-123") {
		t.Errorf("error %q does not name the offending session %q", err, "abc-123")
	}
}

// The fake claude stands in for the real binary everywhere, so it must
// actually launch and echo the session id it was given.
func TestStartSession_RunsTheFakeClaude(t *testing.T) {
	// Absolute, because Go resolves a relative cmd.Path against cmd.Dir - the
	// session directory - not against the caller's cwd. A bare "claude" goes
	// through PATH and is unaffected.
	bin, err := filepath.Abs("../../../testdata/fake-claude")
	if err != nil {
		t.Fatal(err)
	}
	l := sessions.NewLauncher(startProfile(), bin, "/h.json", t.TempDir(), &detach.Plain{})
	sess := session.Session{ID: "smoke-uuid", Dir: t.TempDir()}

	term, err := startSession(l, terminal.Start, sess, 60, 12)
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	defer func() { _ = term.Close() }()

	launch, err := l.Launch(sess)
	if err != nil || !strings.Contains(strings.Join(launch.Argv, " "), "smoke-uuid") {
		t.Errorf("command %v, %v does not carry the session id", launch.Argv, err)
	}
}

func TestStartSession_SurfacesAHolderFailure_issue43(t *testing.T) {
	h := failingHolder{err: errors.New("socket path too long")}
	factory := func(int, int, session.Launch) (terminal.Terminal, error) {
		t.Error("the factory was called despite the holder failing")
		return nil, nil
	}

	_, err := startSession(sessions.NewLauncher(startProfile(), "claude", "/h.json", t.TempDir(), h), factory, session.Session{ID: "abc-123", Dir: "/w"}, 80, 24)

	if err == nil {
		t.Fatal("Start() returned nil after the holder failed, want an error")
	}
}
