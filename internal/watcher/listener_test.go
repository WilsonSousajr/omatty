package watcher_test

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/hooks"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func shortDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "om")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func TestKindOf_MapsEveryHookEvent(t *testing.T) {
	tests := []struct {
		event, notif string
		want         watcher.Kind
	}{
		{"SessionStart", "", watcher.SessionStarted},
		{"UserPromptSubmit", "", watcher.PromptSubmitted},
		{"PreToolUse", "", watcher.ToolStarted},
		{"PostToolUse", "", watcher.ToolFinished},
		{"PermissionRequest", "", watcher.PermissionRequested},
		{"Notification", "idle_prompt", watcher.Idle},
		{"Notification", "permission_prompt", watcher.PermissionRequested},
		{"Stop", "", watcher.TurnEnded},
		{"SessionEnd", "", watcher.SessionEnded},
	}
	for _, tt := range tests {
		p := hooks.Payload{HookEventName: tt.event, NotificationType: tt.notif}
		got, ok := watcher.KindOf(p)
		if !ok || got != tt.want {
			t.Errorf("KindOf(%s/%s) = (%v, %v), want (%v, true)", tt.event, tt.notif, got, ok, tt.want)
		}
	}
}

func TestKindOf_UnknownEventIsDropped(t *testing.T) {
	if _, ok := watcher.KindOf(hooks.Payload{HookEventName: "PreCompact"}); ok {
		t.Error("KindOf mapped an event omatty does not track")
	}
}

func TestListen_EmitsAnEventPerConnection_issue18(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	fixed := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	sink := make(chan watcher.Event, 4)

	l, err := watcher.Listen(path, sink, func() time.Time { return fixed })
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer func() { _ = l.Close() }()

	dial(t, path, `{"session_id":"abc","hook_event_name":"PreToolUse","tool_name":"Bash"}`)

	select {
	case ev := <-sink:
		if ev.SessionID != "abc" || ev.Kind != watcher.ToolStarted {
			t.Errorf("event = %+v, want session abc, ToolStarted", ev)
		}
		if !ev.At.Equal(fixed) {
			t.Errorf("event time = %v, want the injected clock %v", ev.At, fixed)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event emitted for a connection")
	}
}

func TestListen_RejectsOversizedPayloadButKeepsAccepting_issue18(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	sink := make(chan watcher.Event, 4)
	l, err := watcher.Listen(path, sink, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()

	dialTolerant(path, fmt.Sprintf(`{"session_id":"%s","hook_event_name":"Stop"}`, string(make([]byte, 128<<10))))
	select {
	case ev := <-sink:
		t.Fatalf("an oversized payload produced an event: %+v", ev)
	case <-time.After(300 * time.Millisecond):
	}

	// The listener must still be alive for the next hook.
	dial(t, path, `{"session_id":"ok","hook_event_name":"Stop"}`)
	select {
	case ev := <-sink:
		if ev.SessionID != "ok" {
			t.Errorf("after an oversized payload, got %+v, want the next valid one", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener stopped accepting after an oversized payload")
	}
}

func TestListen_ReplacesAStaleSocketFile_issue18(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := watcher.Listen(path, make(chan watcher.Event, 1), time.Now)
	if err != nil {
		t.Fatalf("Listen() did not replace a stale socket file: %v", err)
	}
	_ = l.Close()
}

func TestListen_SocketIsUserOnly_issue18(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	l, err := watcher.Listen(path, make(chan watcher.Event, 1), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("socket mode = %o, want 0600", perm)
	}
}

// dialTolerant writes without asserting success: an oversized payload makes
// the listener close early, which is the behaviour under test.
func dialTolerant(path, payload string) {
	c, err := net.Dial("unix", path)
	if err != nil {
		return
	}
	defer func() { _ = c.Close() }()
	_, _ = fmt.Fprintf(c, "%s\n", payload)
}

// dialAndWait sends payload and blocks until the listener closes the
// connection, which it does once the line has been handled - a deterministic
// signal that the event was offered, with no sleep.
func dialAndWait(t *testing.T, path, payload string) {
	t.Helper()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if _, err := fmt.Fprintf(c, "%s\n", payload); err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, c) // returns at EOF when the server closes
}

func dial(t *testing.T, path, payload string) {
	t.Helper()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = c.Close() }()
	if _, err := fmt.Fprintf(c, "%s\n", payload); err != nil {
		t.Fatal(err)
	}
}

// Regression, issue #49: a socket that cannot bind (path too long, no
// permission) must be a recoverable error the caller can log and skip, not a
// nil listener that later panics.
func TestListen_UnbindablePathReturnsAnError_issue49(t *testing.T) {
	// A path well over the macOS sun_path cap (~104 bytes).
	long := filepath.Join(shortDir(t), string(make([]byte, 120)))
	_, err := watcher.Listen(long, make(chan watcher.Event, 1), time.Now)
	if err == nil {
		t.Fatal("Listen on an oversized path returned nil error, want a failure the caller can handle")
	}
}

// Regression, issue #67: connections were served inline with no read
// deadline, so one peer that connected and sent nothing parked the accept
// loop and every later hook on the machine was never read.
func TestListen_ASilentPeerDoesNotStarveLaterHooks_issue67(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	sink := make(chan watcher.Event, 4)
	l, err := watcher.Listen(path, sink, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	silent, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = silent.Close() }()

	dial(t, path, `{"session_id":"after","hook_event_name":"Stop"}`)

	select {
	case ev := <-sink:
		if ev.SessionID != "after" {
			t.Errorf("got %+v, want the hook sent after the silent peer", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a silent peer starved the next hook; the accept loop is parked on it")
	}
}

func TestListen_CloseReturnsWithASilentPeerConnected_issue67(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	l, err := watcher.Listen(path, make(chan watcher.Event, 1), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	silent, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = silent.Close() }()

	closed := make(chan struct{})
	go func() { _ = l.Close(); close(closed) }()

	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not return while a silent peer was connected")
	}
}

// A full sink means the UI is behind; the tailer restores the truth within a
// second, so a hook event is dropped and counted rather than blocking the
// listener, which would stall every hook on the machine.
func TestListen_DropsInsteadOfBlockingOnAFullSink_issue67(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	l, err := watcher.Listen(path, make(chan watcher.Event), time.Now) // unbuffered, never read
	if err != nil {
		t.Fatal(err)
	}

	dialAndWait(t, path, `{"session_id":"a","hook_event_name":"Stop"}`)
	dialAndWait(t, path, `{"session_id":"b","hook_event_name":"Stop"}`)
	_ = l.Close()

	if got := l.Dropped(); got != 2 {
		t.Errorf("Dropped() = %d, want 2: both events had nowhere to go", got)
	}
}

// Regression, issue #68: a second omatty unlinked the first one's socket and
// bound its own; the first kept accepting on a nameless inode and never saw
// another hook, with no log line.
func TestListen_RefusesWhenAnotherInstanceIsLive_issue68(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	sink := make(chan watcher.Event, 4)
	first, err := watcher.Listen(path, sink, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Close() }()

	second, err := watcher.Listen(path, make(chan watcher.Event, 1), time.Now)

	if err == nil {
		_ = second.Close()
		t.Fatal("a second Listen on a live socket succeeded, want an error so it degrades to tailer-only")
	}
	dial(t, path, `{"session_id":"still","hook_event_name":"Stop"}`)
	select {
	case ev := <-sink:
		if ev.SessionID != "still" {
			t.Errorf("first instance got %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the first instance lost its socket")
	}
}
