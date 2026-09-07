package agent_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/agent"
)

// Invariant 9: every session row written before #46 has no agent, and it
// still relaunches.
func TestLookup_AnEmptyNameIsClaude_issue46(t *testing.T) {
	p, err := agent.Lookup("")
	if err != nil || p.Name != "claude" {
		t.Fatalf("Lookup(\"\") = %q, %v; want claude", p.Name, err)
	}
	if byName, _ := agent.Lookup("claude"); byName.Name != p.Name {
		t.Errorf("Lookup(claude) = %q, want the same profile", byName.Name)
	}
}

func TestLookup_UnknownAgentNamesItAndTheKnownOnes_issue46(t *testing.T) {
	_, err := agent.Lookup("codex")
	if err == nil || !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), "claude") {
		t.Fatalf("Lookup(codex) error = %v, want it to name codex and the known profiles", err)
	}
}

// A nil field would panic at session start, not at build.
func TestClaude_EveryProfileFieldIsSet_issue46(t *testing.T) {
	p := agent.Claude()
	if p.Command == nil || p.TranscriptPath == nil || p.HookEvents == nil || p.RenderSettings == nil || p.Status == nil || p.DefaultBin == "" {
		t.Errorf("Claude() has a nil field: %+v", p)
	}
	if len(agent.Names()) != 1 || agent.Names()[0] != "claude" {
		t.Errorf("Names() = %v, want [claude]", agent.Names())
	}
}
