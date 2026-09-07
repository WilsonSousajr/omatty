package ui

import (
	"strings"

	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// The activity lane: a short trace of what each session has been doing,
// drawn as block cells beside its name (#128). Display-only and deliberately
// not persisted: state.json must suffice to relaunch every session
// (invariant 9), and a lane is no part of that - it is derived from the
// watcher's events, which are derived from the transcript, so a restart
// rebuilds whatever the tailer's first poll reports and nothing that mattered
// is lost.

// laneCells is how many statuses a lane shows. Eight, and the title column
// pays for it: SidebarWidth 28, two border columns, marker and glyph four,
// eight cells and a separator leave a session thirteen columns of name. The
// dial to turn if thirteen reads too short on screen, with SidebarWidth as
// the other.
const laneCells = 8

// activityLane is one session's ring of recent statuses, oldest first.
type activityLane struct {
	seen   [laneCells]watcher.Status
	filled int // a new session draws a short lane, not eight cells of nothing
}

// push records one status, dropping the oldest.
func (l activityLane) push(s watcher.Status) activityLane {
	copy(l.seen[:], l.seen[1:])
	l.seen[laneCells-1] = s
	l.filled = min(l.filled+1, laneCells)
	return l
}

// laneBlock maps a status to its cell height: how much was happening. A
// height, not only a colour, so the lane still reads in a monochrome
// terminal. Waiting is the full cell because it is the one urgent thing on
// this screen. Like the rounded border, these are East Asian Ambiguous:
// RUNEWIDTH_EASTASIAN=1 doubles the border, the glyphs and the lane together
// or none of them.
var laneBlock = map[watcher.Status]string{
	watcher.StatusExited: " ", watcher.StatusIdle: "▁", watcher.StatusDone: "▂",
	watcher.StatusError: "▃", watcher.StatusThinking: "▄", watcher.StatusTool: "▆",
	watcher.StatusWaiting: "█",
}

// renderLane draws a session's trace, each cell coloured by the status it
// records through glyphStyle - so "waiting" is lit in the waiting colour and
// the lane answers "which of these needs me" at a glance.
func (m *Model) renderLane(id string) string {
	l := m.lane[id]
	var b strings.Builder
	b.WriteString(strings.Repeat(" ", laneCells-l.filled))
	for _, s := range l.seen[laneCells-l.filled:] {
		b.WriteString(glyphStyle(s).Render(laneBlock[s]))
	}
	return b.String()
}

// trace records one event in the session's lane. A usage update draws
// nothing: it carries tokens and never moves the status, so a cell for one
// would show activity that did not happen.
func (m *Model) trace(id string, before, after watcher.SessionState) {
	if after.At.Equal(before.At) && after.Status == before.Status {
		return
	}
	m.lane[id] = m.lane[id].push(after.Status)
}
