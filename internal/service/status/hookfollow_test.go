package status_test

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/transcript"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

type nopCloser struct{}

func (nopCloser) Close() error { return nil }

// hookFed starts a Watch whose hook server is a channel the test holds, so a
// payload can be handed over exactly as internal/infra/hookserver would hand
// it (step 5.2d, #653), and returns the first event that comes out.
func hookFed(t *testing.T, adapter dstatus.Adapter, clock func() time.Time, p dstatus.HookPayload) dstatus.Event {
	t.Helper()
	var sink chan<- dstatus.HookPayload
	w := status.Start(status.WatchDeps{
		Home: t.TempDir(), Clock: clock, Adapter: adapter,
		TranscriptPath: func(_, _, _ string) string { return os.DevNull },
		OpenTranscript: func(p string) status.Transcript { return transcript.NewReader(p) },
		ListenHooks: func(_ string, s chan<- dstatus.HookPayload) (io.Closer, error) {
			sink = s
			return nopCloser{}, nil
		},
	}, []session.Session{})
	t.Cleanup(w.Close)
	events := w.Subscribe(t.Context())
	sink <- p
	select {
	case e := <-events:
		return e.Payload
	case <-time.After(3 * time.Second):
		t.Fatal("no event came out of the hook follower")
		return dstatus.Event{}
	}
}

// Each payload becomes an event stamped with the injected clock, so it
// compares like-for-like with tailer timestamps (#18).
func TestHooks_aPayloadBecomesAnEventStampedNow_issue18(t *testing.T) {
	fixed := time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)
	ev := hookFed(t, status.ClaudeAdapter(), func() time.Time { return fixed },
		dstatus.HookPayload{SessionID: "abc", HookEventName: "PreToolUse", ToolName: "Bash"})
	if ev.SessionID != "abc" || ev.Kind != dstatus.ToolStarted || !ev.At.Equal(fixed) {
		t.Errorf("event = %+v, want abc ToolStarted at %v", ev, fixed)
	}
}

// The owning registry id rides on the event, so the UI can find the pane a
// new conversation belongs to without guessing from its directory (#316).
func TestHooks_carryTheOwningSession_issue316(t *testing.T) {
	ev := hookFed(t, status.ClaudeAdapter(), time.Now,
		dstatus.HookPayload{SessionID: "new", HookEventName: "SessionStart", Source: "clear", OmattySession: "row-1"})
	if ev.SessionID != "new" || ev.Owner != "row-1" || ev.Kind != dstatus.SessionRebound {
		t.Errorf("event = %+v, want conversation new, owner row-1, SessionRebound", ev)
	}
}

// A hook's events say they came from a hook: only a hook prompt snaps a turn
// baseline (#311).
func TestHooks_markTheirEventsAsFromAHook_issue311(t *testing.T) {
	ev := hookFed(t, status.ClaudeAdapter(), time.Now,
		dstatus.HookPayload{SessionID: "abc", HookEventName: "UserPromptSubmit"})
	if !ev.Hook || ev.Kind != dstatus.PromptSubmitted {
		t.Errorf("event = %+v, want a PromptSubmitted marked Hook", ev)
	}
}

// Which hook names mean what is the agent's business: the adapter's KindOf
// decides, not a table here (#46).
func TestHooks_mapPayloadsThroughTheAdapter_issue46(t *testing.T) {
	a := &fakeAdapter{Kind: dstatus.TurnEnded, At: time.Now()}
	ev := hookFed(t, a, time.Now, dstatus.HookPayload{SessionID: "s1", HookEventName: "whatever-this-agent-says"})
	if ev.SessionID != "s1" || ev.Kind != dstatus.TurnEnded {
		t.Errorf("event = %+v, want s1 TurnEnded from the adapter", ev)
	}
}
