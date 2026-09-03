package ui

import (
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/notify"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// StatusMsg carries one watcher event into the model's Update loop.
type StatusMsg watcher.Event

// TickMsg is the once-a-second heartbeat that re-renders the frame, so a
// quiet session's age keeps counting (issue #71). Exported so tests can send
// one.
type TickMsg time.Time

// tickEvery is the age column's resolution; finer buys nothing.
const tickEvery = time.Second

func scheduleTick() tea.Cmd {
	return tea.Tick(tickEvery, func(t time.Time) tea.Msg { return TickMsg(t) })
}

// waitForEvent blocks on the next status event and delivers it as a StatusMsg.
// nil events (a model built without a watcher) simply never fires.
func (m *Model) waitForEvent() tea.Cmd {
	if m.events == nil {
		return nil
	}
	return func() tea.Msg { return StatusMsg(<-m.events) }
}

// onStatus folds a watcher event into the session's state and re-arms the
// wait. Newer-wins lives in watcher.Apply; the model just stores the result.
// A hook can name any session id; only registered ones may grow the status
// map or reach the operator's notifications (issue #69).
func (m *Model) onStatus(ev StatusMsg) tea.Cmd {
	e := watcher.Event(ev)
	if !m.knownSession(e.SessionID) {
		return m.waitForEvent()
	}
	before := m.status[e.SessionID].Status
	after := watcher.Apply(m.status[e.SessionID], e)
	m.status[e.SessionID] = after
	m.sidebar.SetRows(SidebarRows(m.state, m.statusMap()))
	return tea.Batch(m.waitForEvent(), m.maybeNotify(e, before, after.Status),
		m.refreshReview(e.SessionID, before, after.Status))
}

func (m *Model) knownSession(id string) bool {
	for i := range m.state.Sessions {
		if m.state.Sessions[i].ID == id {
			return true
		}
	}
	return false
}

// notifyCooldown is the least time between two notifications for one
// session, so a permission loop cannot storm the desktop (issue #69).
const notifyCooldown = 5 * time.Second

// maybeNotify returns a command that posts a desktop notification when a
// session enters a state that needs the operator while omatty is
// backgrounded. It is a command, off the Update goroutine, because osascript
// takes tens of milliseconds (issue #69). Suppressed: a repeated state, a
// transition older than this run (issue #70), and a second notification for
// the same session within notifyCooldown.
func (m *Model) maybeNotify(e watcher.Event, before, after watcher.Status) tea.Cmd {
	if m.hasFocus || before == after || e.At.Before(m.startedAt) {
		return nil
	}
	body, ok := needsYou(m.sessionTitle(e.SessionID), after)
	if !ok || !m.cooldownElapsed(e.SessionID) {
		return nil
	}
	return notifyCmd(m.notifier, body)
}

func (m *Model) cooldownElapsed(id string) bool {
	now := m.clock()
	if last, ok := m.notified[id]; ok && now.Sub(last) < notifyCooldown {
		return false
	}
	m.notified[id] = now
	return true
}

func notifyCmd(n notify.Notifier, body string) tea.Cmd {
	return func() tea.Msg {
		if err := n.Notify("omatty", body); err != nil {
			slog.Warn("desktop notification failed", "body", body, "err", err)
		}
		return nil
	}
}

func (m *Model) sessionTitle(id string) string {
	for i := range m.state.Sessions {
		if m.state.Sessions[i].ID == id {
			return m.state.Sessions[i].Title
		}
	}
	return id
}

// needsYou returns the notification body for a status that wants attention.
func needsYou(title string, status watcher.Status) (string, bool) {
	switch status {
	case watcher.StatusWaiting:
		return title + " needs you", true
	case watcher.StatusDone:
		return title + " finished", true
	default:
		return "", false
	}
}

// statusMap projects the per-session state down to the status the sidebar
// needs.
func (m *Model) statusMap() map[string]watcher.Status {
	out := make(map[string]watcher.Status, len(m.status))
	for id, st := range m.status {
		out[id] = st.Status
	}
	return out
}
