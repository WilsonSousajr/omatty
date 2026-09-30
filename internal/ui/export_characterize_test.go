package ui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

// Fingerprint is the part of the model a keypress or a message can change,
// as one line, so the characterization tables (#620) can say what every key
// and message does without knowing how. The fields are the ones ADR 0001
// dissolves into screens; a move that loses one changes a row.
func (m *Model) Fingerprint() string {
	target, ok := m.focus()
	return fmt.Sprintf("focus=%d/%t armed=%t modal=%q view=%d sel=%s open=%t focused=%t zoom=%t "+
		"diff=%d files=%d gate=%d tracker=%d filter=%q note=%t",
		target, ok, m.router.Pending(), m.modalLabel(), m.review.View, m.Selected(), m.review.Open,
		m.review.Focused, m.review.Zoomed, m.review.DiffList.Cursor, m.review.Files.Cursor,
		m.review.GateCursor, m.TrackerCursor(), m.activeFilter().Query, m.review.Note.Active)
}

func (m *Model) modalLabel() string {
	if !m.modalOpen() {
		return ""
	}
	return modalName(m.modal)
}

// UnexportedRouted is one value of each message type Update routes that has
// no exported constructor, keyed by the type's name as msgroute.go spells it,
// so the message table can cover every case (#620).
func UnexportedRouted(sessionID string) map[string]tea.Msg {
	return map[string]tea.Msg{
		"coverageMsg":        coverageMsg{id: sessionID},
		"generatedMsg":       generatedMsg{id: sessionID, gen: map[string]bool{}},
		"previewRestMsg":     previewRestMsg{},
		"sessionRelaunchMsg": sessionRelaunchMsg{Session: sessions.Session{ID: sessionID}},
	}
}

// TickPeriods is every tea.Tick period the model arms itself, so the message
// table can prove its "pending" deadline is well under all of them (#620
// review). spinEvery is left out: Deps.SpinTick injects that one, and the
// tests answer it at once.
//
//	for _, p := range ui.TickPeriods() { ... }
func TickPeriods() []time.Duration {
	return []time.Duration{tickEvery, statEvery, prEvery, issueEvery, previewRest, sweepCap}
}

// WaitForEvent is the re-armed Cmd that drains the status subscription, so a
// test can check an event published there arrives as a StatusMsg (#653).
func (m *Model) WaitForEvent() tea.Cmd { return m.waitForEvent() }
