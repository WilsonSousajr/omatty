// The tier-aware surface (#526): the TUI never claims more than a session's
// agent lets omatty know. A Process-tier session is running or exited and
// never idle; an agent with no turn boundary has no "this turn", no gate
// auto-run and no turn-end notice; an agent with no resume offers to start
// fresh rather than a resume that cannot happen. Each degradation says why,
// naming the agent, rather than leaving a feature silently missing.

package app

import (
	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/tui/theme"
)

// runningGlyph is a Process-tier session whose process lives. A Geometric
// Shape like the status glyphs (#175), and none of theirs: an agent that
// cannot say busy must not look idle, nor busy.
const runningGlyph = "▷"

// fullCaps is what a session is assumed to offer when no catalog says
// otherwise: claude's, which is every session before M17.
var fullCaps = agent.Caps{Identity: agent.Assigned, Status: agent.StatusHooks, Waiting: true, Resume: true, TurnBoundary: true}

// agentOf is the session's agent by name, claude for a row that names none.
func (m *Model) agentOf(id string) string {
	if i, ok := m.sessionIndex(id); ok && m.state.Sessions[i].Agent != "" {
		return m.state.Sessions[i].Agent
	}
	return session.ImplicitAgent
}

// capsOf is the session's agent's capabilities, claude's when the catalog
// does not know it or none was wired.
func (m *Model) capsOf(id string) agent.Caps {
	if m.agentCaps == nil {
		return fullCaps
	}
	if caps, ok := m.agentCaps(m.agentOf(id)); ok {
		return caps
	}
	return fullCaps
}

func (m *Model) processTier(id string) bool  { return m.capsOf(id).Tier() == agent.Process }
func (m *Model) turnBoundary(id string) bool { return m.capsOf(id).TurnBoundary }

// processCell is a Process-tier session's whole status: its process runs, or
// it does not.
func (m *Model) processCell(id string) string {
	if m.terms[id] != nil {
		return theme.Muted.Render(runningGlyph)
	}
	return statusCell(dstatus.StatusExited)
}

// processState is the header's status for a Process-tier session.
func (m *Model) processState(id string) string {
	if m.terms[id] != nil {
		return m.processCell(id) + " running"
	}
	return m.processCell(id) + " exited"
}

// agentLabel names a session's agent and tier for the header, "" for
// claude: one agent named on every pane is noise, and a claude-only
// operator sees the header as it was.
func (m *Model) agentLabel(id string) string {
	name := m.agentOf(id)
	if name == session.ImplicitAgent {
		return ""
	}
	return name + " · " + m.capsOf(id).Tier().String()
}

// resumeHint is what enter does on a stopped session: resume it, or, for an
// agent with no resume, start it fresh.
func (m *Model) resumeHint(id string) string {
	if m.capsOf(id).Resume {
		return "enter resumes it"
	}
	return "enter starts it fresh"
}

// noTurnReason is why a turn-scoped feature is unavailable for a session.
func (m *Model) noTurnReason(id string) string {
	return m.agentOf(id) + " reports no turn's end, so the diff stays on the whole session"
}

// lostConversation is the stopped pane's line for an agent with no resume.
func (m *Model) lostConversation(id string) string {
	return m.agentOf(id) + " cannot resume: the conversation is lost"
}
