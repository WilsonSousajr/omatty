package status_test

import (
	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"os"
	"path/filepath"
	"testing"
	"time"

	"errors"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/transcript"
	"github.com/WilsonSousajr/omatty/internal/service/status"
	"io"
)

// The tailer parses every line through the adapter it was given and emits
// the kind the adapter derived, never calling claude's own parser (#46).
func TestTail_ParsesThroughTheAdapter_issue46(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.jsonl")
	if err := os.WriteFile(path, []byte("not json at all\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	a := &fakeAdapter{Kind: dstatus.ToolStarted, At: at, Entry: dstatus.Entry{Type: "user", At: at}}
	sink := make(chan dstatus.Event, 4)

	tl := status.Tail("s1", transcript.NewReader(path), sink, time.Now, time.Hour, a)
	defer tl.Close()
	tl.Poll()

	select {
	case ev := <-sink:
		if ev.Kind != dstatus.ToolStarted || a.Parsed != 1 {
			t.Errorf("event %+v after %d parses; want the adapter's ToolStarted from one parse", ev, a.Parsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event: the line was not parsed through the adapter")
	}
}

// Start tails the path the profile names, not ~/.claude (#46).
func TestStart_TailsThePathTheProfileNames_issue46(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "elsewhere.jsonl")
	if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &fakeAdapter{Kind: dstatus.TurnEnded, At: time.Now()}
	w := status.Start(status.WatchDeps{
		Home: home, Clock: time.Now, Agents: status.AgentsOf(a, func(_, _, _ string) string { return path }),
		OpenTranscript: func(p string) status.Transcript { return transcript.NewReader(p) },
		ListenHooks: func(string, chan<- dstatus.HookPayload) (io.Closer, error) {
			return nil, errors.New("no hook socket in this test")
		},
	}, []session.Session{{ID: "s1", Dir: "/w"}})
	defer w.Close()

	select {
	case e := <-w.Subscribe(t.Context()):
		ev := e.Payload
		if ev.SessionID != "s1" || ev.Kind != dstatus.TurnEnded {
			t.Errorf("event %+v, want s1 TurnEnded from the profile's path", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no event: the watcher did not tail the profile's path")
	}
}

// Regression, #521: the watcher held one adapter and one transcript path for
// the whole app, so a second agent's session was read as claude's. Each
// session now reads its own agent's transcript through its own parser.
func TestWatch_EachSessionReadsThroughItsOwnAgent_issue521(t *testing.T) {
	home := t.TempDir()
	write := func(name string) string {
		path := filepath.Join(home, name)
		if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	claudePath, toyPath := write("claude.jsonl"), write("toy.log")
	at := time.Now()
	agents, err := agent.NewCatalog(
		agent.Profile{Name: "claude", Command: agent.ClaudeCommand, Status: &fakeAdapter{Kind: dstatus.TurnEnded, At: at},
			TranscriptPath: func(_, _, _ string) string { return claudePath }},
		agent.Profile{Name: "toy", Command: agent.ClaudeCommand, Status: &fakeAdapter{Kind: dstatus.ToolStarted, At: at},
			TranscriptPath: func(_, _, _ string) string { return toyPath }})
	if err != nil {
		t.Fatal(err)
	}
	w := status.Start(status.WatchDeps{Home: home, Clock: time.Now, Agents: agents,
		OpenTranscript: func(p string) status.Transcript { return transcript.NewReader(p) },
		ListenHooks: func(string, chan<- dstatus.HookPayload) (io.Closer, error) {
			return nil, errors.New("no hook socket in this test")
		},
	}, []session.Session{{ID: "c1", Dir: "/w"}, {ID: "t1", Dir: "/w", Agent: "toy"}})
	defer w.Close()

	got := firstKindPerSession(t, w, 2)
	if got["c1"] != dstatus.TurnEnded || got["t1"] != dstatus.ToolStarted {
		t.Errorf("kinds = %v, want c1 TurnEnded from claude's parser and t1 ToolStarted from toy's", got)
	}
}

// firstKindPerSession reads events until n sessions have each reported one.
func firstKindPerSession(t *testing.T, w *status.Watch, n int) map[string]dstatus.Kind {
	t.Helper()
	got := map[string]dstatus.Kind{}
	events := w.Subscribe(t.Context())
	deadline := time.After(3 * time.Second)
	for len(got) < n {
		select {
		case e := <-events:
			if _, seen := got[e.Payload.SessionID]; !seen {
				got[e.Payload.SessionID] = e.Payload.Kind
			}
		case <-deadline:
			t.Fatalf("only %v reported before the deadline", got)
		}
	}
	return got
}
