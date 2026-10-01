package sessions_test

import (
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"testing"
	"time"

	statestore "github.com/WilsonSousajr/omatty/internal/infra/store"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

// #332 needs a start time to measure lead time from, and Session had none. It
// is optional with a derivable empty value, which is the argument Base, Agent,
// Conversation, Gate and Carry all already make: Version stays 1 (invariant 9).
func TestCreate_RecordsWhenTheSessionStarted_issue332(t *testing.T) {
	at := time.Date(2026, 9, 26, 10, 0, 0, 0, time.UTC)
	st := session.State{Projects: []session.Project{{Name: "omatty", Root: t.TempDir()}}}
	c := sessions.NewCreator(&FakeGit{}, sessions.CreatorOpts{WorktreeDir: paths.WorktreeDir,
		WorktreeRoot: t.TempDir(),
		Clock:        func() time.Time { return at },
	}, func() string { return "s1" })

	sess, err := c.Create(t.Context(), &st, "omatty", "one", "")
	if err != nil {
		t.Fatal(err)
	}

	if !sess.Started.Equal(at) {
		t.Errorf("Started = %v, want %v", sess.Started, at)
	}
}

// No clock is the wall clock, so nothing has to pass one.
func TestCreate_WithoutAClockUsesTheWallClock_issue332(t *testing.T) {
	st := session.State{Projects: []session.Project{{Name: "omatty", Root: t.TempDir()}}}
	c := sessions.NewCreator(&FakeGit{}, sessions.CreatorOpts{WorktreeDir: paths.WorktreeDir, WorktreeRoot: t.TempDir()},
		func() string { return "s1" })

	sess, err := c.Create(t.Context(), &st, "omatty", "one", "")
	if err != nil {
		t.Fatal(err)
	}

	if sess.Started.IsZero() {
		t.Error("Started is zero with no clock injected; want the wall clock")
	}
}

// The counters #332 asks for: of the gate runs that followed a turn, the share
// that passed. Two ints on Project, omitempty, so a file written before this
// needs no migration.
func TestTallyGateRun_CountsRunsAndTheOnesThatPassed_issue332(t *testing.T) {
	store := storeHolding(t, session.Project{Name: "omatty", Root: "/p/omatty"})

	for _, passed := range []bool{true, false, true} {
		if err := sessions.TallyGateRun(t.Context(), store, "omatty", passed); err != nil {
			t.Fatal(err)
		}
	}

	st, err := store.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Projects[0].GateRuns; got != 3 {
		t.Errorf("GateRuns = %d, want 3", got)
	}
	if got := st.Projects[0].GateFirstPass; got != 2 {
		t.Errorf("GateFirstPass = %d, want 2", got)
	}
}

// A project that is not there is an error naming it, not a silent no-op that
// loses every measurement.
func TestTallyGateRun_ReportsAnUnknownProject_issue332(t *testing.T) {
	store := storeHolding(t)

	if err := sessions.TallyGateRun(t.Context(), store, "ghost", true); err == nil {
		t.Error("tallying against a project that does not exist should fail")
	}
}

// Version stays 1: the new fields are absent from a file that never had them,
// and absent means "nothing measured yet", which is derivable.
func TestTallyGateRun_KeepsTheSchemaAtVersionOne_issue332(t *testing.T) {
	store := storeHolding(t, session.Project{Name: "omatty", Root: "/p/omatty"})

	if err := sessions.TallyGateRun(t.Context(), store, "omatty", true); err != nil {
		t.Fatal(err)
	}

	st, err := store.Load(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if st.Version != 1 {
		t.Errorf("Version = %d, want 1: an optional counter is not a schema break", st.Version)
	}
}

// storeHolding is a store already saved with these projects.
func storeHolding(t *testing.T, projects ...session.Project) *statestore.Store {
	t.Helper()
	store, _ := newStoreAt(t)
	if err := store.Save(t.Context(), session.State{Version: session.Version, Projects: projects}); err != nil {
		t.Fatal(err)
	}
	return store
}

// FakeGit stands in for every git port the session service declares; since
// migration step 5.4 (#653) those take a context, so it no longer matches
// vcs.Git, whose methods do not.
var _ interface {
	sessions.Worktrees
	sessions.RepoRooter
	sessions.SessionBrancher
	sessions.BranchRenamer
} = (*FakeGit)(nil)
