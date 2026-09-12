package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// gateFixture is a registered Go project, which is the shape Detect
// recognises, plus a store to put its gate in.
func gateFixture(t *testing.T) (*registry.Store, string) {
	t.Helper()
	home := t.TempDir()
	repo := filepath.Join(home, "omatty")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{repo: repo}}
	if _, err := registry.AddProject(store, git, repo); err != nil {
		t.Fatal(err)
	}
	return store, repo
}

func configuredGate(t *testing.T, store *registry.Store) []gate.Step {
	t.Helper()
	st, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	return st.Projects[0].Gate
}

// Detection proposes; confirming is a separate act. An empty answer must leave
// the registry exactly as it was - this is the boundary that stops a cloned
// repository getting a command run because omatty looked at it.
func TestGateCommand_proposesAndWritesNothingWithoutConfirmation(t *testing.T) {
	store, _ := gateFixture(t)

	if err := gateCommand(store, []string{"omatty"}, strings.NewReader("\n")); err != nil {
		t.Fatalf("gateCommand() error = %v", err)
	}

	if got := configuredGate(t, store); got != nil {
		t.Errorf("gate = %v, want nothing written for an empty answer", got)
	}
}

func TestGateCommand_confirmingWritesTheProposal(t *testing.T) {
	store, _ := gateFixture(t)

	if err := gateCommand(store, []string{"omatty"}, strings.NewReader("y\n")); err != nil {
		t.Fatalf("gateCommand() error = %v", err)
	}

	got := configuredGate(t, store)
	if len(got) != 3 {
		t.Fatalf("gate = %v, want the three steps Detect proposes for a Go module", got)
	}
	if got[0].Run != "gofmt -l ." || got[2].Run != "go test ./... -race" {
		t.Errorf("gate = %+v, want Detect's proposal intact", got)
	}
}

// --detect is the read-only form: it must print and never write, whatever the
// operator types.
func TestGateCommand_detectFlagNeverWrites(t *testing.T) {
	store, _ := gateFixture(t)

	if err := gateCommand(store, []string{"omatty", "--detect"}, strings.NewReader("y\ny\n")); err != nil {
		t.Fatalf("gateCommand() error = %v", err)
	}

	if got := configuredGate(t, store); got != nil {
		t.Errorf("gate = %v, want --detect to write nothing", got)
	}
}

// --set is the headless form, for scripts and for the smoke harness: it writes
// without asking.
func TestGateCommand_setFlagWritesWithoutAsking(t *testing.T) {
	store, _ := gateFixture(t)

	if err := gateCommand(store, []string{"omatty", "--set"}, strings.NewReader("")); err != nil {
		t.Fatalf("gateCommand() error = %v", err)
	}

	if got := configuredGate(t, store); len(got) != 3 {
		t.Errorf("gate = %v, want --set to write the proposal", got)
	}
}

func TestGateCommand_clearForgetsTheGate(t *testing.T) {
	store, _ := gateFixture(t)
	if err := gateCommand(store, []string{"omatty", "--set"}, strings.NewReader("")); err != nil {
		t.Fatal(err)
	}

	if err := gateCommand(store, []string{"omatty", "--clear"}, strings.NewReader("")); err != nil {
		t.Fatalf("gateCommand() error = %v", err)
	}

	if got := configuredGate(t, store); got != nil {
		t.Errorf("gate = %v, want nil after --clear", got)
	}
}

// A project already carrying a gate is shown it rather than re-proposed one:
// the common case is "what is this checked by?", not "replace it".
func TestGateCommand_aConfiguredProjectIsShownItsGate(t *testing.T) {
	store, _ := gateFixture(t)
	if err := registry.SetGate(store, "omatty", []gate.Step{{Name: "only", Run: "make check"}}); err != nil {
		t.Fatal(err)
	}

	if err := gateCommand(store, []string{"omatty"}, strings.NewReader("")); err != nil {
		t.Fatalf("gateCommand() error = %v", err)
	}

	got := configuredGate(t, store)
	if len(got) != 1 || got[0].Run != "make check" {
		t.Errorf("gate = %+v, want the configured one untouched", got)
	}
}

func TestGateCommand_unknownProject_isAnErrorNamingIt(t *testing.T) {
	store, _ := gateFixture(t)

	err := gateCommand(store, []string{"not-a-project"}, strings.NewReader(""))

	if err == nil || !strings.Contains(err.Error(), "not-a-project") {
		t.Errorf("gateCommand() error = %v, want one naming the unknown project", err)
	}
}

func TestGateCommand_noProjectArgument_saysWhatItWants(t *testing.T) {
	store, _ := gateFixture(t)

	err := gateCommand(store, nil, strings.NewReader(""))

	if err == nil || !strings.Contains(err.Error(), "<project>") {
		t.Errorf("gateCommand() error = %v, want it to state the expected argument", err)
	}
}

// A checkout omatty recognises nothing in is not an error - there is simply
// nothing to propose, and saying so beats an empty prompt.
func TestGateCommand_nothingRecognised_saysSoAndSucceeds(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "plain")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	store := storeIn(t)
	git := &FakeGit{Roots: map[string]string{repo: repo}}
	if _, err := registry.AddProject(store, git, repo); err != nil {
		t.Fatal(err)
	}

	if err := gateCommand(store, []string{"plain"}, strings.NewReader("y\n")); err != nil {
		t.Errorf("gateCommand() error = %v, want nil", err)
	}
	if got := configuredGate(t, store); got != nil {
		t.Errorf("gate = %v, want nothing proposed", got)
	}
}

// Every path says what it did. A silent success is indistinguishable from a
// command that did nothing at all.
func TestGateCommand_everyPathReportsWhatItDid(t *testing.T) {
	store, _ := gateFixture(t)
	cases := [][]string{
		{"omatty"},
		{"omatty", "--detect"},
		{"omatty", "--set"},
		{"omatty", "--clear"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out := captureReport(t, func() {
				if err := gateCommand(store, args, strings.NewReader("\n")); err != nil {
					t.Fatalf("gateCommand(%v) error = %v", args, err)
				}
			})
			if strings.TrimSpace(out) == "" {
				t.Errorf("gateCommand(%v) printed nothing", args)
			}
		})
	}
}

// captureReport collects what report() wrote while fn ran.
func captureReport(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	return <-done
}
