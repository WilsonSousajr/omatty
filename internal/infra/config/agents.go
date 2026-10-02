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

	"github.com/BurntSushi/toml"
)

// Agent is one [agents.<name>] table.
type Agent struct {
	// Bin is the binary to run for this agent; empty is the profile's default.
	Bin string `toml:"bin"`
	// Command declares a generic agent: any program, run as written in the
	// session's directory, at the Process tier (#525). Its first word is its
	// binary, so it takes no bin beside it. Only command: no resume template
	// and no transcript rules, because declarative adapters are refused by
	// name in M17's design, and this is the line that keeps them out.
	//
	//	[agents.aider]
	//	command = ["aider", "--no-auto-commits"]
	Command []string `toml:"command"`
}

// GenericAgents is every agent the config declares by command, by name.
//
//	for name, argv := range cfg.GenericAgents() { ... agent.Generic(name, argv) }
func (c Config) GenericAgents() map[string][]string {
	out := map[string][]string{}
	for name, a := range c.Agents {
		if len(a.Command) > 0 {
			out[name] = a.Command
		}
	}
	return out
}

// refuseBadAgentBlocks rejects a block whose command was written and names
// nothing to run, or that names a bin beside its command.
func refuseBadAgentBlocks(path string, md toml.MetaData, agents map[string]Agent) error {
	for name, a := range agents {
		if !md.IsDefined("agents", name, "command") {
			continue
		}
		if len(a.Command) == 0 || strings.TrimSpace(a.Command[0]) == "" {
			return fmt.Errorf("config %s: agents.%s.command names no program, want one such as [%q]", path, name, name)
		}
		if a.Bin != "" {
			return fmt.Errorf("config %s: agents.%s has both bin and command, want only command: its first word is the binary", path, name)
		}
	}
	return nil
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
