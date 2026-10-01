package discovery_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/service/discovery"
)

// upperPrompt is a PromptText that reads every content as one fixed title, so
// a test can tell its answer from claude's.
func upperPrompt(json.RawMessage) (string, bool) { return "READ BY THE PORT", true }

// Since migration step 7.2 (#653) which bodies are typed prompts is the
// agent's answer, passed in, not service/status's, imported: the titles
// adoption offers come from the PromptText it is given.
func TestProposeSessions_titlesThroughThePromptPort_issue653(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "omatty")
	mkdirs(t, repo)
	root := sessionStore(t, repo, fixture{ID: "s1", Prompt: "fix the wheel"})

	got, err := discovery.ProposeSessions(root, &FakeGit{Repos: map[string]bool{repo: true}}, repo, nil, upperPrompt)

	if err != nil || len(got) != 1 || got[0].Title != "READ BY THE PORT" {
		t.Fatalf("ProposeSessions() = %+v, %v; want one session titled by the port", got, err)
	}
}

func TestFirstPromptTitle_readsThroughThePromptPort_issue653(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "omatty")
	root := sessionStore(t, repo, fixture{ID: "s1", Prompt: "fix the wheel"})

	got, err := discovery.FirstPromptTitle(transcriptOf(root, repo, "s1"), upperPrompt)

	if err != nil || got != "READ BY THE PORT" {
		t.Fatalf("FirstPromptTitle() = %q, %v; want the port's answer", got, err)
	}
}
