package ui_test

import (
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func TestStartTerminals_OnePerSessionInItsOwnDirectory(t *testing.T) {
	var dirs []string
	factory := func(_, _ int, cmd *exec.Cmd) (termwrap.Terminal, error) {
		dirs = append(dirs, cmd.Dir)
		return termwrap.NewFake(""), nil
	}

	terms, err := ui.StartTerminals(
		twoProjectState(), supervisor.NewLauncher("claude", "/h.json"), factory, 80, 24)

	if err != nil {
		t.Fatalf("StartTerminals() error = %v, want nil", err)
	}
	if len(terms) != 3 {
		t.Errorf("started %d terminals, want 3", len(terms))
	}
	if len(dirs) != 3 {
		t.Errorf("factory called %d times, want 3", len(dirs))
	}
	for _, id := range []string{"s1", "s2", "s3"} {
		if terms[id] == nil {
			t.Errorf("no terminal registered for session %q", id)
		}
	}
}

func TestStartTerminals_FailureNamesTheSession(t *testing.T) {
	factory := func(int, int, *exec.Cmd) (termwrap.Terminal, error) {
		return nil, errors.New("pty exhausted")
	}

	_, err := ui.StartTerminals(
		twoProjectState(), supervisor.NewLauncher("claude", "/h.json"), factory, 80, 24)

	if err == nil {
		t.Fatal("StartTerminals() returned nil after a factory failure, want an error")
	}
	if !strings.Contains(err.Error(), "s1") {
		t.Errorf("error %q does not name the offending session", err)
	}
}

func TestStartTerminals_EmptyRegistryStartsNothing(t *testing.T) {
	called := 0
	factory := func(int, int, *exec.Cmd) (termwrap.Terminal, error) {
		called++
		return termwrap.NewFake(""), nil
	}

	terms, err := ui.StartTerminals(
		emptyState(), supervisor.NewLauncher("claude", "/h.json"), factory, 80, 24)

	if err != nil {
		t.Fatalf("StartTerminals() error = %v, want nil", err)
	}
	if len(terms) != 0 || called != 0 {
		t.Errorf("started %d terminals with %d factory calls, want 0 and 0", len(terms), called)
	}
}
