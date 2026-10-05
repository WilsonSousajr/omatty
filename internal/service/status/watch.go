package status

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/pubsub"
)

// eventBuffer sizes the channel between the watcher and the UI. A short burst
// (a session finishing several tools) must not block a hook.
const eventBuffer = 64

// pollEvery is how often each tailer re-reads its transcript. One second is
// the sidebar's own resolution; hooks cover the sub-second cases.
const pollEvery = time.Second

// Watch owns the status subsystem's goroutines: one hook listener and one
// tailer per session, feeding one channel. cmd/omatty holds a Watch; it no longer
// knows the socket path, the transcript path, the poll interval, or the
// buffer size (issue #77).
//
//	w := status.Start(status.WatchDeps{Home: home, Clock: time.Now, Agents: agents}, st.Sessions)
//	defer w.Close()
//	model := ui.NewModel(ui.Deps{Events: w.Subscribe(ctx), TailStart: w.Add, /* ... */})
type Watch struct {
	deps   WatchDeps
	events chan dstatus.Event
	// broker fans events out to every subscriber (ADR 0001, step 5.2a, #653).
	// The pump that feeds it starts on the first Subscribe: tailers report a
	// session's current status the moment Start adds them, and published to
	// nobody those would be lost. Until someone subscribes they wait in events,
	// as they waited for the TUI to start reading before.
	broker   *pubsub.Broker[dstatus.Event]
	pumpOnce sync.Once
	stopPump context.CancelFunc
	pumpCtx  context.Context
	listener io.Closer // the hook server; nil when the socket could not bind (issue #49)
	mu       sync.Mutex
	// tailers is keyed by session id so archiving one session can stop its
	// tailer and only its tailer (#40). As a slice there was no way back from
	// an id to a goroutine, so a removed session polled a path that no longer
	// existed once a second until omatty quit.
	tailers map[string]*Tailer
	// adapters is each tailed session's parser, so a hook payload is read by
	// the agent of the session it names (#521). Guarded by mu.
	adapters map[string]dstatus.Adapter
	// binders scan for the conversation of each Scanned session not yet
	// bound, and sessions is every session added, whose conversations a
	// binder must not claim (#523). Both guarded by mu.
	binders  map[string]*binder
	sessions map[string]session.Session
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
		events:   make(chan dstatus.Event, eventBuffer),
		broker:   pubsub.NewBroker[dstatus.Event](eventBuffer),
		pumpCtx:  pumpCtx,
		stopPump: stopPump,
		tailers:  map[string]*Tailer{},
		adapters: map[string]dstatus.Adapter{},
		binders:  map[string]*binder{},
		sessions: map[string]session.Session{},
	}
	w.serveHooks()
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
func (w *Watch) Subscribe(ctx context.Context) <-chan pubsub.Event[dstatus.Event] {
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
			_ = w.broker.Publish(w.pumpCtx, pubsub.Event[dstatus.Event]{Kind: pubsub.Updated, Payload: ev})
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
	profile, err := w.deps.Agents.Lookup(sess.Agent)
	if err != nil {
		slog.Warn("session not tailed: its agent is unknown", "session", sess.ID, "err", err)
		w.Remove(sess.ID)
		return
	}
	w.Remove(sess.ID)
	if !profile.KeepsTranscript() {
		// Process tier: running or exited is all there is to know, and the
		// pane's process says that without a tailer (#525).
		return
	}
	if profile.Caps.Identity == agent.Scanned && sess.Conversation == "" {
		w.startBinder(sess, profile)
		return
	}
	w.tail(sess, profile)
}

// tail follows a session whose transcript is known.
func (w *Watch) tail(sess session.Session, profile agent.Profile) {
	conv := sess.ConversationID()
	tl := Tail(conv, w.deps.OpenTranscript(profile.TranscriptPath(w.deps.Home, sess.Dir, conv)), w.events, w.deps.Clock, pollEvery, profile.Status)
	w.mu.Lock()
	defer w.mu.Unlock()
	w.tailers[sess.ID] = tl
	w.adapters[sess.ID] = profile.Status
	w.sessions[sess.ID] = sess
}

// startBinder scans for an unbound Scanned session's conversation. The
// session is not tailed meanwhile: its transcript is not known yet. Once
// bound, the TUI re-adds it with its Conversation and it is tailed (#523).
func (w *Watch) startBinder(sess session.Session, profile agent.Profile) {
	b := newBinder(sess, profile, w.deps.Home, w.events, w.deps.Clock, func(c string) bool { return w.heldBy(sess.ID, c) })
	w.mu.Lock()
	w.binders[sess.ID] = b
	w.sessions[sess.ID] = sess
	w.mu.Unlock()
	go b.run(pollEvery)
}

// heldBy reports whether a session other than id answers to conversation.
func (w *Watch) heldBy(id, conversation string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for other, sess := range w.sessions {
		if other != id && sess.ConversationID() == conversation {
			return true
		}
	}
	return false
}

// Remove stops one session's tailer, for a session archived at runtime (#40).
// An id that is not tailed is a no-op: the model calls this for every archive,
// including one whose tailer never started.
//
//	w.Remove(sess.ID)
func (w *Watch) Remove(sessionID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if tl := w.tailers[sessionID]; tl != nil {
		tl.Close()
	}
	if b := w.binders[sessionID]; b != nil {
		b.Close()
	}
	delete(w.tailers, sessionID)
	delete(w.adapters, sessionID)
	delete(w.binders, sessionID)
	delete(w.sessions, sessionID)
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
	for _, b := range w.binders {
		b.Close()
	}
}

// serveHooks opens the hook socket through the injected server and follows
// what it hands over. A socket that cannot bind leaves listener nil: status
// then comes from the transcript alone (issue #49).
func (w *Watch) serveHooks() {
	payloads := make(chan dstatus.HookPayload, eventBuffer)
	l, err := w.deps.ListenHooks(w.deps.HookSocket, payloads)
	if err != nil {
		slog.Warn("hook socket unavailable; status comes from the transcript only", "err", err)
		return
	}
	w.listener = l
	go w.followHooks(payloads)
}

// followHooks turns what the hook server hands over into events: the agent's
// adapter says which payloads are tracked and what they mean (#46), and each
// is stamped now so it compares like-for-like with the tailer's. It offers,
// never waits: the server already dropped rather than block a hook, and a
// full channel here means the same thing - the tailer restores the truth
// within a second (invariant 11, step 5.2d, #653).
func (w *Watch) followHooks(payloads <-chan dstatus.HookPayload) {
	for {
		select {
		case p := <-payloads:
			w.offerHook(p)
		case <-w.pumpCtx.Done():
			return
		}
	}
}

func (w *Watch) offerHook(p dstatus.HookPayload) {
	kind, ok := w.hookAdapter(p.OmattySession).KindOf(p)
	if !ok {
		return
	}
	ev := dstatus.Event{SessionID: p.SessionID, Kind: kind, At: w.deps.Clock(), Owner: p.OmattySession, Hook: true}
	select {
	case w.events <- ev:
	default:
		slog.Debug("hook event dropped, events full", "session", ev.SessionID)
	}
}

// hookAdapter is the parser for a hook payload: the agent of the session the
// payload's OMATTY_SESSION names. A payload naming no tailed session - a
// claude started before #316 set the variable - is read as the default
// agent's, which is what every hook was before M17 (#521).
func (w *Watch) hookAdapter(owner string) dstatus.Adapter {
	w.mu.Lock()
	a := w.adapters[owner]
	w.mu.Unlock()
	if a != nil {
		return a
	}
	profile, _ := w.deps.Agents.Lookup("")
	return profile.Status
}
