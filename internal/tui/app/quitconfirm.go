// Quitting with something to lose asks once (#749). ctrl+o q used to quit on
// the first press, so one mistyped key after the leader - q sits beside w -
// closed the window with sessions mid-turn and review notes unsent. dtach keeps
// the conversations since M6, but the notes live only in memory.
//
// The confirmation is Claude Code's own convention, which omatty's operators
// meet every day: a second press, a hint in the footer, no modal. Quitting with
// nothing to lose stays a single press, because asking then would be noise.

package app

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
)

// requestQuit is ctrl+o q: it quits at once when nothing would be lost, and
// otherwise arms the confirmation and says what is at stake.
//
//	case "q": return m.requestQuit()
func (m *Model) requestQuit() tea.Cmd {
	if m.quitPrimed {
		return tea.Quit
	}
	stake := m.quitStake()
	if stake == "" {
		return tea.Quit
	}
	m.quitArmed = true
	m.notice = stake + " - " + m.leader + " q again to quit"
	return nil
}

// disarmQuit runs on every key. Only the key straight after the first q may
// confirm it, so the armed state moves into quitPrimed for that one key and is
// cleared. The leader itself is skipped: it only begins the second ctrl+o q.
// Any other key means the operator moved on, and a q pressed minutes later
// asks again.
//
//	m.disarmQuit(route == keys.Swallow)
func (m *Model) disarmQuit(swallowed bool) {
	if !swallowed {
		m.quitPrimed, m.quitArmed = m.quitArmed, false
	}
}

// quitStake names what quitting would cost, or "" when it costs nothing:
// sessions mid-turn or at a permission prompt, and unsent review notes.
func (m *Model) quitStake() string {
	working, waiting := m.busySessions()
	var parts []string
	parts = appendCount(parts, working, "working", "working")
	parts = appendCount(parts, waiting, "waiting", "waiting")
	parts = appendCount(parts, m.unsentNotes(), "unsent note", "unsent notes")
	return strings.Join(parts, " · ")
}

// busySessions counts the sessions in a turn and those waiting on a
// permission, from the same statuses the sidebar draws.
func (m *Model) busySessions() (working, waiting int) {
	for _, st := range m.status {
		switch st.Status {
		case dstatus.StatusThinking, dstatus.StatusTool:
			working++
		case dstatus.StatusWaiting:
			waiting++
		}
	}
	return working, waiting
}

// unsentNotes counts the review comments queued across every session.
func (m *Model) unsentNotes() int {
	n := 0
	for _, c := range m.comments {
		n += c.PendingLen()
	}
	return n
}

// appendCount adds "N noun" to parts when n is non-zero.
//
//	appendCount(nil, 2, "unsent note", "unsent notes") // ["2 unsent notes"]
func appendCount(parts []string, n int, one, many string) []string {
	switch {
	case n == 1:
		return append(parts, "1 "+one)
	case n > 1:
		return append(parts, strconv.Itoa(n)+" "+many)
	}
	return parts
}
