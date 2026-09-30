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
		if _, err := sessions.AddProject(store, git, dir); err != nil {
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

func TestAdoptSessions_RegistersThePickedSession_issue122(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "omatty")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{repo: repo}}
	if _, err := sessions.AddProject(store, git, repo); err != nil {
		t.Fatal(err)
	}
	adoptFixture(t, home, repo, "abc-123", "fix the parser")

	err := adoptSessions(store, home, git, []string{"omatty"}, strings.NewReader("1\n"))

	if err != nil {
		t.Fatalf("adoptSessions() error = %v, want nil", err)
	}
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Sessions) != 1 || st.Sessions[0].ID != "abc-123" {
		t.Fatalf("sessions = %+v, want the adopted one", st.Sessions)
	}
	if st.Sessions[0].Worktree {
		t.Error("the adopted session claims a worktree omatty did not create")
	}
}

// An empty answer is how you back out, and it must register nothing.
func TestAdoptSessions_RegistersNothingForAnEmptyAnswer_issue122(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "omatty")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{repo: repo}}
	if _, err := sessions.AddProject(store, git, repo); err != nil {
		t.Fatal(err)
	}
	adoptFixture(t, home, repo, "abc-123", "fix the parser")

	if err := adoptSessions(store, home, git, []string{"omatty"}, strings.NewReader("\n")); err != nil {
		t.Fatal(err)
	}

	st, _ := store.Load()
	if len(st.Sessions) != 0 {
		t.Errorf("sessions = %+v, want none: an empty answer chooses nothing", st.Sessions)
	}
}

// The subcommand acts on one named project, so a missing name is a usage error
// rather than a scan of everything.
func TestAdoptSessions_RequiresAProjectName_issue122(t *testing.T) {
	err := adoptSessions(storeIn(t), t.TempDir(), &FakeGit{}, nil, strings.NewReader(""))

	if err == nil {
		t.Fatal("adoptSessions() with no project returned nil, want a usage error")
	}
	if !strings.Contains(err.Error(), "adopt") {
		t.Errorf("error %q does not name the subcommand", err)
	}
}

func TestRemoveProject_ForgetsAnEmptyProject_issue159(t *testing.T) {
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{"/p/omatty": "/p/omatty"}}
	if _, err := sessions.AddProject(store, git, "/p/omatty"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	if err := removeProject(store, []string{"omatty"}); err != nil {
		t.Fatalf("removeProject: %v", err)
	}

	st, _ := store.Load()
	if len(st.Projects) != 0 {
		t.Errorf("projects after rm = %+v, want none", st.Projects)
	}
}

// `omatty rm` with no argument is usage, not a silent no-op.
func TestRemoveProject_RequiresAProjectName_issue159(t *testing.T) {
	err := removeProject(storeIn(t), nil)
	if err == nil || !strings.Contains(err.Error(), "<project>") {
		t.Errorf("removeProject with no argument = %v, want a usage error naming <project>", err)
	}
}

func TestDispatch_KnowsRm_issue159(t *testing.T) {
	err := dispatch("bogus", nil, t.TempDir(), config.Config{}, storeIn(t))
	if err == nil || !strings.Contains(err.Error(), "rm") {
		t.Errorf("the unknown-command error %v does not list rm", err)
	}
}
