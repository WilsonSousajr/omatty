package hooks_test

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/hooks"
)

// commandFor pulls one event's hook command string out of the rendered JSON,
// so the test asserts against what claude will actually run through a shell.
func commandFor(t *testing.T, binPath, event string) string {
	t.Helper()
	raw, err := hooks.Render(binPath, []string{event})
	if err != nil {
		t.Fatal(err)
	}
	var settings struct {
		Hooks map[string][]struct {
			Hooks []struct{ Command string } `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		t.Fatal(err)
	}
	groups := settings.Hooks[event]
	if len(groups) != 1 || len(groups[0].Hooks) != 1 {
		t.Fatalf("rendered %d groups for %s, want exactly one hook", len(groups), event)
	}
	return groups[0].Hooks[0].Command
}

// Invariant 11: a hook must never block or fail claude. `omatty hook` keeps
// that promise once it runs, but the command around it did not - claude reads
// --settings once at startup, so a session started by one omatty binary keeps
// that absolute path forever. Reinstalling (go install -> brew, a brew upgrade
// to a new Cellar path, go clean) removes it, and then `sh` exits 127 with
// "No such file or directory" on stderr for every tool call in every running
// session (#380).
func TestRender_aMissingBinaryStillExitsZeroAndSaysNothing_issue380(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "omatty-that-was-uninstalled")

	command := commandFor(t, gone, "PreToolUse")

	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Stdin = strings.NewReader(`{"session_id":"s1","hook_event_name":"PreToolUse"}`)
	var out, errOut strings.Builder
	cmd.Stdout, cmd.Stderr = &out, &errOut

	if err := cmd.Run(); err != nil {
		t.Errorf("a missing binary exited non-zero: %v", err)
	}
	if out.String() != "" {
		t.Errorf("stdout = %q, want empty", out.String())
	}
	if errOut.String() != "" {
		t.Errorf("stderr = %q, want empty - sh's own \"not found\" is what claude showed", errOut.String())
	}
}

// The quoting #56 won must survive: a path with a space is still one argument,
// not two, once the guard is appended.
func TestRender_stillQuotesAPathWithASpace_issue380(t *testing.T) {
	dir := t.TempDir()
	spaced := filepath.Join(dir, "my omatty")

	command := commandFor(t, spaced, "Stop")

	if !strings.Contains(command, "'"+spaced+"'") {
		t.Errorf("command %q does not carry the quoted path", command)
	}
}
