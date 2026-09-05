package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// overPane is a window cell inside the focused session's pane; overSidebar is
// one outside it. Both are derived from the origin rather than written down,
// so a change to the layout moves the tests with it.
func overPane() (x, y int) {
	ox, oy := ui.PaneOrigin()
	return ox + 5, oy + 3
}

func wheel(x, y int, b tea.MouseButton) tea.MouseWheelMsg {
	return tea.MouseWheelMsg{X: x, Y: y, Button: b}
}

// The fix: a wheel notch becomes the key claude documents for scrolling.
// Before it, the notch became arrow keys via the terminal's alternate scroll
// and edited the prompt instead.
func TestUpdate_TheWheelScrollsWithClaudesOwnKeys_issue107(t *testing.T) {
	for _, tt := range []struct {
		name   string
		button tea.MouseButton
		want   string
	}{
		{"down", tea.MouseWheelDown, "\x1b[6~"},
		{"up", tea.MouseWheelUp, "\x1b[5~"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, fakes := modelWithFakes(t)
			x, y := overPane()

			m.Update(wheel(x, y, tt.button))

			if got := fakes["s1"].Sent; len(got) != 1 || got[0] != tt.want {
				t.Errorf("Sent = %q, want exactly [%q]", got, tt.want)
			}
		})
	}
}

// The pointer is over the sidebar, not the session; scrolling there must not
// scroll a pane it is not on.
func TestUpdate_TheWheelOutsideThePaneReachesNobody_issue107(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(wheel(3, 5, tea.MouseWheelDown))

	if got := fakes["s1"].Sent; len(got) != 0 {
		t.Errorf("Sent = %q for a notch over the sidebar, want nothing", got)
	}
}

// The trap this fix exists to avoid: Update's default arm ends in broadcast,
// which would hand an untranslated mouse event to every emulator at once.
func TestUpdate_MouseEventsNeverReachAnUnfocusedSession_issue107(t *testing.T) {
	m, fakes := modelWithFakes(t)
	x, y := overPane()

	m.Update(wheel(x, y, tea.MouseWheelDown))

	for _, id := range []string{"s2", "s3"} {
		if got := fakes[id].Sent; len(got) != 0 {
			t.Errorf("unfocused %s was sent %q, want nothing", id, got)
		}
		if got := fakes[id].Msgs; len(got) != 0 {
			t.Errorf("unfocused %s got messages %v, want none", id, got)
		}
	}
}

// Clicks and drags belong to #45, which needs sidebar hit-testing. Until then
// they must be dropped rather than broadcast into every PTY.
func TestUpdate_ClicksAreDroppedRatherThanBroadcast_issue107(t *testing.T) {
	m, fakes := modelWithFakes(t)
	x, y := overPane()

	m.Update(tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: x, Y: y})

	for id, f := range fakes {
		if len(f.Sent) != 0 || len(f.Msgs) != 0 {
			t.Errorf("%s got Sent=%q Msgs=%v from a click, want nothing", id, f.Sent, f.Msgs)
		}
	}
}

// Nothing arrives at all unless omatty asks the host terminal for it, and
// asking is also what stops alternate scroll turning the wheel into arrows.
func TestView_EnablesMouseReportingSoTheWheelArrives_issue107(t *testing.T) {
	m, _ := modelWithFakes(t)

	if got := m.View().MouseMode; got != tea.MouseModeCellMotion {
		t.Errorf("View().MouseMode = %v, want MouseModeCellMotion", got)
	}
}

// pgup/pgdn always reached claude - bubbleterm translates them and the router
// forwards them - but nothing said so, which is why the pane looked unable to
// scroll at all. The help modal is where a key that omatty does not own gets
// written down.
func TestModel_helpNamesTheKeysThatScrollTheSession_issue107(t *testing.T) {
	m, _ := modelWithFakes(t)

	press(m, ctrl('o'))
	press(m, key('?'))

	got := m.View().Content
	for _, want := range []string{"pgup / pgdn", "shift+drag"} {
		if !strings.Contains(got, want) {
			t.Errorf("the help modal does not name %q:\n%s", want, got)
		}
	}
}

// A modal owns the pane's surface, so there is no transcript under the
// pointer to scroll.
func TestUpdate_TheWheelIsIgnoredBehindAModal_issue107(t *testing.T) {
	m, fakes := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('?'))
	x, y := overPane()

	m.Update(wheel(x, y, tea.MouseWheelDown))

	if got := fakes["s1"].Sent; len(got) != 0 {
		t.Errorf("Sent = %q with a modal open, want nothing", got)
	}
}
