package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// cmd/omatty had no test file at all, and the coverage gate measures only
// ./internal/..., so every line here was unmeasured as well as unexercised -
// which is why the readLine EOF path and the projectRegistrar name mismatch
// both shipped (#91). These cover the parts that hold logic; the wiring in
// tuiDeps and run is exercised by the milestone's PTY smoke test, which a
// person reads (AGENTS.md, "Build and test commands").

// FakeGit is a named fake for the git methods the adapters here need.
//
// RepoRoot and MainCheckout answer from separate maps, and that separation is
// the point: they are different questions - inside a linked worktree the first
// returns the worktree and the second the repository it was forked from - and
// one shared answer for both is what made adoption's worktree bug invisible to
// this package (#91, #122). Worktrees fills MainCheckout alone.
type FakeGit struct {
	Roots     map[string]string
	Worktrees map[string]string
	Branch    string
}

func (f *FakeGit) RepoRoot(dir string) (string, error) { return lookup(f.Roots, dir) }

func (f *FakeGit) MainCheckout(dir string) (string, error) {
	if root, ok := f.Worktrees[dir]; ok {
		return root, nil
	}
	return lookup(f.Roots, dir)
}

func (f *FakeGit) CurrentBranch(string) (string, error) { return f.Branch, nil }

func (f *FakeGit) RemoveWorktree(string, string) error { return nil }

func lookup(roots map[string]string, dir string) (string, error) {
	if root, ok := roots[dir]; ok {
		return root, nil
	}
	return "", errNotARepo{dir}
}

type errNotARepo struct{ dir string }

func (e errNotARepo) Error() string { return "not a git repository: " + e.dir }

// storeIn builds a registry over a temporary state.json.
func storeIn(t *testing.T) *registry.Store {
	t.Helper()
	return registry.NewStore(filepath.Join(t.TempDir(), "state.json"))
}

func TestReadLine_TakesTheAnswerAndTrimsIt(t *testing.T) {
	for _, tt := range []struct {
		name, in, want string
	}{
		{"a selection", "1 3\n", "1 3"},
		{"trailing spaces", "  all  \n", "all"},
		{"just enter", "\n", ""},
		// The EOF path: a closed stdin is no answer, which is the same as
		// choosing nothing. The old guard for it was dead code - both branches
		// returned "" - and nothing covered either (#91).
		{"eof with no newline", "all", "all"},
		{"eof with nothing at all", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := readLine(strings.NewReader(tt.in)); got != tt.want {
				t.Errorf("readLine(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestRegisteredRoots_ListsWhatStateJSONHolds(t *testing.T) {
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{"/p/omatty": "/p/omatty", "/work/api": "/work/api"}}
	for _, dir := range []string{"/p/omatty", "/work/api"} {
		if _, err := registry.AddProject(store, git, dir); err != nil {
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

// Regression, issue #91: projectRegistrar discarded the Project AddProject
// wrote, so the TUI rebuilt one from the picked row instead. Discovery names a
// candidate after MainCheckout's directory and AddProject after RepoRoot's, so
// where those disagree the sidebar held a name state.json did not.
func TestProjectRegistrar_ReturnsTheProjectTheRegistryWrote_issue91(t *testing.T) {
	store := storeIn(t)
	// A worktree whose main checkout is elsewhere: the two names differ.
	git := &FakeGit{Roots: map[string]string{"/p/omatty": "/p/omatty"}}

	got := projectRegistrar(store, git)([]string{"/p/omatty"})

	if len(got) != 1 {
		t.Fatalf("registrar returned %d registrations, want 1", len(got))
	}
	if got[0].Err != nil {
		t.Fatalf("registering /p/omatty: %v", got[0].Err)
	}
	if got[0].Project.Name != "omatty" || got[0].Project.Root != "/p/omatty" {
		t.Errorf("Project = %+v, want the row the registry wrote", got[0].Project)
	}
}

// A collision is reported against the one root it belongs to, and the rest
// still register: one bad candidate must not abandon a bulk pick (#91).
func TestProjectRegistrar_ReportsACollisionAndCarriesOn_issue91(t *testing.T) {
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{
		"/p/omatty": "/p/omatty", "/other/omatty": "/other/omatty", "/p/notes": "/p/notes",
	}}
	registrar := projectRegistrar(store, git)
	registrar([]string{"/p/omatty"})

	got := registrar([]string{"/other/omatty", "/p/notes"})

	if len(got) != 2 {
		t.Fatalf("registrar returned %d registrations, want 2", len(got))
	}
	if got[0].Err == nil {
		t.Errorf("registering a duplicate name reported no error: %+v", got[0])
	}
	if got[1].Err != nil {
		t.Errorf("the second root was abandoned after the first failed: %v", got[1].Err)
	}
}

// The archiver returns the row the registry removed, which is what decides
// whether a worktree may be deleted (#40).
func TestSessionArchiver_ReturnsTheRemovedSession_issue40(t *testing.T) {
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{"/p/omatty": "/p/omatty"}}
	if _, err := registry.AddProject(store, git, "/p/omatty"); err != nil {
		t.Fatalf("AddProject: %v", err)
	}
	st, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	st.Sessions = append(st.Sessions, registry.Session{
		ID: "s1", Project: "omatty", Title: "main", Dir: "/wt/omatty/fix", Worktree: true,
	})
	if err := store.Save(st); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := sessionArchiver(store)("s1")

	if err != nil {
		t.Fatalf("archiving s1: %v", err)
	}
	if !got.Worktree || got.Dir != "/wt/omatty/fix" {
		t.Errorf("removed session = %+v, want the row with its worktree fields", got)
	}
}

func TestSessionRenamer_RefusesABlankTitle_issue41(t *testing.T) {
	store := storeIn(t)
	rename := sessionRenamer(store)

	if err := rename("s1", "   "); err == nil {
		t.Error("renaming to a whitespace-only title succeeded, want an error")
	}
}

// noAddProject-style check on the proposer: a store it cannot read is an error
// the picker surfaces, not an empty list that reads as "claude has never run".
func TestProjectProposer_SurfacesAnUnreadableStore_issue91(t *testing.T) {
	proposals, err := projectProposer(storeIn(t), t.TempDir(), &FakeGit{})()

	if err == nil {
		t.Errorf("proposer returned %v and no error for a store with no transcripts dir", proposals)
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
	if _, err := registry.AddProject(store, git, repo); err != nil {
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
	if _, err := registry.AddProject(store, git, repo); err != nil {
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

// The wiring took the concrete *vcs.CLI, so not one of these adapters could be
// built in a test at all - the untestability registry.RepoRooter's own doc
// records as the #91 defect, restated for the pickers (#122). This is the test
// that could not be written before, and it is the whole point of narrowing the
// parameter: every picker dependency is now reachable without a repository.
func TestWithPickerDeps_BuildsEveryPickerDependency_issue122(t *testing.T) {
	deps := withPickerDeps(ui.RunDeps{}, storeIn(t), t.TempDir(), &FakeGit{})

	for name, built := range map[string]bool{
		"Discover":     deps.Discover != nil,
		"AddProject":   deps.AddProject != nil,
		"AdoptPropose": deps.AdoptPropose != nil,
		"AdoptCommit":  deps.AdoptCommit != nil,
	} {
		if !built {
			t.Errorf("withPickerDeps left %s unset; the picker key would report missing wiring", name)
		}
	}
}

// sessionAdopter is the seam between the picker and the registry, and it has to
// hand back the row that was written: the branch is filled in there and nowhere
// else, so a picker fed the pick it sent would start a session with the wrong
// diff base (#122).
func TestSessionAdopter_ReturnsTheRowTheRegistryWrote_issue122(t *testing.T) {
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{"/p/omatty": "/p/omatty"}, Branch: "fix/parser"}
	if _, err := registry.AddProject(store, git, "/p/omatty"); err != nil {
		t.Fatal(err)
	}

	got := sessionAdopter(store, git)("omatty", []ui.SessionProposal{
		{ID: "abc-123", Title: "fix the parser", Dir: "/p/omatty/.omatty/wt/fix"},
	})

	if len(got) != 1 {
		t.Fatalf("adopted %d sessions, want 1", len(got))
	}
	if got[0].Err != nil {
		t.Fatalf("Err = %v, want nil", got[0].Err)
	}
	if got[0].Session.Branch != "fix/parser" {
		t.Errorf("Branch = %q, want the branch the registry recorded for the worktree", got[0].Session.Branch)
	}
}
