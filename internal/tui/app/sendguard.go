package app

import (
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// atPermissionPrompt reports whether session id is waiting on a permission
// and, when it is, says in the footer why nothing was sent.
//
// A paste ends in one \r (invariant 8), and at claude's permission dialog
// that \r selects the highlighted option, "Yes": S approved the pending tool
// call, and the comments it carried were lost while marked sent (#745). The
// \r is only sent to a session that is not waiting on one.
//
//	if m.atPermissionPrompt(id) { return nil } // nothing sent, nothing marked
func (m *Model) atPermissionPrompt(id string) bool {
	if m.status[id].Status != dstatus.StatusWaiting {
		return false
	}
	m.lastErr = "the session is waiting on a permission prompt; answer it, then press S again"
	return true
}

// pasteTarget is the terminal a paste ending in \r may go to, or false with
// the reason in the footer: the session is at a permission prompt (#745), or
// it has no terminal.
//
//	term, ok := m.pasteTarget(id)
func (m *Model) pasteTarget(id string) (terminal.Terminal, bool) {
	if m.atPermissionPrompt(id) {
		return nil, false // the paste's \r would answer the dialog "Yes"
	}
	term := m.terms[id]
	if term == nil {
		m.lastErr = "session " + id + " has no terminal to send to"
		return nil, false
	}
	return term, true
}
