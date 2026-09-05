package watcher

import (
	"log/slog"
	"sync"
	"time"

	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
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
//	w := watcher.Start(home, st.Sessions, time.Now)
//	defer w.Close()
//	model := ui.NewModel(ui.Deps{Events: w.Events(), TailStart: w.Add, /* ... */})
type Watch struct {
	home     string
	clock    func() time.Time
	events   chan Event
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
func Start(home string, sessions []registry.Session, clock func() time.Time) *Watch {
	w := &Watch{
		home: home, clock: clock,
		events:  make(chan Event, eventBuffer),
		tailers: map[string]*Tailer{},
	}
	l, err := Listen(paths.HookSocket(home), w.events, clock)
	if err != nil {
		slog.Warn("hook socket unavailable; status comes from the transcript only", "err", err)
	}
	w.listener = l
	for _, sess := range sessions {
		w.Add(sess)
	}
	return w
}

// Events is the stream the model reads status from.
func (w *Watch) Events() <-chan Event { return w.events }

// Add starts tailing a session's transcript, for a session created at
// runtime as well as the initial ones. Adding an id that is already tailed
// stops the tailer it displaces, which nothing else holds a reference to (#40).
func (w *Watch) Add(sess registry.Session) {
	tl := Tail(sess.ID, paths.Transcript(w.home, sess.Dir, sess.ID), w.events, w.clock, pollEvery)
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
	if w.listener != nil {
		_ = w.listener.Close()
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, tl := range w.tailers {
		tl.Close()
	}
}
