package cli_test

import (
	"bytes"
	"errors"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/cli"
	"github.com/WilsonSousajr/omatty/internal/domain/forge"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

// Migration step 7.2 (#653) moved adopt, rm and stats here with their output
// unchanged. cmd's tests never read what they printed - stdout was not theirs
// to capture - so these pin the lines themselves.

func registered(t *testing.T) (sessions.StateStore, string, *FakeGit) {
	t.Helper()
	home := t.TempDir()
	repo := filepath.Join(home, "omatty")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	store, git := storeIn(t), &FakeGit{Roots: map[string]string{repo: repo}}
	if _, err := sessions.AddProject(t.Context(), store, git, repo); err != nil {
		t.Fatal(err)
	}
	return store, home, git
}

func TestAdopt_saysSoWhenNothingIsUnregistered_issue653(t *testing.T) {
	store, home, git := registered(t)
	if err := os.MkdirAll(paths.TranscriptsDir(home), 0o700); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := cli.Adopt(t.Context(), &out, strings.NewReader(""), portsFor(store, home, git), []string{"omatty"}); err != nil {
		t.Fatal(err)
	}
	if want := "no unregistered claude sessions found in " + filepath.Join(home, "omatty") + "\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

func TestAdopt_listsAsksAndReportsEachAdoption_issue653(t *testing.T) {
	store, home, git := registered(t)
	repo := filepath.Join(home, "omatty")
	adoptFixture(t, home, repo, "abc-123", "fix the parser")
	var out bytes.Buffer
	if err := cli.Adopt(t.Context(), &out, strings.NewReader("1\n"), portsFor(store, home, git), []string{"omatty"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"fix the parser", "adopt which? (numbers, or `all`, or enter for none)",
		"adopted abc-123 (fix the parser) in " + repo} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}

func TestRemove_saysTheRepositoryIsUntouched_issue653(t *testing.T) {
	store, home, _ := registered(t)
	var out bytes.Buffer
	if err := cli.Remove(t.Context(), &out, store, []string{"omatty"}); err != nil {
		t.Fatal(err)
	}
	if want := "removed omatty (the repository at " + filepath.Join(home, "omatty") + " is untouched)\n"; out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// With no forge to ask, the gate rate still prints: half a measurement is
// better than none (#332).
func TestStats_withNoForgeStillReports_issue653(t *testing.T) {
	var out bytes.Buffer
	project := session.Project{Name: "omatty", Root: "/p/omatty"}
	if err := cli.Stats(t.Context(), &out, &FakeStore{}, project, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "the numbers for omatty:\n  nothing measured yet") {
		t.Errorf("output = %q", out.String())
	}
}

// A forge that will not answer is said once and is not a failure (#310).
func TestStats_aForgeThatFailsIsSaidNotFatal_issue653(t *testing.T) {
	var out bytes.Buffer
	failing := func(string) ([]forge.PR, error) { return nil, errors.New("gh is not installed") }
	project := session.Project{Name: "omatty", Root: "/p/omatty"}
	if err := cli.Stats(t.Context(), &out, &FakeStore{}, project, failing); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "(no pull requests: gh is not installed)\nthe numbers for omatty:") {
		t.Errorf("output = %q", out.String())
	}
}

// A state.json that will not load is the command's error.
func TestStats_aStoreThatWillNotLoadIsAnError_issue653(t *testing.T) {
	err := cli.Stats(t.Context(), &bytes.Buffer{}, &FakeStore{Err: errors.New("broken")}, session.Project{}, nil)
	if err == nil {
		t.Error("Stats over a broken store = nil, want its error")
	}
}
