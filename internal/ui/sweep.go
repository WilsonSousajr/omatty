// The idle sweep (#319): with [sessions] idle_stop set, a session quiet for
// that long is stopped exactly as ctrl+o s stops one - process ended, row,
// transcript and comments kept, enter resumes it. Lazy start stops omatty
// paying for sessions nobody opens (#317); this reclaims the ones opened days
// ago and forgotten, which on the machine this was measured on were ten of
// eleven, and 2.9 GB.

package ui

import (
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// SweepTickMsg is the idle sweep's heartbeat. Exported so tests can send one.
type SweepTickMsg time.Time

// sweepCap is the longest the sweep waits between looks, so a long threshold
// is still honoured to within a minute.
const sweepCap = time.Minute

// sweepEvery is the tick for a threshold: the threshold itself, capped.
func sweepEvery(threshold time.Duration) time.Duration { return min(threshold, sweepCap) }

// withSweep attaches the threshold and seeds activeAt with the boot's own
// starts: a session omatty has just started is not idle, whatever its
// transcript last recorded.
func (m *Model) withSweep(d Deps) *Model {
	m.idleStop = d.IdleStop
	m.activeAt = make(map[string]time.Time, len(m.terms))
	for id := range m.terms {
		m.markActive(id)
	}
	return m
}

// markActive records that omatty started id's process or the operator typed
// into its pane, now. Written in exactly those two places.
func (m *Model) markActive(id string) { m.activeAt[id] = m.clock() }

// scheduleSweep arms the next sweep, or nothing at all when the sweep is off,
// so a machine that never asked for it pays not even for a timer.
//
// Deliberately NOT gated on hasFocus, unlike pollAll. That gate exists
// because polling git for cards nobody can see is waste; the sweep matters
// most in exactly the state pollAll skips - omatty left in a background window
// for days, which is the measurement #319 opened with.
func (m *Model) scheduleSweep() tea.Cmd {
	if m.idleStop <= 0 {
		return nil
	}
	return tea.Tick(sweepEvery(m.idleStop), func(t time.Time) tea.Msg { return SweepTickMsg(t) })
}

// onSweepTick stops every sweepable session and arms the next tick. Each stop
// is stopSession's own command, so N stops are N goroutines and none of them
// holds the frame.
func (m *Model) onSweepTick() tea.Cmd {
	return tea.Batch(append(m.sweep(), m.scheduleSweep())...)
}

// sweep stops each session sweepable says may go, and logs why.
func (m *Model) sweep() []tea.Cmd {
	var cmds []tea.Cmd
	for _, sess := range m.state.Sessions {
		if !m.sweepable(sess.ID) {
			continue
		}
		// Strings, not Durations: the JSON handler writes a Duration as
		// nanoseconds, and "idle":259200000000000 is not "3 days".
		idle := m.clock().Sub(m.lastActive(sess.ID)).Round(time.Second)
		slog.Info("stopping an idle session", "session", sess.ID, "title", sess.Title,
			"idle", idle.String(), "threshold", m.idleStop.String())
		cmds = append(cmds, m.stopSession(sess))
	}
	return cmds
}

// sweepable refuses, in order: a session with no process, since there is
// nothing to stop; the selected one, whose pane must not go blank under the
// operator's hands; one whose status is not settled, so neither a turn in
// flight nor a question waiting on the operator is ever cut off; and one
// active inside the threshold.
func (m *Model) sweepable(id string) bool {
	if m.terms[id] == nil || m.isSelected(id) || !settled(m.status[id].Status) {
		return false
	}
	return m.clock().Sub(m.lastActive(id)) >= m.idleStop
}

// settled reports whether a status is one a session may be stopped in.
func settled(s watcher.Status) bool {
	switch s {
	case watcher.StatusThinking, watcher.StatusTool, watcher.StatusWaiting:
		return false
	}
	return true
}

// lastActive is the newer of what the transcript last recorded - the entry's
// own timestamp, which the tailer's first read of the whole file restores on
// every boot, so nothing is persisted - and activeAt: when omatty started the
// process or the operator last typed into it. The transcript alone is not
// enough. A session started two seconds ago has none, and one being typed
// into is in use whatever its transcript says.
func (m *Model) lastActive(id string) time.Time {
	at := m.status[id].At
	if active := m.activeAt[id]; active.After(at) {
		return active
	}
	return at
}
