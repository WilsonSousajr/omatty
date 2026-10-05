package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// fakeInstalled stands in for the PATH check cmd wires: which agents ctrl+o n
// may offer, and whether each one's binary is there (#524).
type fakeInstalled struct{ Options []app.AgentOption }

func (f fakeInstalled) fn() []app.AgentOption { return f.Options }

// recordProjectAgent is a named fake for Deps.SetProjectAgent.
type recordProjectAgent struct {
	Project, Agent string
	Calls          int
}

func (r *recordProjectAgent) fn(project, agent string) error {
	r.Calls++
	r.Project, r.Agent = project, agent
	return nil
}

var (
	bothInstalled = fakeInstalled{Options: []app.AgentOption{{Name: "claude", Installed: true}, {Name: "codex", Installed: true}}}
	onlyClaude    = fakeInstalled{Options: []app.AgentOption{{Name: "claude", Installed: true}, {Name: "codex"}}}
)

func modelWithAgents(t *testing.T, st session.State, c *recordCreate, f fakeInstalled, r *recordProjectAgent) *app.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	m := app.NewModel(app.Deps{State: st, Terms: terms, Create: c.fn, Start: noStart,
		Agents: f.fn, DefaultAgent: "claude", SetProjectAgent: r.fn})
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return m
}

func newSessionPrompt(m *app.Model, title string) {
	press(m, ctrl('o'))
	press(m, key('n'))
	for _, r := range title {
		press(m, key(r))
	}
	pressAndSettle(m, special(tea.KeyEnter))
}

// With two agents installed, ctrl+o n asks which one before creating
// anything, and the one chosen is the session's (#524).
func TestModel_TwoInstalledAgentsAddAnAgentStep_issue524(t *testing.T) {
	c := &recordCreate{}
	m := modelWithAgents(t, twoProjectState(), c, bothInstalled, &recordProjectAgent{})

	newSessionPrompt(m, "fix")
	if c.Calls != 0 {
		t.Fatalf("created before an agent was chosen: %+v", c)
	}
	if !strings.Contains(m.View().Content, "codex") {
		t.Fatalf("the agent step does not list codex:\n%s", m.View().Content)
	}
	press(m, special(tea.KeyDown))
	pressAndSettle(m, special(tea.KeyEnter))

	if c.Calls != 1 || c.Agent != "codex" || c.Title != "fix" || c.Project != "omatty" {
		t.Errorf("created %+v, want one fix session on omatty running codex", c)
	}
}

// A claude-only machine sees no change: no step, the session created at once
// with the project's agent (#524).
func TestModel_OneInstalledAgentSkipsTheStep_issue524(t *testing.T) {
	c := &recordCreate{}
	m := modelWithAgents(t, twoProjectState(), c, onlyClaude, &recordProjectAgent{})

	newSessionPrompt(m, "fix")

	if c.Calls != 1 || c.Agent != "" {
		t.Errorf("created %+v, want one session with no agent chosen", c)
	}
}

// The step opens on the project's own agent, so enter alone keeps it (#524).
func TestModel_AgentStepIsPreselectedOnTheProjectsAgent_issue524(t *testing.T) {
	st := twoProjectState()
	st.Projects[0].Agent = "codex"
	c := &recordCreate{}
	m := modelWithAgents(t, st, c, bothInstalled, &recordProjectAgent{})

	newSessionPrompt(m, "fix")
	pressAndSettle(m, special(tea.KeyEnter))

	if c.Agent != "codex" {
		t.Errorf("created with %q, want the project's codex", c.Agent)
	}
}

// An agent whose binary is not installed is listed, says so, and cannot be
// chosen (#524).
func TestModel_AnUninstalledAgentCannotBeChosen_issue524(t *testing.T) {
	f := fakeInstalled{Options: []app.AgentOption{{Name: "claude", Installed: true}, {Name: "codex", Installed: true}, {Name: "gemini"}}}
	c := &recordCreate{}
	m := modelWithAgents(t, twoProjectState(), c, f, &recordProjectAgent{})

	newSessionPrompt(m, "fix")
	if !strings.Contains(m.View().Content, "not installed") {
		t.Errorf("gemini is not marked as not installed:\n%s", m.View().Content)
	}
	press(m, special(tea.KeyDown))
	press(m, special(tea.KeyDown))
	pressAndSettle(m, special(tea.KeyEnter))

	if c.Calls != 0 {
		t.Errorf("created %+v with an agent that is not installed", c)
	}
}

// ctrl+o c sets the selected project's default agent, persisted (#524).
func TestModel_ctrlOcSetsTheProjectsAgent_issue524(t *testing.T) {
	r := &recordProjectAgent{}
	m := modelWithAgents(t, twoProjectState(), &recordCreate{}, bothInstalled, r)

	press(m, ctrl('o'))
	press(m, key('c'))
	press(m, special(tea.KeyDown))
	pressAndSettle(m, special(tea.KeyEnter))

	if r.Calls != 1 || r.Project != "omatty" || r.Agent != "codex" {
		t.Errorf("SetProjectAgent got %+v, want omatty -> codex", r)
	}
}
