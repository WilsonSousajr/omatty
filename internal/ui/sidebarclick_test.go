package ui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func clickAt(x, y int) tea.MouseClickMsg { return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft} }
func sidebarRowY(i int) int              { return ui.SidebarTop() + i }

// Pinned as a literal, the way TestPaneOrigin_IsTheEmulatorsTopLeftCell pins
// its origin: a hit test asserted against the function under test cannot
// fail (#45).
func TestSidebarTop_IsTheFirstScrollableRow_issue45(t *testing.T) {
	if ui.SidebarTop() != 2 {
		t.Errorf("SidebarTop() = %d, want 2 (border, then the projects line)", ui.SidebarTop())
	}
}

// Rows in twoProjectState: omatty, s1, s2, api-svc, s3.
func TestUpdate_AClickOnASidebarRowSelectsItAndResizesItsTerminal_issue45(t *testing.T) {
	m, fakes := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	_, cmd := m.Update(clickAt(4, sidebarRowY(4)))
	settle(m, cmd)

	if m.Selected() != "s3" {
		t.Fatalf("click selected %q, want s3", m.Selected())
	}
	w, h := ui.PTYSize(100, 30, false)
	if f := fakes["s3"]; f.Width != w || f.Height != h {
		t.Errorf("s3 is %dx%d after the click, want %dx%d (#73)", f.Width, f.Height, w, h)
	}
}

func TestUpdate_AClickMovesAnOpenReviewColumnWithIt_issue45(t *testing.T) {
	m, _, rec := modelWithDiff(t)
	leader(m, key('d'))

	_, cmd := m.Update(clickAt(4, sidebarRowY(2)))
	deliver(m, cmd)

	if len(rec.Asked) != 2 || rec.Asked[1] != "s2" {
		t.Errorf("diff asked for %v, want s1 then s2", rec.Asked)
	}
}

func TestUpdate_AClickOnAProjectHeaderOrBelowTheListChangesNothing_issue45(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	for _, y := range []int{sidebarRowY(0), sidebarRowY(3), sidebarRowY(12), 0} {
		m.Update(clickAt(4, y))
		if m.Selected() != "s1" {
			t.Errorf("click at y=%d selected %q, want s1 unchanged", y, m.Selected())
		}
	}
}

func TestUpdate_AClickOverThePaneOrBehindAModalOrWithTheRightButtonSelectsNothing_issue45(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	x, y := overPane()
	m.Update(clickAt(x, y))
	m.Update(tea.MouseClickMsg{X: 4, Y: sidebarRowY(4), Button: tea.MouseRight})
	leader(m, key('?'))
	m.Update(clickAt(4, sidebarRowY(4)))
	if m.Selected() != "s1" {
		t.Errorf("Selected() = %q, want s1", m.Selected())
	}
}

func TestUpdate_AClickOnTheSelectedRowResizesNothing_issue45(t *testing.T) {
	m, fakes := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	before := fakes["s1"].Width

	_, cmd := m.Update(clickAt(4, sidebarRowY(1)))
	settle(m, cmd)

	if fakes["s1"].Width != before {
		t.Error("clicking the selected row re-resized its terminal")
	}
}

// The test that fails if the hit test forgets the scroll offset (#129).
func TestUpdate_AClickOnAScrolledSidebarSelectsTheRowUnderThePointer_issue45(t *testing.T) {
	st := sevenProjectState()
	m := ui.NewModel(baseDeps(st, fakeTermsFor(st)))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	for range 13 {
		leader(m, key('j'))
	}
	m.View() // draw once, so the offset is the drawn one

	// Cursor on row 20 of 21 with 16 rows drawn puts the offset at 5, so the
	// third drawn row is row 7: p2-s0. Without the offset it would be p0-s1.
	_, cmd := m.Update(clickAt(4, sidebarRowY(2)))
	settle(m, cmd)

	if got := m.Selected(); got != "p2-s0" {
		t.Errorf("click on the third drawn row selected %q, want p2-s0; the offset was ignored", got)
	}
}
