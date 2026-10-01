package sessions_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

func agentCreator(defaultAgent string) *sessions.Creator {
	return sessions.NewCreator(&FakeGit{}, sessions.CreatorOpts{WorktreeDir: paths.WorktreeDir, WorktreeRoot: "/wt", DefaultAgent: defaultAgent}, stubID)
}

// A new session runs the agent chosen for it, else its project's, else the
// config's default_agent; claude is recorded as "" (#524).
func TestCreator_ResolvesTheSessionsAgent_issue524(t *testing.T) {
	cases := []struct {
		name, chosen, project, def, want string
	}{
		{"the default, claude", "", "", "claude", ""},
		{"the default, another", "", "", "codex", "codex"},
		{"the project's wins over the default", "", "codex", "claude", "codex"},
		{"the chosen one wins over the project's", "claude", "codex", "codex", ""},
		{"a chosen agent", "gemini", "", "claude", "gemini"},
	}
	for _, tc := range cases {
		st := baseState()
		st.Projects[0].Agent = tc.project
		got, err := agentCreator(tc.def).CreateAs(t.Context(), st, sessions.NewSession{Project: "omatty", Agent: tc.chosen})
		if err != nil {
			t.Fatal(err)
		}
		if got.Agent != tc.want {
			t.Errorf("%s: Agent = %q, want %q", tc.name, got.Agent, tc.want)
		}
	}
}

// `omatty new`, which names no agent, honours the project's default (#524).
func TestAddSession_UsesTheProjectsAgent_issue524(t *testing.T) {
	store, _ := newStoreAt(t)
	if _, err := sessions.AddProject(t.Context(), store, &FakeGit{}, "/p/omatty"); err != nil {
		t.Fatal(err)
	}
	if err := sessions.SetProjectAgent(t.Context(), store, "omatty", "codex"); err != nil {
		t.Fatal(err)
	}
	got, err := sessions.AddSession(t.Context(), store, agentCreator("claude"), "omatty", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	if got.Agent != "codex" {
		t.Errorf("Agent = %q, want the project's codex", got.Agent)
	}
}

// The project default is persisted, so a relaunch honours it (#524).
func TestSetProjectAgent_Persists_issue524(t *testing.T) {
	store, _ := newStoreAt(t)
	if _, err := sessions.AddProject(t.Context(), store, &FakeGit{}, "/p/omatty"); err != nil {
		t.Fatal(err)
	}
	if err := sessions.SetProjectAgent(t.Context(), store, "omatty", "codex"); err != nil {
		t.Fatal(err)
	}
	st, err := store.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if st.Projects[0].Agent != "codex" {
		t.Errorf("persisted Agent = %q, want codex", st.Projects[0].Agent)
	}
	if err := sessions.SetProjectAgent(t.Context(), store, "nope", "codex"); err == nil {
		t.Error("an unknown project was accepted")
	}
}

var _ = session.State{}
