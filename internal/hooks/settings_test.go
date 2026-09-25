package hooks_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/hooks"
)

// statusEvents is what the listener asks for; every one must be registered,
// each running the omatty binary's hook subcommand.
var statusEvents = []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
	"PermissionRequest", "Notification", "Stop", "SessionEnd"}

func TestRender_RegistersEveryStatusEvent_issue17(t *testing.T) {
	out, err := hooks.Render("/Users/w/go/bin/omatty", statusEvents)
	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}

	var parsed struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Type    string `json:"type"`
				Command string `json:"command"`
				Timeout int    `json:"timeout"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("Render() is not valid JSON: %v\n%s", err, out)
	}
	want := statusEvents
	for _, event := range want {
		groups, ok := parsed.Hooks[event]
		if !ok || len(groups) == 0 || len(groups[0].Hooks) == 0 {
			t.Errorf("event %q has no hook", event)
			continue
		}
		checkHook(t, event, groups[0].Hooks[0].Type, groups[0].Hooks[0].Command, groups[0].Hooks[0].Timeout)
	}
	if len(parsed.Hooks) != len(want) {
		t.Errorf("registered %d events, want exactly %d", len(parsed.Hooks), len(want))
	}
}

// wantCommand is the whole line claude runs, written out literally rather than
// built the way Render builds it: a test that composes its own expectation
// from the same pieces asserts the code against itself and would follow any
// change silently.
//
// The trailing guard is #380's: a binary that has been reinstalled elsewhere
// is gone from a running session's --settings, and without the guard sh exited
// 127 onto stderr for every hook event. What #17 and #56 won is still asserted
// here - the path is absolute and quoted, and the timeout is 5.
const wantCommand = "'/Users/w/go/bin/omatty' hook 2>/dev/null || true"

func checkHook(t *testing.T, event, typ, command string, timeout int) {
	t.Helper()
	if typ != "command" || command != wantCommand || timeout != 5 {
		t.Errorf("event %q hook = {%q %q %d}, want command %q timeout 5",
			event, typ, command, timeout, wantCommand)
	}
}

func TestRender_UsesTheAbsoluteBinaryPath_issue17(t *testing.T) {
	out, _ := hooks.Render("/opt/homebrew/bin/omatty", statusEvents)
	// Not anchored on the JSON string's closing quote: #380 appended a guard
	// after `hook`, and what this test is about is the path being absolute and
	// quoted, which is still exactly what it asserts.
	if !strings.Contains(string(out), `'/opt/homebrew/bin/omatty' hook`) {
		t.Errorf("Render did not use the absolute binary path:\n%s", out)
	}
}

// Regression, issue #56: claude runs a command hook through a shell, so an
// unquoted install path with a space split into a command that did not exist
// and every hook failed with 127.
func TestRender_QuotesTheBinaryPathForTheShell_issue56(t *testing.T) {
	out, err := hooks.Render(`/Users/w/My Tools/it's $HOME/omatty`, statusEvents)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}

	got := parsed.Hooks["Stop"][0].Hooks[0].Command
	// The path is still one quoted argument; #380's guard follows it.
	want := `'/Users/w/My Tools/it'\''s $HOME/omatty' hook 2>/dev/null || true`
	if got != want {
		t.Errorf("command = %q, want %q", got, want)
	}
}
