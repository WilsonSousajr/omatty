package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// Invariant 9: every session row written before #46 has no agent, and it
// still relaunches.
func TestLookup_AnEmptyNameIsClaude_issue46(t *testing.T) {
	p, err := lookupAgent("")
	if err != nil || p.Name != "claude" {
		t.Fatalf("lookupAgent(\"\") = %q, %v; want claude", p.Name, err)
	}
	if byName, _ := lookupAgent("claude"); byName.Name != p.Name {
		t.Errorf("Lookup(claude) = %q, want the same profile", byName.Name)
	}
}

func TestLookup_UnknownAgentNamesItAndTheKnownOnes_issue46(t *testing.T) {
	_, err := lookupAgent("codex")
	if err == nil || !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), "claude") {
		t.Fatalf("Lookup(codex) error = %v, want it to name codex and the known profiles", err)
	}
}

// A nil field would panic at session start, not at build.
func TestClaude_EveryProfileFieldIsSet_issue46(t *testing.T) {
	p := claudeProfile()
	if p.Command == nil || p.TranscriptPath == nil || p.HookEvents == nil || p.RenderSettings == nil || p.Status == nil || p.DefaultBin == "" {
		t.Errorf("Claude() has a nil field: %+v", p)
	}
	if len(agentNames()) != 1 || agentNames()[0] != "claude" {
		t.Errorf("Names() = %v, want [claude]", agentNames())
	}
}

// Pins the seam against the function it replaced.
func TestClaude_TranscriptPathMatchesPathsTranscript_issue46(t *testing.T) {
	if got, want := claudeProfile().TranscriptPath("/h", "/p/x", "id"), paths.Transcript("/h", "/p/x", "id"); got != want {
		t.Errorf("TranscriptPath = %q, want %q", got, want)
	}
}

// One real line through the adapter proves the delegation is wired rather
// than merely declared.
func TestClaude_StatusDerivesAPromptFromATypedLine_issue46(t *testing.T) {
	s := claudeProfile().Status
	e, ok := s.ParseEntry([]byte(`{"type":"user","timestamp":"2026-09-02T12:00:00Z","message":{"role":"user","content":"hi"}}`))
	if !ok || !e.UserIsPrompt {
		t.Fatalf("ParseEntry = %+v ok=%v, want a typed prompt", e, ok)
	}
	if k, _, ok := s.DeriveKind([]status.Entry{e}); !ok || k != status.PromptSubmitted {
		t.Errorf("DeriveKind = %v ok=%v, want PromptSubmitted", k, ok)
	}
	if k, ok := s.KindOf(dstatus.HookPayload{SessionID: "s", HookEventName: "Stop"}); !ok || k != status.TurnEnded {
		t.Errorf("KindOf(Stop) = %v ok=%v, want TurnEnded", k, ok)
	}
	if len(claudeProfile().HookEvents()) != len(status.HookEventNames()) {
		t.Error("HookEvents is not the listener's own list (#78)")
	}
}

// symlinkedDir makes root/real/proj and returns root/link/proj, which reaches
// it through a symlink, together with the path claude would report for it:
// the kernel's, with every link resolved. t.TempDir is itself behind one on
// macOS (/var is /private/var), so the real path is resolved too.
func symlinkedDir(t *testing.T) (dir, physical string) {
	t.Helper()
	root := t.TempDir()
	target := filepath.Join(root, "real", "proj")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "link", "proj"), physical
}

// Regression, issue #564: claude names its transcript directory after its
// working directory as the kernel reports it, with symlinks resolved - a
// project registered as /tmp/x writes under -private-tmp-x on macOS. The
// profile slugged the path as registered, so the tailer never found the
// file and a crash restart used --session-id where it had to resume.
func TestClaude_TranscriptPathFollowsASymlinkedDir_issue564(t *testing.T) {
	dir, physical := symlinkedDir(t)
	got := claudeProfile().TranscriptPath("/h", dir, "id")
	if want := paths.Transcript("/h", physical, "id"); got != want {
		t.Errorf("TranscriptPath(%q) = %q, want the resolved directory's %q", dir, got, want)
	}
}

// Regression, issue #564: a session whose directory is reached through a
// symlink has its transcript under the resolved directory's slug, because
// that is where claude writes it. Resuming it is the #36 condition, so
// missing the file restarted it with --session-id, which claude refuses.
// Here since 5.2b (#653): it drives the launcher with the catalog's own
// profile, which only cmd composes.
func TestLauncher_ResumesASessionBehindASymlink_issue564(t *testing.T) {
	home := t.TempDir()
	dir, physical := symlinkedDir(t)
	transcript := paths.Transcript(home, physical, "abc-123")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := sessions.NewLauncher(claudeProfile(), "claude", "/h.json", home, &detach.Plain{})

	cmd, err := l.Launch(sessions.Session{ID: "abc-123", Dir: dir})
	if err != nil {
		t.Fatalf("Command error = %v, want nil", err)
	}
	if args := strings.Join(cmd.Argv, " "); !strings.Contains(args, "--resume abc-123") {
		t.Errorf("args %q lack --resume for a session whose transcript is under its resolved directory", args)
	}
}
