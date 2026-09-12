package ui_test

import (
	"fmt"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func twoProjectState() registry.State {
	return registry.State{
		Projects: []registry.Project{
			{Name: "omatty", Root: "/p/omatty"},
			{Name: "api-svc", Root: "/p/api-svc"},
		},
		Sessions: []registry.Session{
			{ID: "s1", Project: "omatty", Title: "main"},
			{ID: "s2", Project: "omatty", Title: "parser-fix"},
			{ID: "s3", Project: "api-svc", Title: "main"},
		},
	}
}

func TestSidebarRows_GroupsSessionsUnderTheirProject(t *testing.T) {
	rows := ui.SidebarRows(twoProjectState(), map[string]watcher.Status{
		"s2": watcher.StatusThinking,
	})

	want := []string{"omatty", "s1", "s2", "api-svc", "s3"}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %+v", len(rows), len(want), rows)
	}
	for i, w := range want {
		got := rows[i].Project
		if rows[i].Session != nil {
			got = rows[i].Session.ID
		}
		if got != w {
			t.Errorf("row %d = %q, want %q", i, got, w)
		}
	}
	if rows[2].Status != watcher.StatusThinking {
		t.Errorf("row 2 status = %q, want %q", rows[2].Status, watcher.StatusThinking)
	}
	if rows[1].Status != watcher.StatusIdle {
		t.Errorf("row 1 status = %q, want %q for an unreported session",
			rows[1].Status, watcher.StatusIdle)
	}
}

// Each row must point at a distinct session: taking &sess of a range variable
// would make every row alias the last one.
func TestSidebarRows_EachRowPointsAtItsOwnSession(t *testing.T) {
	rows := ui.SidebarRows(twoProjectState(), nil)

	seen := map[string]bool{}
	for _, r := range rows {
		if r.Session == nil {
			continue
		}
		if seen[r.Session.ID] {
			t.Errorf("session %q appears twice; rows alias one another", r.Session.ID)
		}
		seen[r.Session.ID] = true
	}
	if len(seen) != 3 {
		t.Errorf("saw %d distinct sessions, want 3", len(seen))
	}
}

func TestSidebarRows_ProjectWithNoSessionsStillShows(t *testing.T) {
	st := registry.State{Projects: []registry.Project{{Name: "empty", Root: "/p/empty"}}}

	rows := ui.SidebarRows(st, nil)

	if len(rows) != 1 || rows[0].Project != "empty" || rows[0].Session != nil {
		t.Errorf("rows = %+v, want a single header row for %q", rows, "empty")
	}
}

func TestSidebar_CursorSkipsProjectHeadersAndWraps_issue126(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(twoProjectState(), nil))

	got, ok := s.Selected()
	if !ok || got.Session == nil || got.Session.ID != "s1" {
		t.Fatalf("Selected() = %+v (ok=%v), want session s1", got, ok)
	}
	s.MoveDown()
	if got, _ := s.Selected(); got.Session.ID != "s2" {
		t.Errorf("after MoveDown, Selected() = %q, want s2", got.Session.ID)
	}
	s.MoveDown()
	if got, _ := s.Selected(); got.Session.ID != "s3" {
		t.Errorf("after two MoveDowns, Selected() = %q, want s3 (header skipped)", got.Session.ID)
	}
	s.MoveDown()
	if got, _ := s.Selected(); got.Session.ID != "s1" {
		t.Errorf("MoveDown at the end moved to %q, want to wrap to s1 (#126)", got.Session.ID)
	}
	s.MoveUp()
	if got, _ := s.Selected(); got.Session.ID != "s3" {
		t.Errorf("MoveUp at the start moved to %q, want to wrap to s3 (#126)", got.Session.ID)
	}
	s.MoveUp()
	if got, _ := s.Selected(); got.Session.ID != "s2" {
		t.Errorf("after MoveUp, Selected() = %q, want s2 (header skipped)", got.Session.ID)
	}
}

func TestSidebar_EmptyStateSelectsNothing(t *testing.T) {
	s := ui.NewSidebar(nil)

	if _, ok := s.Selected(); ok {
		t.Error("Selected() on an empty sidebar returned ok=true, want false")
	}
	s.MoveDown()
	s.MoveUp()
	if _, ok := s.Selected(); ok {
		t.Error("Selected() after moving on an empty sidebar returned ok=true, want false")
	}
}

// A project with no sessions must not strand the cursor before the sessions
// that follow it.
func TestSidebar_HeaderOnlyProjectFirstStillSelectsALaterSession(t *testing.T) {
	st := registry.State{
		Projects: []registry.Project{{Name: "empty"}, {Name: "omatty"}},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: "main"}},
	}

	s := ui.NewSidebar(ui.SidebarRows(st, nil))

	got, ok := s.Selected()
	if !ok || got.Session == nil || got.Session.ID != "s1" {
		t.Errorf("Selected() = %+v (ok=%v), want s1 past the empty project", got, ok)
	}
}

func emptyState() registry.State { return registry.State{} }

func TestSidebar_MoveDownFromTheLastSessionWrapsToTheFirst_issue126(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(twoProjectState(), nil))
	s.SelectByID("s3")

	s.MoveDown()

	if got, _ := s.Selected(); got.Session.ID != "s1" {
		t.Errorf("MoveDown from the last session selected %q, want s1 past both headers", got.Session.ID)
	}
}

func TestSidebar_MoveUpFromTheFirstSessionWrapsToTheLast_issue126(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(twoProjectState(), nil))

	s.MoveUp()

	if got, _ := s.Selected(); got.Session.ID != "s3" {
		t.Errorf("MoveUp from the first session selected %q, want s3", got.Session.ID)
	}
}

func TestSidebar_OneSessionStaysPutInBothDirections_issue126(t *testing.T) {
	st := registry.State{
		Projects: []registry.Project{{Name: "omatty"}},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: "main"}},
	}
	s := ui.NewSidebar(ui.SidebarRows(st, nil))
	s.MoveDown()
	s.MoveUp()
	if got, ok := s.Selected(); !ok || got.Session.ID != "s1" {
		t.Errorf("Selected() = %+v ok=%v, want s1", got, ok)
	}
}

// A sidebar of headers only has no session to select. The cursor now rests on
// the first header (#158), but Selected() is still ok=false there; and the
// lap bound is still what stops a modulo walk spinning here forever.
func TestSidebar_HeadersOnlyReturnsWithoutSelecting_issue126(t *testing.T) {
	st := registry.State{Projects: []registry.Project{{Name: "empty"}, {Name: "also"}}}
	s := ui.NewSidebar(ui.SidebarRows(st, nil))

	s.MoveDown()
	s.MoveUp()

	if _, ok := s.Selected(); ok {
		t.Error("Selected() ok=true on a sidebar of headers only")
	}
}

func sevenProjectState() registry.State {
	var st registry.State
	for p := range 7 {
		name := fmt.Sprintf("p%d", p)
		st.Projects = append(st.Projects, registry.Project{Name: name})
		for i := range 2 {
			st.Sessions = append(st.Sessions, registry.Session{
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

func threeProjectState() registry.State {
	return registry.State{
		Projects: []registry.Project{{Name: "a"}, {Name: "b"}, {Name: "none"}, {Name: "c"}},
		Sessions: []registry.Session{
			{ID: "a1", Project: "a", Title: "t"}, {ID: "a2", Project: "a", Title: "t"},
			{ID: "b1", Project: "b", Title: "t"}, {ID: "b2", Project: "b", Title: "t"}, {ID: "b3", Project: "b", Title: "t"},
			{ID: "c1", Project: "c", Title: "t"},
		},
	}
}

// landedOn is where a project jump ended: a session id, or the name of the
// empty project whose header the cursor rests on (#158).
func landedOn(s *ui.Sidebar) string {
	if row, ok := s.Selected(); ok {
		return row.Session.ID
	}
	p, _ := s.SelectedHeader()
	return p
}

// The "none" project has no sessions and sits between b and c. #130 specified
// that both directions skip it; that skip was the whole of #158 - a project
// nothing can land on is a project nothing can create a session in - so "none"
// is now a stop in both directions, on its header.
func TestSidebar_NextProjectLandsOnTheNextProjectsFirstSession_issue130_issue158(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(threeProjectState(), nil))
	for _, tt := range []struct{ from, want string }{
		{"a1", "b1"}, {"b2", "none"}, {"c1", "a1"},
	} {
		s.SelectByID(tt.from)
		s.NextProject()
		if got := landedOn(s); got != tt.want {
			t.Errorf("NextProject from %s = %s, want %s", tt.from, got, tt.want)
		}
	}
}

func TestSidebar_PrevProjectLandsOnTheFirstSessionNotTheLast_issue130_issue158(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(threeProjectState(), nil))
	for _, tt := range []struct{ from, want string }{
		{"b2", "a1"}, {"a1", "c1"}, {"c1", "none"},
	} {
		s.SelectByID(tt.from)
		s.PrevProject()
		if got := landedOn(s); got != tt.want {
			t.Errorf("PrevProject from %s = %s, want %s", tt.from, got, tt.want)
		}
	}
}

// Two projects, the second empty: ] visits it and [ comes back to the first
// project's first session (#130's "first, not last" rule). Before #158 this
// test asserted the pair was a no-op, which is exactly the dead row the issue
// reports.
func TestSidebar_ProjectJumpVisitsTheEmptyProject_issue130_issue158(t *testing.T) {
	st := registry.State{
		Projects: []registry.Project{{Name: "only"}, {Name: "empty"}},
		Sessions: []registry.Session{{ID: "o1", Project: "only", Title: "t"}, {ID: "o2", Project: "only", Title: "t"}},
	}
	s := ui.NewSidebar(ui.SidebarRows(st, nil))
	s.SelectByID("o2")
	s.NextProject()
	if got := landedOn(s); got != "empty" {
		t.Fatalf("NextProject from o2 = %s, want the empty project's header", got)
	}
	s.PrevProject()
	if got := landedOn(s); got != "o1" {
		t.Errorf("PrevProject from the empty project = %s, want o1", got)
	}
}

// Regression, issue #158: a project with no sessions was drawn and could not
// be selected, so SelectedProject could never name it and no first session
// could ever be created there. The cursor may now rest on such a project's
// header - and only such a project's: one with sessions is represented by its
// first session, as before.
func TestSidebar_NextProjectLandsOnAnEmptyProjectsHeader_issue158(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(threeProjectState(), nil))
	s.SelectByID("b2")

	s.NextProject()

	if p, ok := s.SelectedHeader(); !ok || p != "none" {
		t.Fatalf("SelectedHeader() = %q, %v; want the empty project none", p, ok)
	}
	if _, ok := s.Selected(); ok {
		t.Error("Selected() ok=true on a header; a header has no session to act on")
	}
	if got := s.CursorProject(); got != "none" {
		t.Errorf("CursorProject() = %q, want none", got)
	}
	s.NextProject()
	if got, _ := s.Selected(); got.Session == nil || got.Session.ID != "c1" {
		t.Errorf("NextProject past the empty project landed on %+v, want c1", got)
	}
}

func TestSidebar_PrevProjectLandsOnAnEmptyProjectsHeader_issue158(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(threeProjectState(), nil))
	s.SelectByID("c1")

	s.PrevProject()

	if p, ok := s.SelectedHeader(); !ok || p != "none" {
		t.Errorf("SelectedHeader() = %q, %v; want none", p, ok)
	}
	s.PrevProject()
	if got, _ := s.Selected(); got.Session == nil || got.Session.ID != "b1" {
		t.Errorf("PrevProject past the empty project landed on %+v, want b1", got)
	}
}

// j and k walk through an empty project too, so the two axes agree on what
// exists.
func TestSidebar_MoveDownStopsOnAnEmptyProjectsHeader_issue158(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(threeProjectState(), nil))
	s.SelectByID("b3")

	s.MoveDown()

	if p, ok := s.SelectedHeader(); !ok || p != "none" {
		t.Errorf("after MoveDown from b3, SelectedHeader() = %q, %v; want none", p, ok)
	}
	s.MoveUp()
	if got, _ := s.Selected(); got.Session == nil || got.Session.ID != "b3" {
		t.Errorf("MoveUp from the header landed on %+v, want b3", got)
	}
}

// A header with sessions beneath it is still not a landing row: its first
// session stands for it, as it did before (#126, #130).
func TestSidebar_AHeaderWithSessionsIsStillSkipped_issue158(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(twoProjectState(), nil))
	s.SelectByID("s2")

	s.MoveDown()

	if got, ok := s.Selected(); !ok || got.Session.ID != "s3" {
		t.Errorf("MoveDown from s2 = %+v (ok=%v), want s3 with api-svc's header skipped", got, ok)
	}
}

// Archiving a project's last session must not strand the project: the
// rebuilt sidebar keeps the cursor in that project, on its header.
func TestSidebar_SetRowsStaysInTheProjectWhoseLastSessionWent_issue158(t *testing.T) {
	st := twoProjectState()
	s := ui.NewSidebar(ui.SidebarRows(st, nil))
	s.SelectByID("s3")
	st.Sessions = st.Sessions[:2] // s3, api-svc's only session, is archived

	s.SetRows(ui.SidebarRows(st, nil))

	if p, ok := s.SelectedHeader(); !ok || p != "api-svc" {
		t.Errorf("after the last session left, SelectedHeader() = %q, %v; want api-svc", p, ok)
	}
}

// With sessions left in the project, the cursor stays in the project rather
// than jumping to the first session anywhere.
func TestSidebar_SetRowsStaysInTheProjectWhenASessionRemains_issue158(t *testing.T) {
	st := twoProjectState()
	s := ui.NewSidebar(ui.SidebarRows(st, nil))
	s.SelectByID("s2")
	st.Sessions = []registry.Session{st.Sessions[0], st.Sessions[2]} // s2 archived

	s.SetRows(ui.SidebarRows(st, nil))

	if got, ok := s.Selected(); !ok || got.Session.ID != "s1" {
		t.Errorf("Selected() = %+v (ok=%v), want s1, omatty's remaining session", got, ok)
	}
}

func TestSidebar_SelectByProjectLandsOnItsFirstSessionOrItsHeader_issue158(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(threeProjectState(), nil))
	for _, tt := range []struct{ project, wantSession, wantHeader string }{
		{"b", "b1", ""}, {"none", "", "none"}, {"a", "a1", ""},
	} {
		if !s.SelectByProject(tt.project) {
			t.Errorf("SelectByProject(%q) = false, want true", tt.project)
		}
		got, _ := s.Selected()
		p, _ := s.SelectedHeader()
		if (tt.wantSession != "" && (got.Session == nil || got.Session.ID != tt.wantSession)) || p != tt.wantHeader {
			t.Errorf("SelectByProject(%q) landed on session %+v header %q; want %q / %q",
				tt.project, got.Session, p, tt.wantSession, tt.wantHeader)
		}
	}
	if s.SelectByProject("ghost") {
		t.Error("SelectByProject of an unregistered project returned true")
	}
}

// A fresh sidebar still opens on the first session anywhere, not on an empty
// project that happens to be registered first - there is something to type
// into, so start there.
func TestSidebar_NewSidebarPrefersAnySessionToAnEmptyHeader_issue158(t *testing.T) {
	st := registry.State{
		Projects: []registry.Project{{Name: "empty"}, {Name: "omatty"}},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: "main"}},
	}
	s := ui.NewSidebar(ui.SidebarRows(st, nil))
	if got, ok := s.Selected(); !ok || got.Session.ID != "s1" {
		t.Errorf("Selected() = %+v (ok=%v), want s1", got, ok)
	}
	if got := s.CursorProject(); got != "omatty" {
		t.Errorf("CursorProject() = %q, want omatty", got)
	}
}

// With no session anywhere the first project is the landing row, which is what
// keeps ctrl+o n working on a fresh install (the rows[0] fallback #158 notes).
func TestSidebar_HeadersOnlySelectsTheFirstHeader_issue158(t *testing.T) {
	st := registry.State{Projects: []registry.Project{{Name: "empty"}, {Name: "also"}}}
	s := ui.NewSidebar(ui.SidebarRows(st, nil))

	if p, ok := s.SelectedHeader(); !ok || p != "empty" {
		t.Errorf("SelectedHeader() = %q, %v; want empty", p, ok)
	}
	s.MoveDown()
	if p, _ := s.SelectedHeader(); p != "also" {
		t.Errorf("after MoveDown, SelectedHeader() = %q, want also", p)
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
