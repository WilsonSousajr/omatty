package hooks_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/hooks"
)

// Every event the status machine needs must be registered, each running the
// omatty binary's hook subcommand.
func TestRender_RegistersEveryStatusEvent_issue17(t *testing.T) {
	out, err := hooks.Render("/Users/w/go/bin/omatty")
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
	want := []string{"SessionStart", "UserPromptSubmit", "PreToolUse", "PostToolUse",
		"PermissionRequest", "Notification", "Stop", "SessionEnd"}
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

func checkHook(t *testing.T, event, typ, command string, timeout int) {
	t.Helper()
	if typ != "command" || command != "'/Users/w/go/bin/omatty' hook" || timeout != 5 {
		t.Errorf("event %q hook = {%q %q %d}, want command \"'/Users/w/go/bin/omatty' hook\" timeout 5",
			event, typ, command, timeout)
	}
}

func TestRender_UsesTheAbsoluteBinaryPath_issue17(t *testing.T) {
	out, _ := hooks.Render("/opt/homebrew/bin/omatty")
	if !strings.Contains(string(out), `"'/opt/homebrew/bin/omatty' hook"`) {
		t.Errorf("Render did not use the absolute binary path:\n%s", out)
	}
}

// Regression, issue #56: claude runs a command hook through a shell, so an
// unquoted install path with a space split into a command that did not exist
// and every hook failed with 127.
func TestRender_QuotesTheBinaryPathForTheShell_issue56(t *testing.T) {
	out, err := hooks.Render(`/Users/w/My Tools/it's $HOME/omatty`)
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
	want := `'/Users/w/My Tools/it'\''s $HOME/omatty' hook`
	if got != want {
		t.Errorf("command = %q, want %q", got, want)
	}
}
