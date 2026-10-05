package status

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
)

// scanned is a Scanned-identity profile whose store answers locate.
func scanned(locate func(home, dir string, since time.Time) []string) agent.Profile {
	return agent.Profile{Name: "scanny", Command: agent.ClaudeCommand, Locate: locate,
		Caps:           agent.Caps{Identity: agent.Scanned, Status: agent.StatusTranscript},
		TranscriptPath: func(_, _, id string) string { return "/nowhere/" + id },
		Status:         ClaudeAdapter()}
}

// One candidate in the agent's store for the session's directory and start
// binds it: a SessionRebound naming the pane, which the TUI persists as the
// row's Conversation exactly as it does after /clear (#316, #523).
func TestBind_OneCandidateBinds_issue523(t *testing.T) {
	sink := make(chan dstatus.Event, 1)
	b := newBinder(session.Session{ID: "s1", Dir: "/w"}, scanned(func(string, string, time.Time) []string { return []string{"conv-1"} }),
		"/h", sink, time.Now, func(string) bool { return false })

	if !b.Try() {
		t.Fatal("Try() = false with exactly one candidate, want it bound")
	}
	ev := <-sink
	if ev.Kind != dstatus.SessionRebound || ev.Owner != "s1" || ev.SessionID != "conv-1" {
		t.Errorf("event %+v, want SessionRebound of s1 onto conv-1", ev)
	}
}

// Two sessions started together in one directory give two candidates, and a
// guess would show one session's status on the other's card - the bug class
// invariant 2 exists for. It stays unbound and says which (#523).
func TestBind_TwoCandidatesStayUnboundAndAreLogged_issue523(t *testing.T) {
	var log bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	defer slog.SetDefault(prev)
	sink := make(chan dstatus.Event, 1)
	b := newBinder(session.Session{ID: "s1", Dir: "/w"}, scanned(func(string, string, time.Time) []string { return []string{"a-conv", "b-conv"} }),
		"/h", sink, time.Now, func(string) bool { return false })

	if b.Try() {
		t.Error("Try() = true with two candidates, want it left unbound")
	}
	if len(sink) != 0 {
		t.Errorf("an event was sent: %+v", <-sink)
	}
	if !strings.Contains(log.String(), "a-conv") || !strings.Contains(log.String(), "b-conv") {
		t.Errorf("log %q does not name both candidates", log.String())
	}
}

// A conversation another row already holds is not a candidate: the earlier
// session in the same directory bound it first (#523).
func TestBind_AHeldConversationIsNotACandidate_issue523(t *testing.T) {
	sink := make(chan dstatus.Event, 1)
	b := newBinder(session.Session{ID: "s2", Dir: "/w"}, scanned(func(string, string, time.Time) []string { return []string{"taken", "mine"} }),
		"/h", sink, time.Now, func(c string) bool { return c == "taken" })

	if !b.Try() || (<-sink).SessionID != "mine" {
		t.Error("did not bind the one candidate no row holds")
	}
}

// Nothing in the store yet - the agent has not written - is a retry on the
// next tick, and Add returns at once rather than wait for it (#523).
func TestWatch_AScannedSessionBindsOnALaterTick_issue523(t *testing.T) {
	calls := make(chan struct{}, 8)
	locate := func(string, string, time.Time) []string {
		calls <- struct{}{}
		if len(calls) < 2 {
			return nil
		}
		return []string{"late"}
	}
	agents, err := agent.NewCatalog(scanned(locate))
	if err != nil {
		t.Fatal(err)
	}
	w := &Watch{deps: WatchDeps{Agents: agents, Clock: time.Now}, events: make(chan dstatus.Event, 4),
		tailers: map[string]*Tailer{}, adapters: map[string]dstatus.Adapter{}, binders: map[string]*binder{}, sessions: map[string]session.Session{}, stopPump: func() {}}
	w.Add(session.Session{ID: "s1", Dir: "/w"})
	defer w.Close()

	select {
	case ev := <-w.events:
		if ev.SessionID != "late" || ev.Owner != "s1" {
			t.Errorf("event %+v, want s1 bound to late", ev)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the scan never retried")
	}
	if w.tailers["s1"] != nil {
		t.Error("an unbound session was tailed before it knew its transcript")
	}
}

// A Scanned session already bound - Conversation persisted, say after a
// crash - is tailed at once, with no scan (invariant 9, #523).
func TestWatch_ABoundScannedSessionIsTailed_issue523(t *testing.T) {
	agents, err := agent.NewCatalog(scanned(func(string, string, time.Time) []string {
		t.Error("a bound session was scanned")
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	w := &Watch{deps: WatchDeps{Agents: agents, Clock: time.Now, OpenTranscript: func(string) Transcript { return nopTranscript{} }},
		events: make(chan dstatus.Event, 4), tailers: map[string]*Tailer{}, adapters: map[string]dstatus.Adapter{},
		binders: map[string]*binder{}, sessions: map[string]session.Session{}, stopPump: func() {}}
	w.Add(session.Session{ID: "s1", Dir: "/w", Conversation: "known"})
	defer w.Close()
	if w.tailers["s1"] == nil || w.binders["s1"] != nil {
		t.Error("a bound Scanned session was not tailed, or was scanned")
	}
}

// A Reported agent's startup hook names its own id; its adapter maps that to
// SessionRebound, and the event names the pane from OMATTY_SESSION - the
// #316 path with a second caller (#523).
func TestWatch_AReportedIDRebindsThePane_issue523(t *testing.T) {
	w := &Watch{deps: WatchDeps{Clock: time.Now}, events: make(chan dstatus.Event, 1),
		adapters: map[string]dstatus.Adapter{"t1": reportingAdapter{&recordingAdapter{}}}}
	w.offerHook(dstatus.HookPayload{SessionID: "toy-own-id", HookEventName: "start", OmattySession: "t1"})
	ev := <-w.events
	if ev.Kind != dstatus.SessionRebound || ev.Owner != "t1" || ev.SessionID != "toy-own-id" {
		t.Errorf("event %+v, want t1 rebound onto toy-own-id", ev)
	}
}

// reportingAdapter is a Reported agent: its startup hook reports its id.
type reportingAdapter struct{ *recordingAdapter }

func (reportingAdapter) KindOf(p dstatus.HookPayload) (dstatus.Kind, bool) {
	return dstatus.SessionRebound, p.HookEventName == "start"
}

// nopTranscript has never been written.
type nopTranscript struct{}

func (nopTranscript) Poll() ([][]byte, bool, bool) { return nil, false, false }

// A Process-tier agent keeps no transcript omatty can read: it is not
// tailed, and adding it must not reach for a transcript path it lacks (#525).
func TestWatch_AProcessAgentIsNotTailed_issue525(t *testing.T) {
	agents, err := agent.NewCatalog(agent.Generic("aider", []string{"aider"}))
	if err != nil {
		t.Fatal(err)
	}
	w := &Watch{deps: WatchDeps{Agents: agents, Clock: time.Now}, events: make(chan dstatus.Event, 1),
		tailers: map[string]*Tailer{}, adapters: map[string]dstatus.Adapter{}, binders: map[string]*binder{},
		sessions: map[string]session.Session{}, stopPump: func() {}}
	w.Add(session.Session{ID: "a1", Dir: "/w"})
	defer w.Close()
	if w.tailers["a1"] != nil || w.binders["a1"] != nil {
		t.Error("a Process-tier session was tailed or scanned")
	}
}
