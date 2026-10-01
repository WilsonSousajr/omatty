package agent_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
)

// The argv assertions #36 and invariant 3 made against the launcher, now
// against the template itself.
func TestClaude_CommandStartsFreshOrResumes_issue36(t *testing.T) {
	fresh := strings.Join(agent.ClaudeCommand("claude", "abc", "/w", false, "/h.json"), " ")
	resume := strings.Join(agent.ClaudeCommand("claude", "abc", "/w", true, "/h.json"), " ")
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
	for _, arg := range agent.ClaudeCommand("claude", "abc", "/w", false, "/h.json") {
		if strings.Contains(arg, ".claude/settings") {
			t.Errorf("argument %q points at the user's settings", arg)
		}
	}
}
