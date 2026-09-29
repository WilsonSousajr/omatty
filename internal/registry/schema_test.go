package registry_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// everyFieldSet is a State in which no field of State, Project or Session is
// its zero value, so every key omitempty would hide is written and pinned.
func everyFieldSet() registry.State {
	return registry.State{
		Version: 1,
		Projects: []registry.Project{{
			Name:          "omatty",
			Root:          "/src/omatty",
			Gate:          []gate.Step{{Name: "test", Run: "go test ./...", Kind: "coverage"}},
			Carry:         []string{".env"},
			GateRuns:      7,
			GateFirstPass: 5,
			Collapsed:     true,
		}},
		Sessions: []registry.Session{{
			ID:           "11111111-2222-4333-8444-555555555555",
			Project:      "omatty",
			Title:        "fix the parser",
			Dir:          "/src/omatty-wt/fix-parser",
			Branch:       "fix/parser",
			Base:         "develop",
			Worktree:     true,
			Agent:        "claude",
			Conversation: "66666666-7777-4888-9999-000000000000",
			Started:      time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC),
		}},
	}
}

// Regression net, issue #620: pins every state.json key before ADR 0001 moves
// these types into domain/session. Invariant 9 - state.json alone must relaunch
// every session - so a renamed tag is a lost session, not a refactor.
func TestState_schemaIsPinned_issue620(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := registry.NewStore(path).Save(everyFieldSet()); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "state.golden.json", raw)
}

// The golden is read as well as written: a file an older omatty saved must
// load back to the same State, field for field.
func TestState_goldenLoadsBackToTheSameState_issue620(t *testing.T) {
	golden, err := os.ReadFile(filepath.Join("testdata", "state.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, golden, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := registry.NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := everyFieldSet(); !reflect.DeepEqual(got, want) {
		t.Errorf("the golden loads as\n%+v\nwant\n%+v", got, want)
	}
}

// A field added later with its zero value in everyFieldSet would be hidden
// by omitempty and escape the golden. This keeps the pin complete.
func TestState_everyFieldIsSetInTheSchemaFixture_issue620(t *testing.T) {
	st := everyFieldSet()
	for _, v := range []reflect.Value{reflect.ValueOf(st), reflect.ValueOf(st.Projects[0]), reflect.ValueOf(st.Sessions[0])} {
		for i := 0; i < v.NumField(); i++ {
			if v.Field(i).IsZero() {
				t.Errorf("%s.%s is zero in everyFieldSet, so the golden does not pin it", v.Type().Name(), v.Type().Field(i).Name)
			}
		}
	}
}
