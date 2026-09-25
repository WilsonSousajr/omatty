package ui

import (
	"errors"
	"testing"
)

// A snapshot that finishes after its session was archived must not bring
// the session's maps back: forgetSessionMaps has already run (#311).
func TestOnTurnSnapped_forAForgottenSessionStoresNothing_issue311(t *testing.T) {
	m := filledModel()
	m.forgetSession(forgottenID)

	m.onTurnSnapped(TurnSnappedMsg{SessionID: forgottenID, Err: errors.New("dir gone")})

	if _, ok := m.turnErr[forgottenID]; ok {
		t.Error("turnErr holds an archived session")
	}
	if _, ok := m.turnPending[forgottenID]; ok {
		t.Error("turnPending holds an archived session")
	}
}
