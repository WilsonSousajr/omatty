package keys_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/keys"
)

// Claude Code binds all of these. Invariant 1 says every one reaches the PTY
// untouched while the terminal has focus - guessing at them is what broke the
// earlier LazyVim attempt.
func TestRouter_claudeKeysReachTheTerminal(t *testing.T) {
	for _, key := range []string{"esc", "shift+tab", "ctrl+r", "ctrl+c", "ctrl+b", "enter", "a", "/", "?"} {
		t.Run(key, func(t *testing.T) {
			r := keys.NewRouter("ctrl+o")
			if got := r.Next(key, true); got != keys.ToTerminal {
				t.Errorf("Next(%q, focused) = %v, want ToTerminal", key, got)
			}
		})
	}
}

func TestRouter_leaderIsSwallowedThenNextKeyIsACommand(t *testing.T) {
	r := keys.NewRouter("ctrl+o")

	if got := r.Next("ctrl+o", true); got != keys.Swallow {
		t.Fatalf("Next(leader) = %v, want Swallow", got)
	}
	if !r.Pending() {
		t.Error("Pending() = false after the leader, want true")
	}
	if got := r.Next("j", true); got != keys.ToOmatty {
		t.Errorf("Next(\"j\") after the leader = %v, want ToOmatty", got)
	}
	if r.Pending() {
		t.Error("Pending() = true after the command key, want false")
	}
}

func TestRouter_leaderTwiceIsACommandNotASecondLeader(t *testing.T) {
	r := keys.NewRouter("ctrl+o")
	r.Next("ctrl+o", true)

	if got := r.Next("ctrl+o", true); got != keys.ToOmatty {
		t.Errorf("Next(leader) while pending = %v, want ToOmatty", got)
	}
	if r.Pending() {
		t.Error("Pending() = true after the second leader, want false")
	}
}

func TestRouter_unfocusedTerminalSendsEverythingToOmatty(t *testing.T) {
	r := keys.NewRouter("ctrl+o")

	for _, key := range []string{"esc", "j", "ctrl+o", "enter"} {
		if got := r.Next(key, false); got != keys.ToOmatty {
			t.Errorf("Next(%q, unfocused) = %v, want ToOmatty", key, got)
		}
	}
	if r.Pending() {
		t.Error("Pending() = true after unfocused keys, want false - the leader must not arm while unfocused")
	}
}

// A leader pressed while focused, then focus lost, must not leave the router
// armed to eat the next key as a command.
func TestRouter_pendingSurvivesOnlyWhileFocused(t *testing.T) {
	r := keys.NewRouter("ctrl+o")
	r.Next("ctrl+o", true)

	if got := r.Next("j", false); got != keys.ToOmatty {
		t.Errorf("Next(\"j\", unfocused) = %v, want ToOmatty", got)
	}
}

func TestRoute_StringIsReadableInFailures(t *testing.T) {
	tests := []struct {
		route keys.Route
		want  string
	}{
		{keys.ToTerminal, "ToTerminal"},
		{keys.ToOmatty, "ToOmatty"},
		{keys.Swallow, "Swallow"},
	}
	for _, tt := range tests {
		if got := tt.route.String(); got != tt.want {
			t.Errorf("Route(%d).String() = %q, want %q", tt.route, got, tt.want)
		}
	}
}
