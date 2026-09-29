package ui

import "fmt"

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
