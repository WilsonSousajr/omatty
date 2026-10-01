// The adopt and rm tests, moved from cmd/omatty with the code they test
// (migration step 7.2, #653).

package cli_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/cli"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// portsFor is adopt's ports over a scratch home, with git answered by fake.
func portsFor(store sessions.StateStore, home string, git *FakeGit) cli.AdoptPorts {
	return cli.AdoptPorts{Store: store, Git: git, Branches: git, TranscriptsDir: paths.TranscriptsDir(home),
		Prompts: status.PromptText}
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
	if _, err := sessions.AddProject(t.Context(), store, git, repo); err != nil {
		t.Fatal(err)
	}
	adoptFixture(t, home, repo, "abc-123", "fix the parser")

	err := cli.Adopt(t.Context(), io.Discard, strings.NewReader("1\n"), portsFor(store, home, git), []string{"omatty"})

	if err != nil {
		t.Fatalf("Adopt() error = %v, want nil", err)
	}
	st, err := store.Load(t.Context())
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
	if _, err := sessions.AddProject(t.Context(), store, git, repo); err != nil {
		t.Fatal(err)
	}
	adoptFixture(t, home, repo, "abc-123", "fix the parser")

	if err := cli.Adopt(t.Context(), io.Discard, strings.NewReader("\n"), portsFor(store, home, git), []string{"omatty"}); err != nil {
		t.Fatal(err)
	}

	st, _ := store.Load(t.Context())
	if len(st.Sessions) != 0 {
		t.Errorf("sessions = %+v, want none: an empty answer chooses nothing", st.Sessions)
	}
}

// The subcommand acts on one named project, so a missing name is a usage error
// rather than a scan of everything.
func TestAdoptSessions_RequiresAProjectName_issue122(t *testing.T) {
	err := cli.Adopt(t.Context(), io.Discard, strings.NewReader(""), portsFor(storeIn(t), t.TempDir(), &FakeGit{}), nil)

	if err == nil {
		t.Fatal("Adopt() with no project returned nil, want a usage error")
	}
	if !strings.Contains(err.Error(), "adopt") {
		t.Errorf("error %q does not name the subcommand", err)
	}
}

func TestRemoveProject_ForgetsAnEmptyProject_issue159(t *testing.T) {
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{"/p/omatty": "/p/omatty"}}
	if _, err := sessions.AddProject(t.Context(), store, git, "/p/omatty"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}

	if err := cli.Remove(t.Context(), io.Discard, store, []string{"omatty"}); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	st, _ := store.Load(t.Context())
	if len(st.Projects) != 0 {
		t.Errorf("projects after rm = %+v, want none", st.Projects)
	}
}

// `omatty rm` with no argument is usage, not a silent no-op.
func TestRemoveProject_RequiresAProjectName_issue159(t *testing.T) {
	err := cli.Remove(t.Context(), io.Discard, storeIn(t), nil)
	if err == nil || !strings.Contains(err.Error(), "<project>") {
		t.Errorf("Remove with no argument = %v, want a usage error naming <project>", err)
	}
}
