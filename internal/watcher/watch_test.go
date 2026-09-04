package watcher

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// shortHome is a HOME short enough for a unix socket path; macOS caps
// sun_path near 104 bytes and t.TempDir() can exceed it.
func shortHome(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "om")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.MkdirAll(filepath.Join(dir, ".omatty"), 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func twoSessions() []registry.Session {
	return []registry.Session{
		{ID: "s1", Project: "p", Title: "one", Dir: "/p"},
		{ID: "s2", Project: "p", Title: "two", Dir: "/p"},
	}
}

func TestStart_OneTailerPerSessionAndAddGrowsIt_issue77(t *testing.T) {
	w := Start(shortHome(t), twoSessions(), time.Now)
	defer w.Close()

	if len(w.tailers) != 2 {
		t.Fatalf("%d tailers for 2 sessions, want 2", len(w.tailers))
	}
	w.Add(registry.Session{ID: "s3", Project: "p", Title: "three", Dir: "/p"})
	if len(w.tailers) != 3 {
		t.Errorf("%d tailers after Add, want 3", len(w.tailers))
	}
}

func TestStart_CloseStopsEveryTailer_issue77(t *testing.T) {
	w := Start(shortHome(t), twoSessions(), time.Now)

	w.Close()

	for _, tl := range w.tailers {
		select {
		case <-tl.Done():
		case <-time.After(2 * time.Second):
			t.Fatal("a tailer is still running after Close")
		}
	}
}

// Archiving a session must stop its tailer and only its tailer (#40).
// Otherwise the goroutine keeps stat-ing a path that no longer exists once a
// second, holding its ring of entries, until omatty quits.
func TestWatch_RemoveStopsOnlyThatSessionsTailer_issue40(t *testing.T) {
	w := Start(shortHome(t), twoSessions(), time.Now)
	defer w.Close()
	doomed, survivor := w.tailers["s1"], w.tailers["s2"]

	w.Remove("s1")

	select {
	case <-doomed.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the removed session's tailer is still running")
	}
	if _, ok := w.tailers["s1"]; ok {
		t.Error("the removed session is still in the tailer map")
	}
	select {
	case <-survivor.Done():
		t.Fatal("Remove stopped a tailer it was not asked to stop")
	default:
	}
}

// Remove on an id that was never added must not panic or disturb the rest: the
// model calls it for every archive, including one whose tailer never started.
func TestWatch_RemoveOfAnUnknownSessionIsANoOp_issue40(t *testing.T) {
	w := Start(shortHome(t), twoSessions(), time.Now)
	defer w.Close()

	w.Remove("ghost")

	if len(w.tailers) != 2 {
		t.Errorf("%d tailers after removing an unknown id, want 2", len(w.tailers))
	}
}

// Add twice for one id would otherwise leak the first tailer: nothing else
// holds a reference, so it would poll forever (#40).
func TestWatch_AddingTheSameSessionTwiceStopsTheDisplacedTailer_issue40(t *testing.T) {
	w := Start(shortHome(t), twoSessions(), time.Now)
	defer w.Close()
	first := w.tailers["s1"]

	w.Add(registry.Session{ID: "s1", Project: "p", Title: "one again", Dir: "/p"})

	select {
	case <-first.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the displaced tailer is still running")
	}
	if len(w.tailers) != 2 {
		t.Errorf("%d tailers after re-adding s1, want 2", len(w.tailers))
	}
}

func TestStart_ListensOnTheHookSocket_issue77(t *testing.T) {
	home := shortHome(t)
	w := Start(home, twoSessions(), time.Now)
	defer w.Close()

	c, err := net.Dial("unix", paths.HookSocket(home))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fmt.Fprintf(c, "%s\n", `{"session_id":"s1","hook_event_name":"PermissionRequest"}`)
	_ = c.Close()

	select {
	case ev := <-w.Events():
		if ev.SessionID != "s1" || ev.Kind != PermissionRequested {
			t.Errorf("got %+v, want s1 PermissionRequested", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event from the hook socket")
	}
}

// Regression, issue #49: a socket that cannot bind must degrade to
// tailer-only, never fail the start.
func TestStart_DegradesToTailerOnlyWhenTheSocketCannotBind_issue49(t *testing.T) {
	// A HOME long enough that the socket path exceeds the macOS sun_path cap.
	home := filepath.Join(t.TempDir(), strings.Repeat("x", 120))
	sess := registry.Session{ID: "s1", Project: "p", Title: "one", Dir: "/p"}
	w := Start(home, []registry.Session{sess}, time.Now)
	defer w.Close()
	if w.listener != nil {
		t.Fatal("precondition: the listener bound on an over-long path")
	}

	transcript := paths.Transcript(home, sess.Dir, sess.ID)
	if err := os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(transcript, []byte(`{"type":"user","timestamp":"2026-09-02T12:00:01Z","message":{"role":"user","content":"hi"}}`+"\n"), 0o600)
	w.tailers["s1"].Poll()

	select {
	case ev := <-w.Events():
		if ev.SessionID != "s1" || ev.Kind != PromptSubmitted {
			t.Errorf("got %+v, want s1 PromptSubmitted from the tailer", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the tailer produced nothing; status is dead without the socket")
	}
}
