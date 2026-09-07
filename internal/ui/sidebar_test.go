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

// A project with no sessions under it has header rows and nothing to land on.
// Written without a lap bound, a modulo walk spins here forever.
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

// Twenty-one rows, ten fit: the window must hold the cursor, whatever moved it.
func TestSidebar_WindowFollowsTheCursor_issue129(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(sevenProjectState(), nil))
	s.SelectByID("p5-s1") // row 17

	win := s.Window(10)

	if len(win) != 10 {
		t.Fatalf("Window(10) = %d rows, want 10", len(win))
	}
	found := false
	for _, r := range win {
		found = found || (r.Session != nil && r.Session.ID == "p5-s1")
	}
	if !found {
		t.Errorf("the selected row is not in the window: %+v", win)
	}
	if s.Offset() != 8 {
		t.Errorf("Offset() = %d, want 8 (cursor 17 on the last of 10 rows)", s.Offset())
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

	if s.Offset() != 6 || len(win) != 5 {
		t.Errorf("Window(5) after shrinking: offset %d len %d, want 6 and 5", s.Offset(), len(win))
	}
}

func TestSidebar_WindowWithNoSessionsIsSafe_issue129(t *testing.T) {
	s := ui.NewSidebar(nil)
	if got := s.Window(5); len(got) != 0 || s.Offset() != 0 {
		t.Errorf("Window(5) on nothing = %d rows offset %d", len(got), s.Offset())
	}
}
