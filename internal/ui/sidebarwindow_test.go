package ui_test

import (
	"fmt"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func sevenProjectState() sessions.State {
	var st sessions.State
	for p := range 7 {
		name := fmt.Sprintf("p%d", p)
		st.Projects = append(st.Projects, sessions.Project{Name: name})
		for i := range 2 {
			st.Sessions = append(st.Sessions, sessions.Session{
				ID: fmt.Sprintf("%s-s%d", name, i), Project: name, Title: "t"})
		}
	}
	return st
}

// Ten lines fit, and the cursor's card must be among them whatever moved it
// (#129, #176). With three-line cards (#230) that is rows 14..17: p4-s1 (3) +
// p5's header (1) + p5-s0 (3) + p5-s1 (3), exactly ten. The offset moves as
// little as it can, so row 13 would not fit and 14 is where it stops.
func TestSidebar_WindowFollowsTheCursor_issue129(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(sevenProjectState(), nil))
	s.SelectByID("p5-s1") // row 17

	win := s.Window(10)

	if len(win) != 4 || s.Offset() != 14 {
		t.Fatalf("Window(10) = %d rows from offset %d, want 4 from 14", len(win), s.Offset())
	}
	found := false
	for _, r := range win {
		found = found || (r.Session != nil && r.Session.ID == "p5-s1")
	}
	if !found {
		t.Errorf("the selected row is not in the window: %+v", win)
	}
}

func TestSidebar_WindowScrollsBackToZeroAtTheTop_issue129(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(sevenProjectState(), nil))
	s.SelectByID("p5-s1")
	s.Window(10)
	s.SelectByID("p0-s0")

	s.Window(10)

	if s.Offset() != 0 {
		t.Errorf("Offset() = %d after returning to the top, want 0", s.Offset())
	}
}

// Fewer rows than fit: nothing scrolls, the whole list is the window.
func TestSidebar_WindowIsTheWholeListWhenItFits_issue129(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(twoProjectState(), nil))
	if got := s.Window(20); len(got) != 5 || s.Offset() != 0 {
		t.Errorf("Window(20) = %d rows offset %d, want 5 and 0", len(got), s.Offset())
	}
}

// A shrink after the last move: the offset is recomputed, not trusted.
func TestSidebar_WindowShrinkingKeepsTheCursorVisible_issue129(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(sevenProjectState(), nil))
	s.SelectByID("p3-s0") // row 10
	s.Window(20)          // fits, offset 0

	win := s.Window(5)

	// Rows 9 and 10 are p3's header (1) and p3-s0 (3): four of the five lines.
	// Row 11 is another three-line card and does not fit, so the window stops.
	if s.Offset() != 9 || len(win) != 2 {
		t.Errorf("Window(5) after shrinking: offset %d len %d, want 9 and 2", s.Offset(), len(win))
	}
}

// Renamed from _ACardIsTwoLines_ when M9 made it three (#230). The height it
// asserted was right for M8; what actually matters, and always did, is that a
// card is cardLines and a header is one - the window math and the click
// inverse both read that, so the two cannot drift.
func TestRowHeight_ACardIsCardLinesAndAHeaderOne_issue176(t *testing.T) {
	rows := ui.SidebarRows(twoProjectState(), nil)
	if ui.RowHeight(rows[0]) != 1 || ui.RowHeight(rows[1]) != ui.CardLines() {
		t.Errorf("heights = %d, %d; want 1 for a header and %d for a session",
			ui.RowHeight(rows[0]), ui.RowHeight(rows[1]), ui.CardLines())
	}
}

// Whatever the cursor and the budget, the selected card is drawn whole and
// the drawn rows never take more lines than fit.
func TestSidebar_WindowNeverSplitsTheSelectedCard_issue176(t *testing.T) {
	rows := ui.SidebarRows(sevenProjectState(), nil)
	for lines := 3; lines <= 12; lines++ {
		for i := range rows {
			if rows[i].Session == nil {
				continue
			}
			s := ui.NewSidebar(rows)
			s.SelectByID(rows[i].Session.ID)
			win, used, seen := s.Window(lines), 0, false
			for _, r := range win {
				used += ui.RowHeight(r)
				seen = seen || (r.Session != nil && r.Session.ID == rows[i].Session.ID)
			}
			if !seen || used > lines {
				t.Errorf("lines=%d cursor=%d: drawn %d lines, selected drawn=%v", lines, i, used, seen)
			}
		}
	}
}

// The click inverse walks the same heights the window drew with, so every
// line of a card maps back to that card.
//
// The expectation is derived from RowHeight rather than written out, because
// the table of magic line numbers this used to carry had to be recomputed by
// hand the moment cardLines changed (#230) - which is exactly the drift the
// shared height exists to prevent.
func TestSidebar_RowAtLineWalksTheDrawnHeights_issue176(t *testing.T) {
	rows := ui.SidebarRows(twoProjectState(), nil)
	s := ui.NewSidebar(rows)
	s.Window(40)

	line, total := 0, 0
	for want, row := range rows {
		for range ui.RowHeight(row) {
			if got, ok := s.RowAtLine(line); !ok || got != want {
				t.Errorf("RowAtLine(%d) = %d, %v; want %d", line, got, ok, want)
			}
			line++
		}
		total += ui.RowHeight(row)
	}
	for _, past := range []int{-1, total, total + 32} {
		if _, ok := s.RowAtLine(past); ok {
			t.Errorf("RowAtLine(%d) = ok, want false past the list", past)
		}
	}
}

func TestSidebar_WindowWithNoSessionsIsSafe_issue129(t *testing.T) {
	s := ui.NewSidebar(nil)
	if got := s.Window(5); len(got) != 0 || s.Offset() != 0 {
		t.Errorf("Window(5) on nothing = %d rows offset %d", len(got), s.Offset())
	}
}

// revealHeader's courtesy must never cost the cursor its own card. A pane
// exactly one header and one card tall drew the header alone: the offset was
// pulled back to reveal the project name, and the card under it no longer fit.
//
// Latent since #129 - it needs a budget of exactly cardLines+1, and the old
// two-line card made that 3, which is where the loop in
// WindowNeverSplitsTheSelectedCard started. #230's three-line card moved the
// boundary into the range the loop covers, which is how it surfaced.
func TestSidebar_revealHeaderNeverHidesTheSelectedCard_issue244(t *testing.T) {
	rows := ui.SidebarRows(twoProjectState(), nil)
	s := ui.NewSidebar(rows)
	s.SelectByID("s1") // row 1: the first session under omatty's header

	win := s.Window(ui.CardLines()) // room for the card, and not for the header too

	drawn := false
	for _, r := range win {
		drawn = drawn || (r.Session != nil && r.Session.ID == "s1")
	}
	if !drawn {
		t.Errorf("Window(%d) = %+v; the selected card was dropped to make room for its header",
			ui.CardLines(), win)
	}
}

// And it still does the courtesy when both fit, which is the whole point of
// #129: moving up onto a project's first session should name the project.
func TestSidebar_revealHeaderStillShowsTheHeaderWhenItFits_issue244(t *testing.T) {
	rows := ui.SidebarRows(twoProjectState(), nil)
	s := ui.NewSidebar(rows)
	s.SelectByID("s1")

	s.Window(ui.CardLines() + 1)

	if s.Offset() != 0 {
		t.Errorf("Offset() = %d, want 0: the header fits alongside the card and should be shown", s.Offset())
	}
}
