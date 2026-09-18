package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// #107 asked the host for the mouse and never gave it back, so a session held
// ?1002h from its first frame to its last and the terminal could never make a
// selection of its own. A leader key hands it back (#217).
func TestModel_aLeaderKeyReleasesTheMouseAndTakesItBack_issue217(t *testing.T) {
	m, _ := modelWithFakes(t)

	leader(m, key('m'))
	if got := m.View().MouseMode; got != tea.MouseModeNone {
		t.Errorf("after the toggle MouseMode = %v, want MouseModeNone", got)
	}

	leader(m, key('m'))
	if got := m.View().MouseMode; got != tea.MouseModeCellMotion {
		t.Errorf("after toggling back MouseMode = %v, want MouseModeCellMotion", got)
	}
}

// While the host owns the pointer, a stray event answers nothing: the wheel
// does not scroll and a click does not move the cursor or reach a pane.
func TestModel_aReleasedMouseAnswersNothing_issue217(t *testing.T) {
	m, fakes := modelWithFakes(t)
	leader(m, key('m'))
	x, y := overPane()
	sx, sy := overSidebar()
	before := m.View().Content

	for range ui.WheelNotchesPerPage {
		m.Update(tea.MouseWheelMsg{X: x, Y: y, Button: tea.MouseWheelDown})
	}
	m.Update(tea.MouseClickMsg{X: sx, Y: sy, Button: tea.MouseLeft})

	if after := m.View().Content; after != before {
		t.Errorf("a released mouse still changed the screen:\nbefore %q\nafter  %q", before, after)
	}
	for id, f := range fakes {
		if len(f.Sent) != 0 || len(f.Msgs) != 0 {
			t.Errorf("%s got Sent=%q Msgs=%v while the mouse was released, want nothing", id, f.Sent, f.Msgs)
		}
	}
}

// A toggle nothing says the state of is a toggle the operator cannot use. The
// marker sits in the header's sidebar share, the one piece of chrome that is
// present at every width - the footer's facts are dropped whole once the
// keymap fills a default window.
func TestModel_theHeaderSaysWhenTheMouseIsReleased_issue217(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: ui.DefaultWidth, Height: ui.DefaultHeight})

	if head := stripSGR(frameLines(m)[0]); strings.Contains(head, "mouse") {
		t.Errorf("the header marks the default state: %q", head)
	}

	leader(m, key('m'))

	if head := stripSGR(frameLines(m)[0]); !strings.Contains(head, "mouse off") {
		t.Errorf("header = %q, want it to say the mouse is released", head)
	}
	assertFrameIs(t, m, ui.DefaultWidth, ui.DefaultHeight)
}

// The keyboard is untouched while the mouse is released: the leader still
// arms and its commands still run.
func TestModel_releasingTheMouseLeavesTheKeyboardAlone_issue217(t *testing.T) {
	m, _ := modelWithFakes(t)
	leader(m, key('m'))

	leader(m, key('?'))

	if !strings.Contains(m.View().Content, "quit") {
		t.Errorf("the help modal did not open while the mouse was released:\n%s", m.View().Content)
	}
}
