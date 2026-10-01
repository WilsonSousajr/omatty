package sessions_test

import (
	"errors"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/infra/hooks"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// Invariant 3: --settings points at omatty's own file, so the user's
// ~/.claude/settings.json is never read or written.
func TestLauncher_CommandPassesSessionIDAndOwnSettings(t *testing.T) {
	l := sessions.NewLauncher(claudeProfile(), "claude", "/home/u/.omatty/hooks.json", t.TempDir(), &detach.Plain{})

	cmd, err := l.Launch(session.Session{ID: "abc-123", Dir: "/w/parser-fix"})

	if err != nil {
		t.Fatalf("Launch() error = %v, want nil", err)
	}
	got := strings.Join(cmd.Argv, " ")
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
func commandArgs(t *testing.T, l *sessions.Launcher, sessionID, dir string) string {
	t.Helper()
	cmd, err := l.Launch(session.Session{ID: sessionID, Dir: dir})
	if err != nil {
		t.Fatalf("Launch(%q, %q) error = %v, want nil", sessionID, dir, err)
	}
	return strings.Join(cmd.Argv, " ")
}

// Invariant 3, stated as a property: nothing on the command line points at
// the user's own settings file.
func TestLauncher_CommandNeverReferencesTheUserSettings(t *testing.T) {
	l := sessions.NewLauncher(claudeProfile(), "claude", "/home/u/.omatty/hooks.json", t.TempDir(), &detach.Plain{})

	for _, arg := range strings.Fields(commandArgs(t, l, "abc-123", "/w")) {
		if strings.Contains(arg, ".claude/settings") {
			t.Errorf("argument %q points at the user's settings; invariant 3 forbids it", arg)
		}
	}
}

// Regression, issue #36: claude refuses `--session-id <uuid>` once a transcript
// for that id exists ("Session ID ... is already in use"), so every session the
// operator had typed into died on relaunch. There is no lock file - the
// transcript is the claim - and `--resume` is the documented way back in.
func TestLauncher_UsesSessionIDForAFreshSession_issue36(t *testing.T) {
	home := t.TempDir()
	l := sessions.NewLauncher(claudeProfile(), "claude", "/h.json", home, &detach.Plain{})

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
	l := sessions.NewLauncher(claudeProfile(), "claude", "/h.json", home, &detach.Plain{})

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
		args := commandArgs(t, sessions.NewLauncher(claudeProfile(), "claude", "/h.json", home, &detach.Plain{}), "abc-123", "/w")
		if !strings.Contains(args, "--settings /h.json") {
			t.Errorf("transcript=%v: args %q lack --settings", withTranscript, args)
		}
	}
}

func TestHasTranscript_issue36(t *testing.T) {
	home := t.TempDir()
	if sessions.HasTranscript(claudeProfile(), home, "/w", "none") {
		t.Error("HasTranscript() = true for a session that has never spoken")
	}
	p := paths.Transcript(home, "/w", "spoke")
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	_ = os.WriteFile(p, []byte("{}\n"), 0o600)
	if !sessions.HasTranscript(claudeProfile(), home, "/w", "spoke") {
		t.Error("HasTranscript() = false for a session with a transcript on disk")
	}
	// A directory at the path is not a transcript.
	_ = os.MkdirAll(paths.Transcript(home, "/w", "dir"), 0o700)
	if sessions.HasTranscript(claudeProfile(), home, "/w", "dir") {
		t.Error("HasTranscript() = true for a directory")
	}
}

// The launcher no longer starts claude directly: it starts whatever the holder
// says to, which under dtach is a client attaching to a master that outlives
// omatty. The claude command itself is unchanged and is what the holder is
// handed, so the --session-id / --resume decision above is untouched (#43).
func TestLauncher_CommandWrapsThroughTheHolder_issue43(t *testing.T) {
	h := &fakeHolder{Wrapped: []string{"dtach", "-A", "/s.sock"}}
	l := sessions.NewLauncher(claudeProfile(), "claude", "/h.json", t.TempDir(), h)

	cmd, err := l.Launch(session.Session{ID: "abc-123", Dir: "/w/parser-fix"})

	if err != nil {
		t.Fatalf("Launch() error = %v, want nil", err)
	}
	if h.GotID != "abc-123" {
		t.Errorf("holder was given id %q, want %q", h.GotID, "abc-123")
	}
	if got := strings.Join(h.GotArgs, " "); !strings.Contains(got, "claude --session-id abc-123") {
		t.Errorf("holder was handed %q, want the unwrapped claude command", got)
	}
	if cmd.Argv[0] != "dtach" {
		t.Errorf("Launch() = %v, want the command the holder returned", cmd.Argv)
	}
}

// An unusable socket path must stop the session from starting and say so,
// rather than launching a claude the holder cannot later stop (#43).
func TestLauncher_CommandSurfacesAHolderFailure_issue43(t *testing.T) {
	h := &fakeHolder{WrapErr: errors.New("socket path is 130 bytes, over the 104-byte limit")}
	l := sessions.NewLauncher(claudeProfile(), "claude", "/h.json", t.TempDir(), h)

	_, err := l.Launch(session.Session{ID: "abc-123", Dir: "/w"})

	if err == nil {
		t.Fatal("Launch() returned nil after the holder failed, want an error")
	}
	if !strings.Contains(err.Error(), "104") {
		t.Errorf("error %q does not carry the holder's reason", err)
	}
}

// fakeProfile is a Profile whose command template and transcript location
// are nothing like claude's, so a launcher still carrying claude's flags or
// claude's path fails visibly (#46).
func fakeProfile(home string) agent.Profile {
	return agent.Profile{
		Name: "other", DefaultBin: "other-agent",
		Command: func(bin, sessionID, _ string, resume bool, _ string) []string {
			if resume {
				return []string{bin, "--resumed", sessionID}
			}
			return []string{bin, "--go", sessionID}
		},
		TranscriptPath: func(_, _, id string) string { return filepath.Join(home, "other", id+".log") },
		HookEvents:     func() []string { return []string{"Stop"} },
		RenderSettings: hooks.Render,
		Status:         status.ClaudeAdapter(),
	}
}

// The test that proves the hardcoding is gone: the launcher builds exactly
// the profile's template and adds nothing of its own.
func TestLauncher_UsesTheProfilesCommandTemplate_issue46(t *testing.T) {
	l := sessions.NewLauncher(fakeProfile(t.TempDir()), "other-agent", "/h.json", "/home", &detach.Plain{})
	if got := commandArgs(t, l, "abc", "/w"); got != "other-agent --go abc" {
		t.Errorf("args = %q, want exactly the profile's template", got)
	}
}

func TestLauncher_ResumesWhenTheProfilesTranscriptExists_issue46(t *testing.T) {
	home := t.TempDir()
	p := fakeProfile(home)
	if err := os.MkdirAll(filepath.Dir(p.TranscriptPath(home, "/w", "abc")), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p.TranscriptPath(home, "/w", "abc"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	l := sessions.NewLauncher(p, "other-agent", "/h.json", home, &detach.Plain{})
	if got := commandArgs(t, l, "abc", "/w"); got != "other-agent --resumed abc" {
		t.Errorf("args = %q, want the profile's resume form once its transcript exists", got)
	}
}

// Reattaching is the holder's answer, surfaced so the boot path can tell a
// pane that needs a repaint nudge from one that will paint itself (#191).
func TestLauncher_ReattachingAsksTheHolder_issue191(t *testing.T) {
	h := &fakeHolder{HeldIDs: map[string]bool{"abc-123": true}}
	l := sessions.NewLauncher(claudeProfile(), "claude", "/h.json", t.TempDir(), h)

	held, err := l.Reattaching("abc-123")
	if err != nil || !held {
		t.Errorf("Reattaching(abc-123) = %v, %v; want true, nil", held, err)
	}
	held, err = l.Reattaching("other")
	if err != nil || held {
		t.Errorf("Reattaching(other) = %v, %v; want false, nil", held, err)
	}
}
