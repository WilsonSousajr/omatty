package sessions_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	statestore "github.com/WilsonSousajr/omatty/internal/infra/store"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

func TestSetCollapsed_roundTripsThroughTheStateFile(t *testing.T) {
	store, _ := storeWithProject(t, "omatty")

	if err := sessions.SetCollapsed(store, "omatty", true); err != nil {
		t.Fatalf("SetCollapsed(true) error = %v", err)
	}
	st, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !st.Projects[0].Collapsed {
		t.Fatalf("Collapsed = false after SetCollapsed(true)")
	}

	if err := sessions.SetCollapsed(store, "omatty", false); err != nil {
		t.Fatalf("SetCollapsed(false) error = %v", err)
	}
	st, _ = store.Load()
	if st.Projects[0].Collapsed {
		t.Errorf("Collapsed = true after SetCollapsed(false)")
	}
}

// An unfolded project must leave the key out, so a state file written by a
// build that never folded anything reads exactly as it did (invariant 9).
func TestSetCollapsed_unfolded_omitsTheKey(t *testing.T) {
	store, path := storeWithProject(t, "omatty")
	if err := sessions.SetCollapsed(store, "omatty", false); err != nil {
		t.Fatalf("SetCollapsed(false) error = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	if strings.Contains(string(raw), "collapsed") {
		t.Errorf("state.json = %s, want no collapsed key for an unfolded project", raw)
	}
}

func TestSetCollapsed_unknownProject_namesIt(t *testing.T) {
	store, _ := storeWithProject(t, "omatty")
	err := sessions.SetCollapsed(store, "nope", true)
	if err == nil || !strings.Contains(err.Error(), `"nope"`) {
		t.Fatalf("SetCollapsed(nope) error = %v, want one naming the project", err)
	}
}

func TestSetCollapsed_unreadableState_isAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("not json at all"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := sessions.SetCollapsed(statestore.NewStore(path), "omatty", true); err == nil {
		t.Fatal("SetCollapsed on a corrupt state file = nil error, want one")
	}
}

// A state file written before #505 has no collapsed key and must load as
// every project unfolded, without a migration.
func TestLoad_aProjectWrittenBeforeFolding_loadsUnfolded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	old := `{"version":1,"projects":[{"name":"omatty","root":"/tmp/omatty"}]}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	st, err := statestore.NewStore(path).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if st.Projects[0].Collapsed {
		t.Error("Collapsed = true for a project with no key, want false")
	}
}
