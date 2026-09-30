package registry_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/gate"
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
			Gate:          []gate.Step{{Name: "test", Run: "go test ./...", Kind: "coverage", Profile: "cover.out"}},
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
// by omitempty and escape the golden. The walk recurses into slices of
// structs, because gate.Step's profile lives there and the first version of
// this test, checking only the top level, let it through (#620 review).
func TestState_everyFieldIsSetInTheSchemaFixture_issue620(t *testing.T) {
	for _, path := range zeroFields(reflect.ValueOf(everyFieldSet()), "State") {
		t.Errorf("%s is zero in everyFieldSet, so the golden does not pin it", path)
	}
}

// zeroFields names every zero field under v, descending into structs and the
// struct elements of slices. time.Time is a leaf: its fields are unexported.
func zeroFields(v reflect.Value, path string) []string {
	if v.Kind() == reflect.Slice {
		return zeroSlice(v, path)
	}
	if v.Kind() != reflect.Struct || v.Type() == reflect.TypeOf(time.Time{}) {
		if v.IsZero() {
			return []string{path}
		}
		return nil
	}
	var out []string
	for i := 0; i < v.NumField(); i++ {
		out = append(out, zeroFields(v.Field(i), path+"."+v.Type().Field(i).Name)...)
	}
	return out
}

func zeroSlice(v reflect.Value, path string) []string {
	if v.Len() == 0 {
		return []string{path}
	}
	var out []string
	for i := 0; i < v.Len(); i++ {
		out = append(out, zeroFields(v.Index(i), fmt.Sprintf("%s[%d]", path, i))...)
	}
	return out
}
