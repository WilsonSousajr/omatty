package ui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// caretModel focuses s1 and gives it a caret at (x, y) in the emulator's own
// grid, which is the only input View needs to place a cursor.
func caretModel(t *testing.T, c termwrap.Caret) *ui.Model {
	t.Helper()
	m, fakes := modelWithFakes(t)
	fakes["s1"].Caret = c
	return m
}

// The bug: no cursor was ever drawn, so Claude's prompt gave no caret. The
// emulator's cell must land at the pane's origin plus its own position.
func TestView_PlacesTheEmbeddedCursorOnTheWindow_issue106(t *testing.T) {
	m := caretModel(t, termwrap.Caret{X: 7, Y: 3, Visible: true})

	got := m.View().Cursor

	if got == nil {
		t.Fatal("View().Cursor = nil, want the embedded terminal's cursor")
	}
	// The expected cell is spelled out rather than taken from PaneOrigin
	// itself. Asserting against the function under test cannot fail: rewriting
	// PaneOrigin to return 0, 0 - which draws the caret in the sidebar's top
	// corner - left every issue106 test green (#106).
	if got.X != ui.SidebarWidth+1+7 || got.Y != 2+3 {
		t.Errorf("cursor at (%d, %d), want (%d, %d)",
			got.X, got.Y, ui.SidebarWidth+1+7, 2+3)
	}
}

// PaneOrigin is where the caret and the wheel both measure from, so its value
// is pinned here rather than only derived. The sidebar box, then the pane
// box's left border; the pane box's top border, then its title row.
func TestPaneOrigin_IsTheEmulatorsTopLeftCell_issue106(t *testing.T) {
	x, y := ui.PaneOrigin()

	if x != ui.SidebarWidth+1 || y != 2 {
		t.Errorf("PaneOrigin() = (%d, %d), want (%d, 2)", x, y, ui.SidebarWidth+1)
	}
}

// DECTCEM is the application's call: claude hides the cursor while it paints,
// and omatty must not draw one it has been told to hide.
func TestView_DrawsNoCursorWhileTheEmulatorHidesIt_issue106(t *testing.T) {
	m := caretModel(t, termwrap.Caret{X: 7, Y: 3, Visible: false})

	if got := m.View().Cursor; got != nil {
		t.Errorf("View().Cursor = %+v with the emulator's cursor hidden, want nil", got)
	}
}

// A modal takes the pane's whole surface, so a caret from the terminal behind
// it would be drawn over someone else's frame.
func TestView_DrawsNoCursorBehindAModal_issue106(t *testing.T) {
	m := caretModel(t, termwrap.Caret{X: 7, Y: 3, Visible: true})

	press(m, ctrl('o'))
	press(m, key('?'))

	if got := m.View().Cursor; got != nil {
		t.Errorf("View().Cursor = %+v with the help modal open, want nil", got)
	}
}

// The dimmed border says keystrokes land in the review column; a live caret
// in the terminal would contradict it (#21).
func TestView_DrawsNoCursorWhileTheReviewColumnHasFocus_issue106(t *testing.T) {
	m := caretModel(t, termwrap.Caret{X: 7, Y: 3, Visible: true})

	press(m, ctrl('o'))
	press(m, key('d'))

	if got := m.View().Cursor; got != nil {
		t.Errorf("View().Cursor = %+v with the review column focused, want nil", got)
	}
}

// fitBlock cuts every cell past the pane, so a cursor there would be drawn
// against content that is not on screen.
func TestView_DrawsNoCursorOutsideThePane_issue106(t *testing.T) {
	// PTYSize, not PaneSize: the bound the code actually uses. Deriving the
	// first out-of-range row as PaneSize's h-1 relied on the two differing by
	// exactly one, which the test never said and nothing enforced (#106).
	w, h := ui.PTYSize(ui.DefaultWidth, ui.DefaultHeight, false)
	for _, tt := range []struct {
		name  string
		caret termwrap.Caret
	}{
		{"past the right edge", termwrap.Caret{X: w, Y: 0, Visible: true}},
		{"past the bottom", termwrap.Caret{X: 0, Y: h, Visible: true}},
		{"negative", termwrap.Caret{X: -1, Y: 0, Visible: true}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := caretModel(t, tt.caret)

			if got := m.View().Cursor; got != nil {
				t.Errorf("View().Cursor = %+v for %+v, want nil", got, tt.caret)
			}
		})
	}
}

// The shape claude asks for via DECSCUSR is what tells a bar caret from a
// block one, which is the whole point of seeing it.
func TestView_CarriesTheEmulatorsCursorShape_issue106(t *testing.T) {
	m := caretModel(t, termwrap.Caret{X: 1, Y: 1, Visible: true, Shape: tea.CursorBar, Blink: true})

	got := m.View().Cursor

	if got == nil {
		t.Fatal("View().Cursor = nil, want a cursor")
	}
	if got.Shape != tea.CursorBar || !got.Blink {
		t.Errorf("cursor shape = %v blink = %v, want CursorBar blinking", got.Shape, got.Blink)
	}
}

// The complement of DrawsNoCursorOutsideThePane: the last cell that *is* in
// range must still get a caret. Without this, tightening the bound by one -
// hiding the caret on the pane's rightmost column and bottom row, which is
// exactly where Claude's prompt sits after a long line - passes every test.
func TestView_DrawsTheCursorOnThePanesLastCell_issue106(t *testing.T) {
	w, h := ui.PTYSize(ui.DefaultWidth, ui.DefaultHeight, false)
	m := caretModel(t, termwrap.Caret{X: w - 1, Y: h - 1, Visible: true})

	got := m.View().Cursor

	if got == nil {
		t.Fatalf("View().Cursor = nil for the last in-range cell (%d, %d)", w-1, h-1)
	}
}

// Regression, issue #106: on exit claude restores DECTCEM and leaves the alt
// screen, so the emulator goes on reporting a visible cursor and nothing
// panicked. A blinking caret over a frozen pane invites typing into a PTY
// nobody is reading.
func TestView_DrawsNoCursorForAnExitedSession_issue106(t *testing.T) {
	m := caretModel(t, termwrap.Caret{X: 7, Y: 3, Visible: true})
	m.Update(ui.StatusMsg{SessionID: m.Focused(), Kind: watcher.SessionEnded})

	if got := m.View().Cursor; got != nil {
		t.Errorf("View().Cursor = %+v for an exited session, want nil", got)
	}
}
