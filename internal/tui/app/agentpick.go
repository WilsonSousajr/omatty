// Choosing the agent (#524): a step on ctrl+o n that asks which agent a new
// session runs, and ctrl+o c, which sets the agent a project's sessions run
// unless one is chosen. The step appears only when two or more agents are
// installed, so a claude-only machine sees ctrl+o n exactly as it was.

package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

// AgentOption is one agent ctrl+o n can offer: its name, and whether its
// binary is on PATH. The check is cmd's - os/exec is not the TUI's to import
// (AGENTS.md, the exec allowlist) - so the TUI is handed the answer.
type AgentOption struct {
	Name      string
	Installed bool
}

// notInstalled is the detail an agent whose binary is missing shows.
const notInstalled = "not installed"

// noProjectAgent is the Deps.SetProjectAgent default. It names the missing
// wiring rather than appearing to succeed, as noRebind does.
func noProjectAgent(project, agent string) error {
	return fmt.Errorf("ui: no project-agent store configured for %s (agent %q)", project, agent)
}

// agentOptions is every agent the catalog knows, with whether it is installed.
func (m *Model) agentOptions() []AgentOption {
	if m.agents == nil {
		return nil
	}
	return m.agents()
}

// installedAgents counts the agents a new session could run.
func (m *Model) installedAgents() int {
	n := 0
	for _, a := range m.agentOptions() {
		if a.Installed {
			n++
		}
	}
	return n
}

// projectAgent is the agent a project's sessions run unless one is chosen:
// its own, else the config's default_agent, else claude.
func (m *Model) projectAgent(name string) string {
	for _, p := range m.state.Projects {
		if p.Name == name && p.Agent != "" {
			return p.Agent
		}
	}
	if m.defaultAgent != "" {
		return m.defaultAgent
	}
	return session.ImplicitAgent
}

// agentList is the pick list over every agent, on the project's own.
func (m *Model) agentList(title, project string) pickList {
	opts := m.agentOptions()
	items := make([]pickItem, len(opts))
	for i, a := range opts {
		items[i] = pickItem{ID: a.Name, Label: a.Name}
		if !a.Installed {
			items[i].Detail = notInstalled
		}
	}
	l := newPickList(title, items, false)
	l.Point(m.projectAgent(project))
	return l
}

// askAgent opens the agent step for a session the prompt described, or
// creates it at once when there is nothing to choose between.
func (m *Model) askAgent(req sessions.NewSession) tea.Cmd {
	if m.installedAgents() < 2 {
		return m.createCmd(req)
	}
	m.openModal(modal{Kind: modalAgent, List: m.agentList("agent for "+req.Project, req.Project), Pending: req})
	return nil
}

// chosenAgent is the agent under the cursor, if it can run. An uninstalled
// one leaves the list open and says why, rather than registering a session
// whose start can only fail.
func (m *Model) chosenAgent() (string, bool) {
	cur, ok := m.modal.List.Current()
	if !ok {
		return "", false
	}
	if cur.Detail == notInstalled {
		m.lastErr = cur.ID + " is not installed; choose another agent"
		return "", false
	}
	return cur.ID, true
}

// commitAgentStep creates the pending session with the chosen agent.
func (m *Model) commitAgentStep() tea.Cmd {
	name, ok := m.chosenAgent()
	if !ok {
		return nil
	}
	req := m.modal.Pending
	req.Agent = name
	m.modal, m.lastErr = modal{}, ""
	return m.createCmd(req)
}

// openProjectAgent opens ctrl+o c over the selected project's agent.
func (m *Model) openProjectAgent() {
	project := m.SelectedProject()
	if project == "" || len(m.agentOptions()) == 0 {
		return
	}
	m.openModal(modal{Kind: modalProjectAgent, List: m.agentList("default agent for "+project, project),
		Pending: sessions.NewSession{Project: project}})
}

// commitProjectAgent persists the project's new default, then applies it.
func (m *Model) commitProjectAgent() tea.Cmd {
	name, ok := m.chosenAgent()
	if !ok {
		return nil
	}
	project := m.modal.Pending.Project
	m.modal, m.lastErr = modal{}, ""
	set := m.setProjectAgent
	return m.persistCmd("setting a project's agent", []any{"project", project, "agent", name},
		func() error { return set(project, name) },
		func(m *Model) tea.Cmd {
			m.applyProjectAgent(project, name)
			return nil
		})
}

func (m *Model) applyProjectAgent(project, agent string) {
	for i := range m.state.Projects {
		if m.state.Projects[i].Name == project {
			m.state.Projects[i].Agent = agent
		}
	}
}
