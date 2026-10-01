package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/config"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

func TestRegisteredRoots_ListsWhatStateJSONHolds(t *testing.T) {
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{"/p/omatty": "/p/omatty", "/work/api": "/work/api"}}
	for _, dir := range []string{"/p/omatty", "/work/api"} {
		if _, err := sessions.AddProject(t.Context(), store, git, dir); err != nil {
			t.Fatalf("AddProject(%q): %v", dir, err)
		}
	}

	got, err := registeredRoots(store)
	if err != nil {
		t.Fatalf("registeredRoots: %v", err)
	}

	if len(got) != 2 || got[0] != "/p/omatty" || got[1] != "/work/api" {
		t.Errorf("registeredRoots() = %v, want both registered roots", got)
	}
}

func TestRegisteredRoots_IsEmptyForAFreshStore(t *testing.T) {
	got, err := registeredRoots(storeIn(t))
	if err != nil {
		t.Fatalf("registeredRoots: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("registeredRoots() = %v on a fresh store, want none", got)
	}
}

// adoptFixture writes a transcript store holding one session in dir, so the
// subcommand has something real to propose.
func adoptFixture(t *testing.T, home, dir, id, prompt string) {
	t.Helper()
	slug := filepath.Join(paths.TranscriptsDir(home), paths.TranscriptSlug(dir))
	if err := os.MkdirAll(slug, 0o700); err != nil {
		t.Fatal(err)
	}
	line := `{"type":"user","cwd":"` + dir + `","message":{"role":"user","content":"` + prompt + `"}}`
	if err := os.WriteFile(filepath.Join(slug, id+".jsonl"), []byte(line+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestDispatch_KnowsRm_issue159(t *testing.T) {
	err := dispatch("bogus", nil, t.TempDir(), config.Config{}, storeIn(t))
	if err == nil || !strings.Contains(err.Error(), "rm") {
		t.Errorf("the unknown-command error %v does not list rm", err)
	}
}

// `omatty new` refuses a default_agent this omatty does not know, before it
// registers a session no launch could start (#524).
func TestNewSession_RefusesAnUnknownDefaultAgent_issue524(t *testing.T) {
	cfg := config.Defaults(t.TempDir())
	cfg.DefaultAgent = "codx"
	err := newSession(nil, cfg, []string{"p", "title"})
	if err == nil || !strings.Contains(err.Error(), "codx") {
		t.Errorf("error = %v, want one naming codx", err)
	}
}
