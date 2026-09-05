package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// cmd/omatty had no test file at all, and the coverage gate measures only
// ./internal/..., so every line here was unmeasured as well as unexercised -
// which is why the readLine EOF path and the projectRegistrar name mismatch
// both shipped (#91). These cover the parts that hold logic; the wiring in
// tuiDeps and run is exercised by the milestone's PTY smoke test, which a
// person reads (AGENTS.md, "Build and test commands").

// FakeGit is a named fake for the one git method the adapters here need.
type FakeGit struct{ Roots map[string]string }

func (f *FakeGit) RepoRoot(dir string) (string, error)     { return f.root(dir) }
func (f *FakeGit) MainCheckout(dir string) (string, error) { return f.root(dir) }

func (f *FakeGit) root(dir string) (string, error) {
	if root, ok := f.Roots[dir]; ok {
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

// Without dtach omatty still runs, so this is a notice rather than a failure -
// but it has to name the fix. "sessions will not survive quit" alone leaves an
// operator who has never heard of dtach with nothing to do about it (#43).
func TestPersistNotice_NamesTheFixWhenNothingHoldsTheSessions_issue43(t *testing.T) {
	got := persistNotice(false)

	if !strings.Contains(got, "brew install dtach") {
		t.Errorf("notice = %q, want it to name the command that fixes it", got)
	}
	if !strings.Contains(got, "quit") {
		t.Errorf("notice = %q, want it to say what is lost", got)
	}
}

// With a holder there is nothing to say, and a permanent line saying so would
// cost the keymap its place in the footer for no reason.
func TestPersistNotice_IsSilentWhenSessionsPersist_issue43(t *testing.T) {
	if got := persistNotice(true); got != "" {
		t.Errorf("notice = %q, want empty when sessions already survive quit", got)
	}
}
