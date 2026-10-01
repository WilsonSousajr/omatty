package status

import "time"

// Read is a session's state as one read of its whole transcript implies:
// the tailer's first poll, run once, with no timer and no hook socket (ADR
// 0001's transcript Read; migration step 7.1, #653). It is what a one-shot
// command - `omatty status --json` - shows, and it is the same derivation
// the live card makes, because it is the same code.
//
//	st := status.Read(sess.ConversationID(), transcript.NewReader(path), profile.Status, time.Now)
func Read(sessionID string, src Transcript, adapter Adapter, clock func() time.Time) SessionState {
	// One poll sends at most a status event and a usage event.
	sink := make(chan Event, 2)
	tl := &Tailer{sessionID: sessionID, src: src, sink: sink, clock: clock, adapter: adapter,
		stop: make(chan struct{}), done: make(chan struct{})}
	tl.Poll()
	close(sink)
	var st SessionState
	for ev := range sink {
		st = Apply(st, ev)
	}
	return st
}
