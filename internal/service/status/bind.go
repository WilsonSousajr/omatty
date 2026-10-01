// Learning a session's identity by scanning the agent's store (#523). An
// agent that takes no id from omatty and reports none at start still writes
// a transcript somewhere under its store; the binder finds the one this
// session began and binds the pane to it, exactly as a /clear rebinds it
// (#316): a SessionRebound naming the pane, which the TUI persists as the
// row's Conversation, so the binding survives a crash (invariant 9).

package status

import (
	"log/slog"
	"sync"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
)

// binder looks for one unbound session's conversation on every tick until it
// finds exactly one, then sends the binding and stops.
type binder struct {
	sess  session.Session
	agent agent.Profile
	home  string
	sink  chan<- dstatus.Event
	clock func() time.Time
	// held reports a conversation another row already answers to, which is
	// never this session's: the earlier session in the directory bound it.
	held func(conversation string) bool
	stop chan struct{}
	once sync.Once
}

// newBinder returns a binder for sess, not yet running; Watch.Add starts it.
func newBinder(sess session.Session, profile agent.Profile, home string, sink chan<- dstatus.Event,
	clock func() time.Time, held func(string) bool) *binder {
	return &binder{sess: sess, agent: profile, home: home, sink: sink, clock: clock, held: held,
		stop: make(chan struct{})}
}

// Try binds the session if the agent's store holds exactly one conversation
// for its directory, begun since it was registered, that no other row holds.
// With none it waits for the next tick. With two or more it refuses to
// guess: a wrong binding would show one session's status on another's card,
// the bug class invariant 2 exists to prevent. Capitalised like Tailer.Poll:
// tests drive it without a ticker.
func (b *binder) Try() bool {
	candidates := b.unheld(b.agent.Locate(b.home, b.sess.Dir, b.sess.Started))
	if len(candidates) > 1 {
		slog.Warn("session not bound: more than one conversation could be its own",
			"session", b.sess.ID, "agent", b.agent.Name, "candidates", candidates)
	}
	if len(candidates) != 1 {
		return false
	}
	b.sink <- dstatus.Event{SessionID: candidates[0], Kind: dstatus.SessionRebound, At: b.clock(), Owner: b.sess.ID}
	return true
}

func (b *binder) unheld(conversations []string) []string {
	var out []string
	for _, c := range conversations {
		if !b.held(c) {
			out = append(out, c)
		}
	}
	return out
}

// run tries now and then on every tick, until bound or closed. The first try
// is immediate so a store already written binds without a second's wait.
func (b *binder) run(every time.Duration) {
	defer recoverLoop("binder", b.sess.ID)
	t := time.NewTicker(every)
	defer t.Stop()
	for !b.Try() {
		select {
		case <-b.stop:
			return
		case <-t.C:
		}
	}
}

// Close stops the binder. It is idempotent.
func (b *binder) Close() { b.once.Do(func() { close(b.stop) }) }
