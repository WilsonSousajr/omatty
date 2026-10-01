package agent_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/domain/status"
)

func toyProfile(name string) agent.Profile {
	return agent.Profile{Name: name, DefaultBin: name, Command: func(bin, _, _ string, _ bool, _ string) []string { return []string{bin} }}
}

func mustCatalog(t *testing.T, profiles ...agent.Profile) agent.Catalog {
	t.Helper()
	c, err := agent.NewCatalog(profiles...)
	if err != nil {
		t.Fatalf("NewCatalog: %v", err)
	}
	return c
}

// An empty name is the first profile, claude in cmd's catalog: every row
// written before #46 has one, and it still relaunches (invariant 9, #521).
func TestCatalog_EmptyNameIsTheFirstProfile_issue521(t *testing.T) {
	c := mustCatalog(t, toyProfile("claude"), toyProfile("toy"))
	if p, err := c.Lookup(""); err != nil || p.Name != "claude" {
		t.Errorf(`Lookup("") = %q, %v; want claude`, p.Name, err)
	}
	if p, err := c.Lookup("toy"); err != nil || p.Name != "toy" {
		t.Errorf("Lookup(toy) = %q, %v; want toy", p.Name, err)
	}
	if got := strings.Join(c.Names(), ","); got != "claude,toy" {
		t.Errorf("Names() = %s, want claude,toy", got)
	}
}

// An unknown agent is an error naming it and every known one, never a
// silent claude (#521).
func TestCatalog_UnknownAgentIsAnErrorNamingIt_issue521(t *testing.T) {
	_, err := mustCatalog(t, toyProfile("claude")).Lookup("codex")
	if err == nil || !strings.Contains(err.Error(), `"codex"`) || !strings.Contains(err.Error(), "claude") {
		t.Errorf("Lookup(codex) error = %v, want it to name codex and claude", err)
	}
}

func TestNewCatalog_RefusesAnEmptyOrDuplicatedCatalog_issue521(t *testing.T) {
	if _, err := agent.NewCatalog(); err == nil {
		t.Error("an empty catalog was accepted")
	}
	_, err := agent.NewCatalog(toyProfile("toy"), toyProfile("toy"))
	if err == nil || !strings.Contains(err.Error(), `"toy"`) {
		t.Errorf("duplicate error = %v, want it to name toy", err)
	}
}

// A capability declared without the function that serves it would fail at
// session start; the catalog refuses it at build (#520, #521).
func TestNewCatalog_RefusesADeclaredCapabilityWithoutItsFunc_issue521(t *testing.T) {
	hooks := toyProfile("hooky")
	hooks.Caps = agent.Caps{Identity: agent.Assigned, Status: agent.StatusHooks}
	hooks.TranscriptPath = func(_, _, _ string) string { return "" }
	hooks.Status = status.Adapter(nil)
	if _, err := agent.NewCatalog(hooks); err == nil || !strings.Contains(err.Error(), "hooky") {
		t.Errorf("error = %v, want one naming hooky", err)
	}
	noCommand := toyProfile("mute")
	noCommand.Command = nil
	if _, err := agent.NewCatalog(noCommand); err == nil {
		t.Error("a profile with no command was accepted")
	}
}

// The configured binary wins over the profile's default (#521).
func TestCatalog_BinPrefersTheConfiguredOne_issue521(t *testing.T) {
	c := mustCatalog(t, toyProfile("claude"), toyProfile("toy")).WithBins(map[string]string{"claude": "/opt/claude"})
	claude, _ := c.Lookup("claude")
	toy, _ := c.Lookup("toy")
	if got := c.Bin(claude); got != "/opt/claude" {
		t.Errorf("Bin(claude) = %q, want the configured /opt/claude", got)
	}
	if got := c.Bin(toy); got != "toy" {
		t.Errorf("Bin(toy) = %q, want its default", got)
	}
}
