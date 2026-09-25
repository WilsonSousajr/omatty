package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// paneCell turns a cell of the embedded terminal's grid into a window
// position, the way the wheel and the caret already do.
func paneCell(x, y int) (int, int) {
	ox, oy := ui.PaneOrigin()
	return ox + x, oy + y
}

// drag presses at one pane cell, moves to another and releases, returning the
// command the release produced.
func drag(m *ui.Model, x0, y0, x1, y1 int) tea.Cmd {
	px, py := paneCell(x0, y0)
	qx, qy := paneCell(x1, y1)
	m.Update(tea.MouseClickMsg{X: px, Y: py, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: qx, Y: qy, Button: tea.MouseLeft})
	_, cmd := m.Update(tea.MouseReleaseMsg{X: qx, Y: qy, Button: tea.MouseLeft})
	return cmd
}

// The host terminal's own selection is linear over the whole screen, so a
// drag that wraps a line copied the sidebar and the review column along with
// the pane. A drag omatty answers itself is clipped to the pane (#360).
func TestModel_aDragAcrossARowCopiesOnlyThePane_issue360(t *testing.T) {
	m, fakes := modelWithFakes(t)
	fakes[m.Selected()].Grid = []string{"ABCDEF", "GHIJKL"}

	msgs := drainCmd(drag(m, 2, 0, 3, 1))

	if !hasMsg(msgs, tea.SetClipboard("CDEF\nGHIJ")()) {
		t.Errorf("the drag did not copy the pane's two rows; got %v", msgs)
	}
	for _, msg := range msgs {
		if s, ok := msg.(string); ok && strings.Contains(s, "projects") {
			t.Errorf("the copy carried the sidebar: %q", s)
		}
	}
}

// A drag that runs off the pane clamps to it rather than reading the columns
// beside it (#360).
func TestModel_aDragLeavingThePaneClampsToIt_issue360(t *testing.T) {
	m, fakes := modelWithFakes(t)
	fakes[m.Selected()].Grid = []string{"ABCDEF"}
	ox, oy := ui.PaneOrigin()

	m.Update(tea.MouseClickMsg{X: ox + 1, Y: oy, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: ox - 12, Y: oy, Button: tea.MouseLeft})
	_, cmd := m.Update(tea.MouseReleaseMsg{X: ox - 12, Y: oy, Button: tea.MouseLeft})

	msgs := drainCmd(cmd)
	if !hasMsg(msgs, tea.SetClipboard("AB")()) {
		t.Errorf("a drag off the left edge did not clamp to column 0; got %v", msgs)
	}
}

// A press and release with no motion between them is a click, not a drag, so
// everything clicks already did keeps working: nothing is copied (#45, #168).
func TestModel_aClickWithoutMotionCopiesNothing_issue360(t *testing.T) {
	m, fakes := modelWithFakes(t)
	fakes[m.Selected()].Grid = []string{"ABCDEF"}
	px, py := paneCell(2, 0)

	m.Update(tea.MouseClickMsg{X: px, Y: py, Button: tea.MouseLeft})
	_, cmd := m.Update(tea.MouseReleaseMsg{X: px, Y: py, Button: tea.MouseLeft})

	for _, msg := range drainCmd(cmd) {
		if _, ok := msg.(string); ok {
			t.Errorf("a plain click copied something: %v", msg)
		}
	}
}

// A selection you cannot see is a selection you cannot trust, so the run is
// drawn in reverse video while the button is down (#360).
func TestModel_aDragInProgressIsHighlighted_issue360(t *testing.T) {
	m, _ := modelWithFakes(t)
	plain := m.View().Content

	px, py := paneCell(1, 0)
	qx, qy := paneCell(3, 0)
	m.Update(tea.MouseClickMsg{X: px, Y: py, Button: tea.MouseLeft})
	m.Update(tea.MouseMotionMsg{X: qx, Y: qy, Button: tea.MouseLeft})
	dragging := m.View().Content

	if dragging == plain {
		t.Error("the frame did not change while a drag was in progress")
	}
	if !strings.Contains(dragging, "\x1b[7m") {
		t.Error("no reverse-video run in the frame during a drag")
	}

	m.Update(tea.MouseReleaseMsg{X: qx, Y: qy, Button: tea.MouseLeft})
	if after := m.View().Content; strings.Contains(after, "\x1b[7m") {
		t.Error("the highlight outlived the drag that made it")
	}
}
