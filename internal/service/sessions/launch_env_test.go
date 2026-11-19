package sessions_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

// A profile's own environment reaches the agent: claude's names omatty in its
// exit hint through CLAUDE_CODE_RESUME_HINT_COMMAND (#815).
func TestLauncher_SetsTheProfilesEnv_issue815(t *testing.T) {
	p := claudeProfile()
	p.Env = []string{"HINT_UNDER_TEST=omatty"}
	l := sessions.NewLauncher(catalogFor(t, p, "claude", "/h/hooks.json"), t.TempDir(), &detach.Plain{})

	cmd, err := l.Launch(session.Session{ID: "row-1", Dir: "/w"})

	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(cmd.Env, "HINT_UNDER_TEST=omatty") {
		t.Errorf("Env lacks HINT_UNDER_TEST=omatty; got %d entries", len(cmd.Env))
	}
}

// A value inherited from the shell that started omatty is replaced, not
// shadowed: with two entries, which one the agent reads is the libc's choice.
func TestLauncher_ProfileEnvReplacesAnInheritedValue_issue815(t *testing.T) {
	t.Setenv("HINT_UNDER_TEST", "outer")
	p := claudeProfile()
	p.Env = []string{"HINT_UNDER_TEST=omatty"}
	l := sessions.NewLauncher(catalogFor(t, p, "claude", "/h/hooks.json"), t.TempDir(), &detach.Plain{})

	cmd, err := l.Launch(session.Session{ID: "row-1", Dir: "/w"})

	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, kv := range cmd.Env {
		if strings.HasPrefix(kv, "HINT_UNDER_TEST=") {
			got = append(got, kv)
		}
	}
	if len(got) != 1 || got[0] != "HINT_UNDER_TEST=omatty" {
		t.Errorf("HINT_UNDER_TEST entries = %q, want exactly [HINT_UNDER_TEST=omatty]", got)
	}
}
