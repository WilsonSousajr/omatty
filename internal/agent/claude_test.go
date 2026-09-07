package agent_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/agent"
	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/watcher"
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
	if k, _, ok := s.DeriveKind([]watcher.Entry{e}); !ok || k != watcher.PromptSubmitted {
		t.Errorf("DeriveKind = %v ok=%v, want PromptSubmitted", k, ok)
	}
	if k, ok := s.KindOf(hooksPayload("Stop")); !ok || k != watcher.TurnEnded {
		t.Errorf("KindOf(Stop) = %v ok=%v, want TurnEnded", k, ok)
	}
	if len(agent.Claude().HookEvents()) != len(watcher.HookEventNames()) {
		t.Error("HookEvents is not the listener's own list (#78)")
	}
}
