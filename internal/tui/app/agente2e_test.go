package app_test

import (
	"encoding/json"
	"errors"
	"github.com/WilsonSousajr/omatty/internal/pubsub"
	"github.com/charmbracelet/x/ansi"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/infra/transcript"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/service/status"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// toyAdapter reads testdata/fake-agent's toy shape: one JSON object a line,
// {"t":"prompt"|"end","at":RFC3339} - a store unlike claude's on purpose, so
// a status that reaches the card can only have come through this parser.
type toyAdapter struct{}

func (toyAdapter) ParseEntry(line []byte) (dstatus.Entry, bool) {
	var rec struct {
		T  string    `json:"t"`
		At time.Time `json:"at"`
	}
	if json.Unmarshal(line, &rec) != nil {
		return dstatus.Entry{}, false
	}
	switch rec.T {
	case "prompt":
		return dstatus.Entry{Type: "user", At: rec.At, UserIsPrompt: true}, true
	case "end":
		return dstatus.Entry{Type: "assistant", At: rec.At, StopReason: "end_turn"}, true
	}
	return dstatus.Entry{}, false
}

func (toyAdapter) DeriveKind(entries []dstatus.Entry) (dstatus.Kind, time.Time, bool) {
	if len(entries) == 0 {
		return 0, time.Time{}, false
	}
	last := entries[len(entries)-1]
	if last.Type == "user" {
		return dstatus.PromptSubmitted, last.At, true
	}
	return dstatus.TurnEnded, last.At, true
}

func (toyAdapter) KindOf(dstatus.HookPayload) (dstatus.Kind, bool) { return 0, false }

// toyProfile is the Transcript-tier toy, registered in a catalog only here.
func toyProfile(bin string) agent.Profile {
	return agent.Profile{
		Name: "toy", DefaultBin: bin,
		Caps: agent.Caps{Identity: agent.Assigned, Status: agent.StatusTranscript, TurnBoundary: true},
		Command: func(b, id, _ string, _ bool, _ string) []string {
			return []string{b, "--shape", "toy", "--session-id", id}
		},
		TranscriptPath: func(home, _, id string) string { return filepath.Join(home, ".toy", id+".jsonl") },
		Status:         toyAdapter{},
	}
}

// Done-when of #527: a second shape drives the whole stack end to end - the
// real launcher, a real PTY running testdata/fake-agent, the real watcher
// tailing the toy's own store through the toy's own parser, and the model's
// own command loop - and its card shows the turn the toy scripted.
func TestE2E_ATranscriptTierToyReportsThroughTheWholeStack_issue527(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	bin, err := filepath.Abs("../../../testdata/fake-agent")
	if err != nil {
		t.Fatal(err)
	}
	agents, err := agent.NewCatalog(agent.Profile{Name: "claude", Command: agent.ClaudeCommand}, toyProfile(bin))
	if err != nil {
		t.Fatal(err)
	}
	st := session.State{
		Projects: []session.Project{{Name: "p", Root: home}},
		Sessions: []session.Session{{ID: "toy-1", Project: "p", Title: "toy one", Dir: home, Agent: "toy"}},
	}
	terms := app.StartTerminals(st, map[string]bool{"toy-1": true}, sessions.NewLauncher(agents, home, &detach.Plain{}),
		terminal.Start, 100, 24, app.DefaultLeader)
	defer app.CloseTerminals(terms)
	w := status.Start(status.WatchDeps{Home: home, Clock: time.Now, Agents: agents,
		OpenTranscript: func(p string) status.Transcript { return transcript.NewReader(p) },
		ListenHooks: func(string, chan<- dstatus.HookPayload) (io.Closer, error) {
			return nil, errors.New("the toy takes no hooks")
		},
	}, st.Sessions)
	defer w.Close()
	m := app.NewModel(app.Deps{State: st, Terms: terms, Create: noCreate, Start: noStart,
		AgentCaps: func(name string) (agent.Caps, bool) {
			p, err := agents.Lookup(name)
			return p.Caps, err == nil
		}})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	drive(t, m, "fake-toy", 10*time.Second)
	got := ansi.Strip(feedEvents(t, m, w.Subscribe(t.Context()), "done", 10*time.Second))

	for _, want := range []string{"fake-toy session=toy-1", "done", "toy · transcript"} {
		if !strings.Contains(got, want) {
			t.Errorf("the screen lacks %q:\n%s", want, got)
		}
	}
}

// feedEvents hands the watcher's events to the model as the program's own
// loop would, until want is on screen. Not through drive: drive abandons a
// command that blocks past its timeout, and an abandoned wait for an event
// would swallow the one TurnEnded the tailer sends.
func feedEvents(t *testing.T, m *app.Model, events <-chan pubsub.Event[dstatus.Event], want string, deadline time.Duration) string {
	t.Helper()
	stop := time.After(deadline)
	for !strings.Contains(m.View().Content, want) {
		select {
		case ev := <-events:
			m.Update(app.StatusMsg(ev.Payload))
		case <-stop:
			return m.View().Content
		}
	}
	return m.View().Content
}
