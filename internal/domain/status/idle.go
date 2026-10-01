package status

import "time"

// The idle sweep's policy (#319): which session may be stopped for being
// quiet. Pure since migration step 5.7 (Amendment 8, #653); the timer that
// asks, and the facts only the TUI holds - which pane is selected, which
// session has a live terminal - stay with the TUI.

// Settled reports whether a status is one a session may be stopped in. A turn
// in flight or a question waiting on the operator is never cut off.
//
//	if status.Settled(st.Status) { ... }
func Settled(s Status) bool {
	switch s {
	case StatusThinking, StatusTool, StatusWaiting:
		return false
	}
	return true
}

// LastActive is the newer of what the transcript last recorded - the entry's
// own timestamp, which the tailer's first read of the whole file restores on
// every boot, so nothing is persisted - and active: when omatty started the
// process or the operator last typed into it. The transcript alone is not
// enough. A session started two seconds ago has none, and one being typed
// into is in use whatever its transcript says.
//
//	last := status.LastActive(st.At, activeAt[id])
func LastActive(transcript, active time.Time) time.Time {
	if active.After(transcript) {
		return active
	}
	return transcript
}

// Sweepable reports whether a session in status s, last active at last, may be
// stopped at now under threshold: settled, and quiet for at least threshold.
//
//	if status.Sweepable(st.Status, last, clock(), idleStop) { ... }
func Sweepable(s Status, last, now time.Time, threshold time.Duration) bool {
	return Settled(s) && now.Sub(last) >= threshold
}
