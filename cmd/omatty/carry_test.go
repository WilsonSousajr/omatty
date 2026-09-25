package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

func configuredCarry(t *testing.T, store *registry.Store) []string {
	t.Helper()
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return st.Projects[0].Carry
}

// Setting the list is the whole command: a worktree cannot carry what nobody
// named (#309).
func TestCarryCommand_setsAndClearsTheList_issue309(t *testing.T) {
	store, repo := gateFixture(t)
	if err := os.WriteFile(filepath.Join(repo, ".env"), []byte("T=1"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := carryCommand(store, []string{"omatty", ".env"}); err != nil {
		t.Fatalf("carryCommand() error = %v", err)
	}
	if got := configuredCarry(t, store); len(got) != 1 || got[0] != ".env" {
		t.Errorf("carry = %v, want [.env]", got)
	}

	if err := carryCommand(store, []string{"omatty", "--clear"}); err != nil {
		t.Fatalf("carryCommand(--clear) error = %v", err)
	}
	if got := configuredCarry(t, store); got != nil {
		t.Errorf("carry = %v, want nil after --clear", got)
	}
}

// A path that is not in the checkout is still recorded - the operator may be
// about to create it - but saying nothing would let a typo sit in the list
// until a worktree quietly came up short (#309).
func TestCarryCommand_warnsAboutAPathTheCheckoutDoesNotHave_issue309(t *testing.T) {
	store, _ := gateFixture(t)

	out := captureReport(t, func() {
		if err := carryCommand(store, []string{"omatty", "nope.env"}); err != nil {
			t.Fatalf("carryCommand() error = %v", err)
		}
	})

	if !strings.Contains(out, "nope.env") {
		t.Errorf("output did not mention the missing path:\n%s", out)
	}
	if got := configuredCarry(t, store); len(got) != 1 {
		t.Errorf("carry = %v, want the path recorded anyway", got)
	}
}

// The plain form prints what is set, so the list is readable without opening
// state.json.
func TestCarryCommand_printsTheListWithNoPaths_issue309(t *testing.T) {
	store, _ := gateFixture(t)
	if err := registry.SetCarry(store, "omatty", []string{".env", "certs"}); err != nil {
		t.Fatal(err)
	}

	out := captureReport(t, func() {
		if err := carryCommand(store, []string{"omatty"}); err != nil {
			t.Fatalf("carryCommand() error = %v", err)
		}
	})

	for _, want := range []string{".env", "certs"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

// No project named is a usage error, as it is for gate.
func TestCarryCommand_needsAProject_issue309(t *testing.T) {
	store, _ := gateFixture(t)
	if err := carryCommand(store, nil); err == nil {
		t.Error("carryCommand(nil) error = nil, want a usage error")
	}
}
