package sessions_test

import (
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

// Regression, issue #316: after /clear the next start ran
// `claude --resume <pre-clear uuid>` and handed back the conversation frozen
// at the clear. A rebound row resumes its conversation, while the holder is
// still asked under the row's ID - the name its dtach socket already has.
func TestLauncher_ResumesTheReboundConversation_issue316(t *testing.T) {
	home := t.TempDir()
	transcript := paths.Transcript(home, "/w", "after-clear")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h := &fakeHolder{Wrapped: []string{"dtach", "-A", "/s.sock"}}
	l := sessions.NewLauncher(catalogFor(t, claudeProfile(), "claude"), "/h.json", home, h)

	if _, err := l.Launch(session.Session{ID: "row-1", Dir: "/w", Conversation: "after-clear"}); err != nil {
		t.Fatal(err)
	}

	if got := strings.Join(h.GotArgs, " "); !strings.Contains(got, "claude --resume after-clear") {
		t.Errorf("holder was handed %q, want claude --resume after-clear", got)
	}
	if h.GotID != "row-1" {
		t.Errorf("holder was given id %q, want the row's own row-1", h.GotID)
	}
}

// The hook inherits claude's environment, and this variable is how a /clear
// names its pane (#316). It carries the row's ID, which never changes, so it
// stays right across any number of clears.
func TestLauncher_ExportsTheOwningSession_issue316(t *testing.T) {
	l := sessions.NewLauncher(catalogFor(t, claudeProfile(), "claude"), "/h.json", t.TempDir(), &detach.Plain{})

	cmd, err := l.Launch(session.Session{ID: "row-1", Dir: "/w", Conversation: "after-clear"})
	if err != nil {
		t.Fatal(err)
	}

	if want := session.SessionEnv + "=row-1"; !slices.Contains(cmd.Env, want) {
		t.Errorf("claude's environment lacks %q", want)
	}
	if !slices.Contains(cmd.Env, "HOME="+os.Getenv("HOME")) {
		t.Error("claude's environment dropped the inherited HOME")
	}
}

// omatty run inside an omatty pane inherits the outer pane's variable. Left
// in, the inner session's /clear would re-bind the outer pane (#316).
func TestLauncher_ReplacesAnInheritedOwningSession_issue316(t *testing.T) {
	t.Setenv(session.SessionEnv, "outer-pane")
	l := sessions.NewLauncher(catalogFor(t, claudeProfile(), "claude"), "/h.json", t.TempDir(), &detach.Plain{})

	cmd, err := l.Launch(session.Session{ID: "row-1", Dir: "/w"})
	if err != nil {
		t.Fatal(err)
	}

	var owners []string
	for _, kv := range cmd.Env {
		if strings.HasPrefix(kv, session.SessionEnv+"=") {
			owners = append(owners, kv)
		}
	}
	if len(owners) != 1 || owners[0] != session.SessionEnv+"=row-1" {
		t.Errorf("claude's environment names owners %v, want only row-1", owners)
	}
}
