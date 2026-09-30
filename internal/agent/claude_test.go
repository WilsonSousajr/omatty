package agent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/agent"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// The argv assertions #36 and invariant 3 made against the launcher, now
// against the profile that owns the template.
func TestClaude_CommandStartsFreshOrResumes_issue36(t *testing.T) {
	p := agent.Claude()
	fresh := strings.Join(p.Command("claude", "abc", "/w", false, "/h.json"), " ")
	resume := strings.Join(p.Command("claude", "abc", "/w", true, "/h.json"), " ")
	if fresh != "claude --session-id abc --settings /h.json" {
		t.Errorf("fresh = %q", fresh)
	}
	if resume != "claude --resume abc --settings /h.json" {
		t.Errorf("resume = %q", resume)
	}
}

// Invariant 3, as a property over the argv: nothing points at the user's
// own settings file.
func TestClaude_CommandNeverReferencesTheUserSettings_issue3(t *testing.T) {
	for _, arg := range agent.Claude().Command("claude", "abc", "/w", false, "/h.json") {
		if strings.Contains(arg, ".claude/settings") {
			t.Errorf("argument %q points at the user's settings", arg)
		}
	}
}

// Pins the seam against the function it replaced.
func TestClaude_TranscriptPathMatchesPathsTranscript_issue46(t *testing.T) {
	if got, want := agent.Claude().TranscriptPath("/h", "/p/x", "id"), paths.Transcript("/h", "/p/x", "id"); got != want {
		t.Errorf("TranscriptPath = %q, want %q", got, want)
	}
}

// One real line through the adapter proves the delegation is wired rather
// than merely declared.
func TestClaude_StatusDerivesAPromptFromATypedLine_issue46(t *testing.T) {
	s := agent.Claude().Status
	e, ok := s.ParseEntry([]byte(`{"type":"user","timestamp":"2026-09-02T12:00:00Z","message":{"role":"user","content":"hi"}}`))
	if !ok || !e.UserIsPrompt {
		t.Fatalf("ParseEntry = %+v ok=%v, want a typed prompt", e, ok)
	}
	if k, _, ok := s.DeriveKind([]status.Entry{e}); !ok || k != status.PromptSubmitted {
		t.Errorf("DeriveKind = %v ok=%v, want PromptSubmitted", k, ok)
	}
	if k, ok := s.KindOf(hooksPayload("Stop")); !ok || k != status.TurnEnded {
		t.Errorf("KindOf(Stop) = %v ok=%v, want TurnEnded", k, ok)
	}
	if len(agent.Claude().HookEvents()) != len(status.HookEventNames()) {
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
	got := agent.Claude().TranscriptPath("/h", dir, "id")
	if want := paths.Transcript("/h", physical, "id"); got != want {
		t.Errorf("TranscriptPath(%q) = %q, want the resolved directory's %q", dir, got, want)
	}
}
