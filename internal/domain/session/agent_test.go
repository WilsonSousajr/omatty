package session_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
)

// claude has one spelling in state.json, the empty one every row before #46
// carries; any other agent is recorded by name (#524).
func TestStoredAgent_ClaudeIsEmpty_issue524(t *testing.T) {
	for name, want := range map[string]string{"": "", "claude": "", "codex": "codex"} {
		if got := session.StoredAgent(name); got != want {
			t.Errorf("StoredAgent(%q) = %q, want %q", name, got, want)
		}
	}
}
