package app_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// keystroke names a message the fake terminal recorded, or "" if it is not a
// keypress.
func keystroke(msg tea.Msg) string {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		return k.Keystroke()
	}
	return ""
}

// Regression, issue #747: ctrl+o ctrl+o on a focused session reaches claude
// as one ctrl+o, which opens its transcript viewer, and runs no omatty
// command.
func TestModel_doubleLeaderReachesTheSession_issue747(t *testing.T) {
	m, fakes := modelWithFakes(t)
	before := len(fakes["s1"].Msgs)

	press(m, ctrl('o'))
	_, cmd := m.Update(ctrl('o'))

	if isQuit(cmd) || m.ModalOpen() {
		t.Fatal("ctrl+o ctrl+o ran an omatty command")
	}
	msgs := fakes["s1"].Msgs[before:]
	if len(msgs) != 1 {
		t.Fatalf("the session received %d messages, want exactly the one leader", len(msgs))
	}
	if got := keystroke(msgs[0]); got != "ctrl+o" {
		t.Errorf("the session received %q, want ctrl+o", got)
	}
}

// Issue #747, pinned either way: with a modal open the first leader closes it
// and arms (#41, #103), so the second leader is the pair's second key and
// passes through to the session the modal was over.
func TestModel_doubleLeaderInAModalClosesItThenPasses_issue747(t *testing.T) {
	m, fakes := modelWithFakes(t)
	leader(m, key('?'))
	before := len(fakes["s1"].Msgs)

	press(m, ctrl('o'))
	press(m, ctrl('o'))

	if m.ModalOpen() {
		t.Error("the modal stayed open after ctrl+o ctrl+o")
	}
	if got := len(fakes["s1"].Msgs) - before; got != 1 {
		t.Errorf("the session received %d messages, want the one leader", got)
	}
}
