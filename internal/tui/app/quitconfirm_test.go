package app_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
)

// Regression, issue #749: inside a modal ctrl+c closed the whole window, where
// every shell and TUI the operator knows reads it as "cancel this field". It
// now closes the surface like esc, which still means no surface can trap the
// keyboard (#28): esc and ctrl+c both get you out.
func TestModal_ctrlCClosesInsteadOfQuitting_issue749(t *testing.T) {
	for _, tc := range []struct {
		name string
		open tea.KeyPressMsg
	}{
		{"prompt", key('n')},
		{"rename", tea.KeyPressMsg{Code: 'r', Mod: tea.ModShift, Text: "R"}},
		{"switcher", key('/')},
		{"help", key('?')},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, fakes := modelWithFakes(t)
			before := len(fakes["s1"].Msgs)
			leader(m, tc.open)
			if !m.ModalOpen() {
				t.Fatalf("ctrl+o %s opened no modal", tc.open.Keystroke())
			}

			_, cmd := m.Update(ctrl('c'))

			if isQuit(cmd) {
				t.Fatalf("ctrl+c in the %s modal quit omatty", tc.name)
			}
			if m.ModalOpen() {
				t.Errorf("ctrl+c left the %s modal open", tc.name)
			}
			if got := len(fakes["s1"].Msgs); got != before {
				t.Errorf("the session received %d messages, want %d", got, before)
			}
		})
	}
}

// quitPress is ctrl+o q, returning what q asked the runtime to do.
func quitPress(m interface {
	Update(tea.Msg) (tea.Model, tea.Cmd)
}) tea.Cmd {
	m.Update(ctrl('o'))
	_, cmd := m.Update(key('q'))
	return cmd
}

// Regression, issue #749: ctrl+o q quit at once with a session mid-turn, on
// one mistyped key after the leader. With anything to lose it now asks for a
// second q, Claude-Code-style, and says what would be lost.
func TestQuit_secondQNeededWhileASessionWorks_issue749(t *testing.T) {
	m, _ := modelWithFakes(t)
	sendStatus(m, "s1", dstatus.PromptSubmitted, time.Now())

	if isQuit(quitPress(m)) {
		t.Fatal("ctrl+o q quit at once with a session working")
	}
	if got := m.View().Content; !strings.Contains(got, "q again to quit") || !strings.Contains(got, "1 working") {
		t.Errorf("the footer does not ask for a second q and name what is working:\n%s", got)
	}
	if !isQuit(quitPress(m)) {
		t.Error("a second ctrl+o q did not quit")
	}
}

// Regression, issue #749: unsent review notes live only in memory, so they
// are the one thing quitting loses even with dtach holding the sessions.
func TestQuit_secondQNeededWithUnsentNotes_issue749(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 3)
	typeNote(m, "was this needed?")

	if isQuit(quitPress(m)) {
		t.Fatal("ctrl+o q quit at once with an unsent review note")
	}
	if got := m.View().Content; !strings.Contains(got, "1 unsent note") {
		t.Errorf("the footer does not count the unsent note:\n%s", got)
	}
}

// Issue #749: with nothing to lose, the confirmation would be noise.
func TestQuit_quitsAtOnceWhenNothingIsLost_issue749(t *testing.T) {
	m, _ := modelWithFakes(t)

	if !isQuit(quitPress(m)) {
		t.Error("ctrl+o q with every session idle and no notes did not quit")
	}
}

// Issue #749: the second q must follow the first. Any other key in between
// disarms it, so a q pressed minutes later asks again.
func TestQuit_anotherKeyDisarmsTheConfirmation_issue749(t *testing.T) {
	m, _ := modelWithFakes(t)
	sendStatus(m, "s1", dstatus.PromptSubmitted, time.Now())
	quitPress(m)

	press(m, key('x'))

	if isQuit(quitPress(m)) {
		t.Error("a q after another key quit without asking again")
	}
}
