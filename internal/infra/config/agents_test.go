package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/config"
)

// With no file, every new session runs claude, as before M17 (#524).
func TestLoad_DefaultAgentIsClaude_issue524(t *testing.T) {
	if got := config.Defaults("/h").DefaultAgent; got != "claude" {
		t.Errorf("DefaultAgent = %q, want claude", got)
	}
}

// default_agent and an [agents.<name>] table's bin are read; claude_bin stays
// an alias for claude's (#524).
func TestLoad_ReadsTheAgentKeys_issue524(t *testing.T) {
	home := t.TempDir()
	cfg, err := config.Load(writeConfig(t, home, `
default_agent = "codex"
claude_bin = "/opt/claude"

[agents.codex]
bin = "/opt/codex"
`), home)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultAgent != "codex" {
		t.Errorf("DefaultAgent = %q, want codex", cfg.DefaultAgent)
	}
	want := map[string]string{"claude": "/opt/claude", "codex": "/opt/codex"}
	if got := cfg.AgentBins(); !reflect.DeepEqual(got, want) {
		t.Errorf("AgentBins() = %v, want %v", got, want)
	}
}

// [agents.claude] bin is the new spelling, and wins over the alias.
func TestAgentBins_TheAgentsTableWinsOverClaudeBin_issue524(t *testing.T) {
	home := t.TempDir()
	cfg, err := config.Load(writeConfig(t, home, "claude_bin = \"/old\"\n[agents.claude]\nbin = \"/new\"\n"), home)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.AgentBins()["claude"]; got != "/new" {
		t.Errorf("claude's bin = %q, want the [agents.claude] one", got)
	}
}

// A key the agent table does not know is refused like any other (#44).
func TestLoad_UnknownAgentKeyIsRefused_issue524(t *testing.T) {
	home := t.TempDir()
	_, err := config.Load(writeConfig(t, home, "[agents.codex]\nflags = [\"-x\"]\n"), home)
	if err == nil || !strings.Contains(err.Error(), "flags") {
		t.Errorf("error = %v, want one naming flags", err)
	}
}

// A blank default_agent would make every new session's agent empty, which
// state.json reads as claude whatever the operator meant.
func TestLoad_RefusesABlankDefaultAgent_issue524(t *testing.T) {
	home := t.TempDir()
	_, err := config.Load(writeConfig(t, home, "default_agent = \" \"\n"), home)
	if err == nil || !strings.Contains(err.Error(), "default_agent") {
		t.Errorf("error = %v, want one naming default_agent", err)
	}
}

// An [agents.x] block with a command declares a generic agent, run as
// written (#525).
func TestLoad_ReadsAGenericAgentsCommand_issue525(t *testing.T) {
	home := t.TempDir()
	cfg, err := config.Load(writeConfig(t, home, "[agents.aider]\ncommand = [\"aider\", \"--no-auto-commits\"]\n"), home)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{"aider": {"aider", "--no-auto-commits"}}
	if got := cfg.GenericAgents(); !reflect.DeepEqual(got, want) {
		t.Errorf("GenericAgents() = %v, want %v", got, want)
	}
}

// A command names its own binary, so bin beside it is two answers to one
// question; an empty command names nothing to run (#525).
func TestLoad_RefusesAMalformedGenericBlock_issue525(t *testing.T) {
	for body, want := range map[string]string{
		"[agents.aider]\nbin = \"a\"\ncommand = [\"aider\"]\n": "bin",
		"[agents.aider]\ncommand = []\n":                       "command",
		"[agents.aider]\ncommand = [\" \"]\n":                  "command",
	} {
		home := t.TempDir()
		_, err := config.Load(writeConfig(t, home, body), home)
		if err == nil || !strings.Contains(err.Error(), "aider") || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error = %v, want one naming aider and %s", body, err, want)
		}
	}
}
