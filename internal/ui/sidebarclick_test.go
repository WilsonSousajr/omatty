package ui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func clickAt(x, y int) tea.MouseClickMsg { return tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft} }

// sidebarLineY is the window row of a drawn sidebar line. Lines, not rows: a
// card is two of them (#176). In twoProjectState line 0 is omatty's header,
// 1-2 are s1, 3-4 are s2, 5 is api-svc, 6-7 are s3.
func sidebarLineY(line int) int { return ui.SidebarTop() + line }

// Pinned as a literal, the way TestPaneOrigin_IsTheEmulatorsTopLeftCell pins
// its origin: a hit test asserted against the function under test cannot
// fail (#45).
func TestSidebarTop_IsTheFirstScrollableRow_issue45(t *testing.T) {
	if ui.SidebarTop() != 2 {
		t.Errorf("SidebarTop() = %d, want 2 (the header row and the rule, #174)", ui.SidebarTop())
	}
}

// The click lands on s3's last line: any line of a card is the card.
//
// The line is computed from cardLines rather than written out, so the
// assertion follows the renderer instead of being rechased whenever a card
// changes height (#230).
func TestUpdate_AClickOnASidebarRowSelectsItAndResizesItsTerminal_issue45(t *testing.T) {
	m, fakes := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	_, cmd := m.Update(clickAt(4, sidebarLineY(lastLineOf(2))))
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

	_, cmd := m.Update(clickAt(4, sidebarLineY(lastLineOf(1))))
	deliver(m, cmd)

	if len(rec.Asked) != 2 || rec.Asked[1] != "s2" {
		t.Errorf("diff asked for %v, want s1 then s2", rec.Asked)
	}
}

func TestUpdate_AClickOnAProjectHeaderOrBelowTheListChangesNothing_issue45(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	// The two headers and a line past the end of the list.
	for _, y := range []int{sidebarLineY(0), sidebarLineY(headerLineOf(1)), sidebarLineY(pastTheList()), 0} {
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
	m.Update(tea.MouseClickMsg{X: 4, Y: sidebarLineY(6), Button: tea.MouseRight})
	leader(m, key('?'))
	m.Update(clickAt(4, sidebarLineY(6)))
	if m.Selected() != "s1" {
		t.Errorf("Selected() = %q, want s1", m.Selected())
	}
}

func TestUpdate_AClickOnTheSelectedRowResizesNothing_issue45(t *testing.T) {
	m, fakes := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	before := fakes["s1"].Width

	_, cmd := m.Update(clickAt(4, sidebarLineY(1)))
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

	// Which row sits on a drawn line depends on where the window starts, and
	// that is the whole point of the test - so the target is resolved through
	// the drawn offset rather than written out. A literal here had to be
	// recomputed whenever a card changed height (#230), which is the same
	// drift the shared rowHeight exists to prevent.
	rows := ui.SidebarRows(st, nil)
	line, want := firstDrawnSession(rows, m.SidebarOffset())
	if want == "" {
		t.Fatal("no session row is drawn; the fixture or the window changed")
	}

	_, cmd := m.Update(clickAt(4, sidebarLineY(line)))
	settle(m, cmd)

	if got := m.Selected(); got != want {
		t.Errorf("click on drawn line %d selected %q, want %q; the offset was ignored", line, got, want)
	}
}

// firstDrawnSession walks the drawn rows from offset and returns the last line
// of the first session card, with that session's id. The last line rather than
// the first, so the test also proves any line of a card is the card.
func firstDrawnSession(rows []ui.Row, offset int) (int, string) {
	line := 0
	for i := offset; i < len(rows); i++ {
		height := ui.RowHeight(rows[i])
		if rows[i].Session != nil {
			return line + height - 1, rows[i].Session.ID
		}
		line += height
	}
	return 0, ""
}

// The sidebar in these tests is twoProjectState: omatty's header, s1, s2,
// api-svc's header, s3. These three turn a card index into the line it
// occupies, so the assertions follow cardLines rather than hard-coding it.

// lastLineOf is the bottom line of the nth session card (0-based across the
// whole list): s0 is the first, s1 the second, s2 the third.
func lastLineOf(card int) int {
	switch card {
	case 0, 1: // s1 and s2, under omatty's header
		return 1 + (card+1)*ui.CardLines() - 1
	default: // s3, under api-svc's header
		return headerLineOf(1) + (card-1)*ui.CardLines()
	}
}

// headerLineOf is the line the nth project header sits on.
func headerLineOf(project int) int {
	if project == 0 {
		return 0
	}
	return 1 + 2*ui.CardLines()
}

// pastTheList is a line below every row, where a click must change nothing.
func pastTheList() int { return headerLineOf(1) + 3*ui.CardLines() }
