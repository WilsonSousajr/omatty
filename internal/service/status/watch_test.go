package status

import (
	"fmt"
	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/hookserver"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/infra/transcript"
	"io"
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

// claudeDeps is Start's dependencies for claude, so the tests written before
// the seam (#46) read as they did.
func claudeDeps(home string) WatchDeps {
	return WatchDeps{Home: home, HookSocket: paths.HookSocket(home), Clock: time.Now, Agents: AgentsOf(ClaudeAdapter(), paths.Transcript),
		OpenTranscript: func(p string) Transcript { return transcript.NewReader(p) },
		ListenHooks:    listenHooks}
}

// listenHooks is the real hook server, as cmd wires it (step 5.2d, #653).
func listenHooks(path string, sink chan<- dstatus.HookPayload) (io.Closer, error) {
	l, err := hookserver.Listen(path, sink)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func twoSessions() []session.Session {
	return []session.Session{
		{ID: "s1", Project: "p", Title: "one", Dir: "/p"},
		{ID: "s2", Project: "p", Title: "two", Dir: "/p"},
	}
}

func TestStart_OneTailerPerSessionAndAddGrowsIt_issue77(t *testing.T) {
	w := Start(claudeDeps(shortHome(t)), twoSessions())
	defer w.Close()

	if len(w.tailers) != 2 {
		t.Fatalf("%d tailers for 2 sessions, want 2", len(w.tailers))
	}
	w.Add(session.Session{ID: "s3", Project: "p", Title: "three", Dir: "/p"})
	if len(w.tailers) != 3 {
		t.Errorf("%d tailers after Add, want 3", len(w.tailers))
	}
}

func TestStart_CloseStopsEveryTailer_issue77(t *testing.T) {
	w := Start(claudeDeps(shortHome(t)), twoSessions())

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
	w := Start(claudeDeps(shortHome(t)), twoSessions())
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
	w := Start(claudeDeps(shortHome(t)), twoSessions())
	defer w.Close()

	w.Remove("ghost")

	if len(w.tailers) != 2 {
		t.Errorf("%d tailers after removing an unknown id, want 2", len(w.tailers))
	}
}

// Add twice for one id would otherwise leak the first tailer: nothing else
// holds a reference, so it would poll forever (#40).
func TestWatch_AddingTheSameSessionTwiceStopsTheDisplacedTailer_issue40(t *testing.T) {
	w := Start(claudeDeps(shortHome(t)), twoSessions())
	defer w.Close()
	first := w.tailers["s1"]

	w.Add(session.Session{ID: "s1", Project: "p", Title: "one again", Dir: "/p"})

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
	w := Start(claudeDeps(home), twoSessions())
	defer w.Close()

	c, err := net.Dial("unix", paths.HookSocket(home))
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fmt.Fprintf(c, "%s\n", `{"session_id":"s1","hook_event_name":"PermissionRequest"}`)
	_ = c.Close()

	select {
	case e := <-w.Subscribe(t.Context()):
		ev := e.Payload
		if ev.SessionID != "s1" || ev.Kind != dstatus.PermissionRequested {
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
	sess := session.Session{ID: "s1", Project: "p", Title: "one", Dir: "/p"}
	w := Start(claudeDeps(home), []session.Session{sess})
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
	case e := <-w.Subscribe(t.Context()):
		ev := e.Payload
		if ev.SessionID != "s1" || ev.Kind != dstatus.PromptSubmitted {
			t.Errorf("got %+v, want s1 PromptSubmitted from the tailer", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the tailer produced nothing; status is dead without the socket")
	}
}

// Regression, issue #316: after /clear the tailer kept reading the transcript
// that had stopped growing. A rebound row is tailed at its conversation's
// path, still keyed by its ID so the next Add replaces it.
func TestWatch_AddTailsTheReboundConversation_issue316(t *testing.T) {
	home := shortHome(t)
	deps := claudeDeps(home)
	var opened string
	deps.OpenTranscript = func(p string) Transcript { opened = p; return transcript.NewReader(p) }
	w := Start(deps, twoSessions())
	defer w.Close()
	first := w.tailers["s1"]

	w.Add(session.Session{ID: "s1", Project: "p", Title: "one", Dir: "/p", Conversation: "c1"})

	tl := w.tailers["s1"]
	if want := paths.Transcript(home, "/p", "c1"); opened != want || tl.sessionID != "c1" {
		t.Errorf("tailer = (%q, %q), want (%q, c1)", tl.sessionID, opened, want)
	}
	select {
	case <-first.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the pre-clear tailer is still running")
	}
}

// The UI needs to know when hooks will never arrive: a turn baseline is only
// ever taken by one, so without them the turn view must say so rather than
// diff against a baseline left from another run (#311, #49).
func TestWatch_HooksLiveSaysWhetherTheSocketBound_issue311(t *testing.T) {
	live := Start(claudeDeps(shortHome(t)), nil)
	defer live.Close()
	if !live.HooksLive() {
		t.Error("HooksLive() = false with a bound socket")
	}

	dead := Start(claudeDeps(filepath.Join(shortHome(t), "no", "such", "home")), nil)
	defer dead.Close()
	if dead.HooksLive() {
		t.Error("HooksLive() = true when the socket could not bind")
	}
}

// AgentsOf is a catalog of one claude-named agent reading through a, at path
// (#521). Exported from a test file so the external tests share it. Building it cannot fail: the profile declares no capability it
// lacks the function for.
func AgentsOf(a dstatus.Adapter, path func(home, dir, sessionID string) string) agent.Catalog {
	c, err := agent.NewCatalog(agent.Profile{Name: "claude", Command: agent.ClaudeCommand, TranscriptPath: path, Status: a})
	if err != nil {
		panic(err)
	}
	return c
}

// A session naming an agent the catalog lacks is not tailed: there is no
// parser to read it with, and reading it as claude's is the bug #521 removes.
func TestWatch_UnknownAgentIsNotTailed_issue521(t *testing.T) {
	w := &Watch{deps: WatchDeps{Agents: AgentsOf(ClaudeAdapter(), paths.Transcript)},
		tailers: map[string]*Tailer{}, adapters: map[string]dstatus.Adapter{}}
	w.Add(session.Session{ID: "x1", Dir: "/w", Agent: "codex"})
	if w.tailers["x1"] != nil {
		t.Error("a session naming an unknown agent is tailed, want it left alone")
	}
}

// A hook payload is read by the agent of the session it names, and one
// naming no tailed session by the default agent's (#521).
func TestWatch_AHookIsReadByItsSessionsAgent_issue521(t *testing.T) {
	toy, claude := &recordingAdapter{}, &recordingAdapter{}
	w := &Watch{deps: WatchDeps{Agents: AgentsOf(claude, paths.Transcript)},
		adapters: map[string]dstatus.Adapter{"t1": toy}}
	w.hookAdapter("t1").KindOf(dstatus.HookPayload{})
	w.hookAdapter("unknown").KindOf(dstatus.HookPayload{})
	if toy.Calls != 1 || claude.Calls != 1 {
		t.Errorf("toy read %d, claude read %d; want one each", toy.Calls, claude.Calls)
	}
}

// recordingAdapter counts the hook payloads it is asked to read.
type recordingAdapter struct{ Calls int }

func (*recordingAdapter) ParseEntry([]byte) (dstatus.Entry, bool) { return dstatus.Entry{}, false }
func (*recordingAdapter) DeriveKind([]dstatus.Entry) (dstatus.Kind, time.Time, bool) {
	return 0, time.Time{}, false
}
func (r *recordingAdapter) KindOf(dstatus.HookPayload) (dstatus.Kind, bool) {
	r.Calls++
	return 0, false
}
