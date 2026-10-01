package agent_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
)

// The derivation the support matrix will be generated from (#520): one row
// per tier boundary, each a single capability away from its neighbour.
func TestCaps_TierIsDerived_issue520(t *testing.T) {
	full := agent.Caps{Identity: agent.Assigned, Status: agent.StatusHooks, Waiting: true, Resume: true, TurnBoundary: true}
	cases := []struct {
		name string
		caps agent.Caps
		want agent.Tier
	}{
		{"claude's shape", full, agent.Full},
		{"reported identity is known", with(full, func(c *agent.Caps) { c.Identity = agent.Reported }), agent.Full},
		{"scanned identity is known", with(full, func(c *agent.Caps) { c.Identity = agent.Scanned }), agent.Full},
		{"no turn boundary is still full", with(full, func(c *agent.Caps) { c.TurnBoundary = false }), agent.Full},
		{"no waiting drops to transcript", with(full, func(c *agent.Caps) { c.Waiting = false }), agent.Transcript},
		{"no resume drops to transcript", with(full, func(c *agent.Caps) { c.Resume = false }), agent.Transcript},
		{"transcript status", with(full, func(c *agent.Caps) { c.Status = agent.StatusTranscript }), agent.Transcript},
		{"process status", with(full, func(c *agent.Caps) { c.Status = agent.StatusProcess }), agent.Process},
		{"unknown identity", with(full, func(c *agent.Caps) { c.Identity = agent.NoIdentity }), agent.Process},
		{"the zero value", agent.Caps{}, agent.Process},
	}
	for _, tc := range cases {
		if got := tc.caps.Tier(); got != tc.want {
			t.Errorf("%s: Tier(%+v) = %v, want %v", tc.name, tc.caps, got, tc.want)
		}
	}
}

// Every combination, against the two rules no tier may break: a session whose
// transcript omatty cannot find, or whose status is the process alone, knows
// nothing beyond running or exited; and Full needs every one of its parts.
func TestCaps_EveryCombinationKeepsTheTierRules_issue520(t *testing.T) {
	for _, c := range everyCaps() {
		tier := c.Tier()
		if (c.Identity == agent.NoIdentity || c.Status == agent.StatusProcess) && tier != agent.Process {
			t.Errorf("Tier(%+v) = %v, want Process", c, tier)
		}
		if tier == agent.Full && (c.Status != agent.StatusHooks || !c.Waiting || !c.Resume) {
			t.Errorf("Tier(%+v) = Full without hooks, waiting and resume", c)
		}
	}
}

func TestTier_StringNamesIt_issue520(t *testing.T) {
	for tier, want := range map[agent.Tier]string{agent.Full: "full", agent.Transcript: "transcript", agent.Process: "process"} {
		if got := tier.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", tier, got, want)
		}
	}
}

func with(c agent.Caps, edit func(*agent.Caps)) agent.Caps {
	edit(&c)
	return c
}

func everyCaps() []agent.Caps {
	var all []agent.Caps
	for _, id := range []agent.Identity{agent.NoIdentity, agent.Assigned, agent.Reported, agent.Scanned} {
		for _, st := range []agent.StatusSource{agent.StatusProcess, agent.StatusTranscript, agent.StatusHooks} {
			for _, flags := range [][3]bool{{}, {true}, {false, true}, {false, false, true}, {true, true, true}, {true, true}} {
				all = append(all, agent.Caps{Identity: id, Status: st, Waiting: flags[0], Resume: flags[1], TurnBoundary: flags[2]})
			}
		}
	}
	return all
}
