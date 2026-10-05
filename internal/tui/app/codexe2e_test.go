package app_test

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/infra/fsread"
	"github.com/WilsonSousajr/omatty/internal/infra/transcript"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/service/status"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// codexFakeProfile is cmd's codex profile with testdata/fake-agent as the
// binary: codex's own command template, rollout locator and status adapter,
// with --shape codex put in front so the fake knows whom to play.
func codexFakeProfile(bin, store string) agent.Profile {
	return agent.Profile{
		Name: "codex", DefaultBin: bin,
		Caps: agent.Caps{Identity: agent.Reported, Status: agent.StatusTranscript, Resume: true, TurnBoundary: true},
		Command: func(b, id, dir string, resume bool, hooks string) []string {
			argv := agent.CodexCommand(b, id, dir, resume, hooks)
			return append([]string{argv[0], "--shape", "codex"}, argv[1:]...)
		},
		TranscriptPath: func(_, _, id string) string {
			path, _ := fsread.CodexRollout(store, id)
			return path
		},
		Status: status.CodexAdapter(),
	}
}

// A codex session bound to its conversation - as it is once its
// SessionStart re-bound the row, or after a crash (invariant 9) - is
// resumed by id, writes codex's own rollout under a date directory omatty
// cannot predict, and its turn reaches the card through codex's adapter:
// the real launcher, a real PTY, the real watcher (#152, #527).
func TestE2E_ACodexSessionReportsThroughTheWholeStack_issue152(t *testing.T) {
	home := t.TempDir()
	store := filepath.Join(home, "codex-store")
	t.Setenv("HOME", home)
	t.Setenv("CODEX_HOME", store)
	bin, err := filepath.Abs("../../../testdata/fake-agent")
	if err != nil {
		t.Fatal(err)
	}
	// The conversation an earlier run wrote, under a date omatty cannot predict.
	earlier := filepath.Join(store, "sessions", "2026", "09", "30", "rollout-2026-09-30T08-00-00-c0dex-1.jsonl")
	if err := os.MkdirAll(filepath.Dir(earlier), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(earlier, []byte(`{"timestamp":"2026-09-30T08:00:00.000Z","type":"session_meta","payload":{"id":"c0dex-1"}}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	agents, err := agent.NewCatalog(agent.Profile{Name: "claude", Command: agent.ClaudeCommand}, codexFakeProfile(bin, store))
	if err != nil {
		t.Fatal(err)
	}
	st := session.State{
		Projects: []session.Project{{Name: "p", Root: home}},
		Sessions: []session.Session{{ID: "pane-1", Project: "p", Title: "codex one", Dir: home, Agent: "codex", Conversation: "c0dex-1"}},
	}
	terms := app.StartTerminals(st, map[string]bool{"pane-1": true}, sessions.NewLauncher(agents, home, &detach.Plain{}),
		terminal.Start, 100, 24, app.DefaultLeader)
	defer app.CloseTerminals(terms)
	w := status.Start(status.WatchDeps{Home: home, Clock: time.Now, Agents: agents,
		OpenTranscript: func(p string) status.Transcript { return transcript.NewReader(p) },
		ListenHooks: func(string, chan<- dstatus.HookPayload) (io.Closer, error) {
			return nil, errors.New("this test reads the transcript alone")
		},
	}, st.Sessions)
	defer w.Close()
	m := app.NewModel(app.Deps{State: st, Terms: terms, Create: noCreate, Start: noStart,
		AgentCaps: func(name string) (agent.Caps, bool) {
			p, err := agents.Lookup(name)
			return p.Caps, err == nil
		}})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	drive(t, m, "fake-codex", 10*time.Second)
	got := ansi.Strip(feedEvents(t, m, w.Subscribe(t.Context()), "done", 10*time.Second))

	for _, want := range []string{"fake-codex session=c0dex-1", "done", "codex · transcript"} {
		if !strings.Contains(got, want) {
			t.Errorf("the screen lacks %q:\n%s", want, got)
		}
	}
}
