package registry_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

func gateSteps() []gate.Step {
	return []gate.Step{
		{Name: "fmt", Run: "gofmt -l ."},
		{Name: "test", Run: "go test ./... -race"},
		{Name: "cov", Run: "./scripts/check-coverage.sh 90", Kind: "coverage"},
	}
}

func TestSetGate_roundTripsThroughTheStateFile(t *testing.T) {
	store, _ := storeWithProject(t, "omatty")

	if err := registry.SetGate(store, "omatty", gateSteps()); err != nil {
		t.Fatalf("SetGate() error = %v", err)
	}

	st, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got := st.Projects[0].Gate
	if len(got) != 3 {
		t.Fatalf("len(Gate) = %d, want 3", len(got))
	}
	if got[2].Kind != "coverage" || got[2].Run != "./scripts/check-coverage.sh 90" {
		t.Errorf("Gate[2] = %+v, want the coverage step intact", got[2])
	}
}

// Invariant 9: a file written before M9 must load and still relaunch every
// session. A nil gate is not missing data, it is "not configured yet" - which
// is exactly what the detector is for - so it needs no migration and no
// version bump, the argument Agent and Base already carry.
func TestLoad_aFileWrittenBeforeGatesExisted_loadsWithNoGate(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	old := `{"version":1,"projects":[{"name":"omatty","root":"/tmp/omatty"}],` +
		`"sessions":[{"id":"u1","project":"omatty","title":"t","dir":"/tmp/omatty","branch":"main","worktree":false}]}`
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	st, err := registry.NewStore(path).Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if st.Version != registry.Version {
		t.Errorf("Version = %d, want %d unchanged", st.Version, registry.Version)
	}
	if st.Projects[0].Gate != nil {
		t.Errorf("Gate = %v, want nil for a file written before gates", st.Projects[0].Gate)
	}
	if len(st.Sessions) != 1 || st.Sessions[0].ID != "u1" {
		t.Error("the session did not survive the load; invariant 9 is broken")
	}
}

// A project with no gate must not write an empty "gate" key. The file is read
// by people and diffed in review; a key that means nothing is noise.
func TestSave_aProjectWithNoGate_omitsTheKey(t *testing.T) {
	store, path := storeWithProject(t, "omatty")

	st, _ := store.Load()
	if err := store.Save(st); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading state: %v", err)
	}
	if strings.Contains(string(b), "gate") {
		t.Errorf("state file mentions a gate for a project that has none:\n%s", b)
	}
}

func TestClearGate_removesIt(t *testing.T) {
	store, _ := storeWithProject(t, "omatty")
	if err := registry.SetGate(store, "omatty", gateSteps()); err != nil {
		t.Fatalf("SetGate() error = %v", err)
	}

	if err := registry.ClearGate(store, "omatty"); err != nil {
		t.Fatalf("ClearGate() error = %v", err)
	}

	st, _ := store.Load()
	if st.Projects[0].Gate != nil {
		t.Errorf("Gate = %v, want nil after ClearGate", st.Projects[0].Gate)
	}
}

func TestSetGate_unknownProject_isAnErrorNamingIt(t *testing.T) {
	store, _ := storeWithProject(t, "omatty")

	err := registry.SetGate(store, "not-a-project", gateSteps())

	if err == nil {
		t.Fatal("SetGate() error = nil, want an error")
	}
	if !strings.Contains(err.Error(), "not-a-project") {
		t.Errorf("error %q does not name the unknown project", err)
	}
}

func TestClearGate_unknownProject_isAnErrorNamingIt(t *testing.T) {
	store, _ := storeWithProject(t, "omatty")

	err := registry.ClearGate(store, "not-a-project")

	if err == nil || !strings.Contains(err.Error(), "not-a-project") {
		t.Errorf("ClearGate() error = %v, want one naming the unknown project", err)
	}
}

// The gate is persisted as the same typed value the runner takes, so nothing
// re-parses it on the way out.
func TestSetGate_persistsTheDeclaredShape(t *testing.T) {
	store, path := storeWithProject(t, "omatty")
	if err := registry.SetGate(store, "omatty", gateSteps()); err != nil {
		t.Fatalf("SetGate() error = %v", err)
	}

	b, _ := os.ReadFile(path)
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("state file is not JSON: %v", err)
	}
	// The store writes indented JSON, so assert on the decoded value rather
	// than on spacing that formatting could change.
	steps := raw["projects"].([]any)[0].(map[string]any)["gate"].([]any)
	if kind := steps[2].(map[string]any)["kind"]; kind != "coverage" {
		t.Errorf("Gate[2].kind = %v, want \"coverage\"", kind)
	}
	if _, present := steps[0].(map[string]any)["kind"]; present {
		t.Errorf("an ordinary step wrote a kind key; omitempty should drop it:\n%s", b)
	}
}

// storeWithProject is a store holding one registered project, which every
// gate command needs before it can do anything.
func storeWithProject(t *testing.T, name string) (*registry.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state.json")
	store := registry.NewStore(path)
	st := registry.State{
		Version:  registry.Version,
		Projects: []registry.Project{{Name: name, Root: filepath.Join("/tmp", name)}},
	}
	if err := store.Save(st); err != nil {
		t.Fatalf("setup: %v", err)
	}
	return store, path
}

// A corrupt state file must surface as an error from the command the operator
// ran, not as a silent no-op that leaves them believing the gate was set.
func TestSetGate_unreadableState_isAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("not json at all"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	err := registry.SetGate(registry.NewStore(path), "omatty", gateSteps())

	if err == nil {
		t.Fatal("SetGate() error = nil, want the load failure surfaced")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the state file", err)
	}
}
