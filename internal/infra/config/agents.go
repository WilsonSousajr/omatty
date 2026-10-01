// The agent keys (#524): which agent a new session runs when nothing else
// says, and where each agent's binary is.
//
//	default_agent = "codex"
//
//	[agents.codex]
//	bin = "/opt/homebrew/bin/codex"
//
// claude_bin stays an alias for [agents.claude] bin. Keys below 1.0 are not
// frozen, but keeping one that every config written before M17 carries costs
// nothing.

package config

import (
	"fmt"
	"strings"
)

// Agent is one [agents.<name>] table.
type Agent struct {
	// Bin is the binary to run for this agent; empty is the profile's default.
	Bin string `toml:"bin"`
}

// AgentBins is the binary the config names for each agent, by agent name:
// what the catalog's WithBins takes. claude's comes from claude_bin unless
// [agents.claude] names one, which wins as the newer spelling.
//
//	agents = agents.WithBins(cfg.AgentBins())
func (c Config) AgentBins() map[string]string {
	bins := map[string]string{}
	if c.ClaudeBin != "" {
		bins["claude"] = c.ClaudeBin
	}
	for name, a := range c.Agents {
		if a.Bin != "" {
			bins[name] = a.Bin
		}
	}
	return bins
}

// refuseBlankDefaultAgent rejects a default_agent of only whitespace: every
// session it chose would record no agent, which state.json reads as claude
// whatever the operator meant.
func refuseBlankDefaultAgent(path, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("config %s: default_agent is blank, want an agent's name such as %q", path, "claude")
	}
	return nil
}
