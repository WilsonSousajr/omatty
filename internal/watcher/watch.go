package watcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/pubsub"
)

// eventBuffer sizes the channel between the watcher and the UI. A short burst
// (a session finishing several tools) must not block a hook.
const eventBuffer = 64

// pollEvery is how often each tailer re-reads its transcript. One second is
// the sidebar's own resolution; hooks cover the sub-second cases.
const pollEvery = time.Second

// Watch owns the status subsystem's goroutines: one hook listener and one
// tailer per session, feeding one channel. ui.Run holds a Watch; it no longer
// knows the socket path, the transcript path, the poll interval, or the
// buffer size (issue #77).
//
//	w := watcher.Start(watcher.WatchDeps{Home: home, Clock: time.Now,
//	        Adapter: profile.Status, TranscriptPath: profile.TranscriptPath}, st.Sessions)
//	defer w.Close()
//	model := ui.NewModel(ui.Deps{Events: w.Subscribe(ctx), TailStart: w.Add, /* ... */})
type Watch struct {
	deps   WatchDeps
	events chan Event
	// broker fans events out to every subscriber (ADR 0001, step 5.2a, #653).
	// The pump that feeds it starts on the first Subscribe: tailers report a
	// session's current status the moment Start adds them, and published to
	// nobody those would be lost. Until someone subscribes they wait in events,
	// as they waited for the TUI to start reading before.
	broker   *pubsub.Broker[Event]
	pumpOnce sync.Once
	stopPump context.CancelFunc
	pumpCtx  context.Context
	listener *Listener // nil when the socket could not bind (issue #49)
	mu       sync.Mutex
	// tailers is keyed by session id so archiving one session can stop its
	// tailer and only its tailer (#40). As a slice there was no way back from
	// an id to a goroutine, so a removed session polled a path that no longer
	// existed once a second until omatty quit.
	tailers map[string]*Tailer
}

// Start opens the hook socket and a tailer per session. A socket that cannot
// bind degrades to tailer-only with a logged warning (issue #49): the
// listener is the low-latency source, the tailer is the source of truth, so
// a lost socket costs only the instant hook-driven "waiting" glyph.
//
// The adapter and the transcript path come from the agent's profile, so
// this package never names claude's own layout (#46).
func Start(d WatchDeps, sessions []session.Session) *Watch {
	pumpCtx, stopPump := context.WithCancel(context.Background())
	w := &Watch{
		deps:     d,
		events:   make(chan Event, eventBuffer),
		broker:   pubsub.NewBroker[Event](eventBuffer),
		pumpCtx:  pumpCtx,
		stopPump: stopPump,
		tailers:  map[string]*Tailer{},
	}
	l, err := Listen(paths.HookSocket(d.Home), w.events, d.Clock, d.Adapter)
	if err != nil {
		slog.Warn("hook socket unavailable; status comes from the transcript only", "err", err)
	}
	w.listener = l
	for _, sess := range sessions {
		w.Add(sess)
	}
	return w
}

// HooksLive reports whether the hook socket bound. Without it every event
// comes from the tailer, and nothing that needs a hook - a turn baseline
// (#311) - will ever happen (#49).
func (w *Watch) HooksLive() bool { return w.listener != nil }

// Subscribe returns every status event from now on, until ctx ends. The first
// call starts the pump, so nothing reported before anyone listened is lost.
//
//	events := w.Subscribe(ctx)
func (w *Watch) Subscribe(ctx context.Context) <-chan pubsub.Event[Event] {
	ch := w.broker.Subscribe(ctx)
	w.pumpOnce.Do(func() { go w.pump() })
	return ch
}

// pump republishes what the tailers and the listener report. Publish waits
// for room rather than drop: a lost status is a wrong card (invariant 2). The
// listener's drop-not-wait is untouched, because it offers into events.
func (w *Watch) pump() {
	for {
		select {
		case ev := <-w.events:
			_ = w.broker.Publish(w.pumpCtx, pubsub.Event[Event]{Kind: pubsub.Updated, Payload: ev})
		case <-w.pumpCtx.Done():
			return
		}
	}
}

// Add starts tailing a session's transcript, for a session created at
// runtime as well as the initial ones. Adding an id that is already tailed
// stops the tailer it displaces, which nothing else holds a reference to (#40).
//
// The transcript is the conversation's, and so are the ids its events carry:
// after /clear that is no longer sess.ID, and re-adding the rebound session
// is how the tailer follows it (#316). The map stays keyed by sess.ID.
func (w *Watch) Add(sess session.Session) {
	conv := sess.ConversationID()
	tl := Tail(conv, w.deps.OpenTranscript(w.deps.TranscriptPath(w.deps.Home, sess.Dir, conv)), w.events, w.deps.Clock, pollEvery, w.deps.Adapter)
	w.mu.Lock()
	defer w.mu.Unlock()
	if old := w.tailers[sess.ID]; old != nil {
		old.Close()
	}
	w.tailers[sess.ID] = tl
}

// Remove stops one session's tailer, for a session archived at runtime (#40).
// An id that is not tailed is a no-op: the model calls this for every archive,
// including one whose tailer never started.
//
//	w.Remove(sess.ID)
func (w *Watch) Remove(sessionID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	tl := w.tailers[sessionID]
	if tl == nil {
		return
	}
	tl.Close()
	delete(w.tailers, sessionID)
}

// Close stops the listener and every tailer. Idempotent per tailer; safe to
// call once the program has exited.
func (w *Watch) Close() {
	defer w.stopPump()
	if w.listener != nil {
		_ = w.listener.Close()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, tl := range w.tailers {
		tl.Close()
	}
}
