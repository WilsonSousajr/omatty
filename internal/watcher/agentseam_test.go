package watcher_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// The tailer parses every line through the adapter it was given and emits
// the kind the adapter derived, never calling claude's own parser (#46).
func TestTail_ParsesThroughTheAdapter_issue46(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.jsonl")
	if err := os.WriteFile(path, []byte("not json at all\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	a := &fakeAdapter{Kind: watcher.ToolStarted, At: at, Entry: watcher.Entry{Type: "user", At: at}}
	sink := make(chan watcher.Event, 4)

	tl := watcher.Tail("s1", path, sink, time.Now, time.Hour, a)
	defer tl.Close()
	tl.Poll()

	select {
	case ev := <-sink:
		if ev.Kind != watcher.ToolStarted || a.Parsed != 1 {
			t.Errorf("event %+v after %d parses; want the adapter's ToolStarted from one parse", ev, a.Parsed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event: the line was not parsed through the adapter")
	}
}

// The listener maps a payload through the adapter (#46).
func TestListen_MapsHookPayloadsThroughTheAdapter_issue46(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	a := &fakeAdapter{Kind: watcher.PermissionRequested}
	sink := make(chan watcher.Event, 1)
	l, err := watcher.Listen(path, sink, time.Now, a)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	dialAndWait(t, path, `{"session_id":"s1","hook_event_name":"Whatever"}`)

	select {
	case ev := <-sink:
		if ev.Kind != watcher.PermissionRequested || a.Payload.HookEventName != "Whatever" {
			t.Errorf("event %+v, adapter saw %+v; want the adapter's kind for the payload it was given", ev, a.Payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event through the adapter")
	}
}

// Start tails the path the profile names, not ~/.claude (#46).
func TestStart_TailsThePathTheProfileNames_issue46(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "elsewhere.jsonl")
	if err := os.WriteFile(path, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &fakeAdapter{Kind: watcher.TurnEnded, At: time.Now()}
	w := watcher.Start(watcher.WatchDeps{
		Home: home, Clock: time.Now, Adapter: a,
		TranscriptPath: func(_, _, _ string) string { return path },
	}, []registry.Session{{ID: "s1", Dir: "/w"}})
	defer w.Close()

	select {
	case ev := <-w.Events():
		if ev.SessionID != "s1" || ev.Kind != watcher.TurnEnded {
			t.Errorf("event %+v, want s1 TurnEnded from the profile's path", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no event: the watcher did not tail the profile's path")
	}
}
