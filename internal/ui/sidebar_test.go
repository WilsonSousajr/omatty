package ui_test

import (
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

func TestSidebar_CursorSkipsProjectHeaders(t *testing.T) {
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
	if got, _ := s.Selected(); got.Session.ID != "s3" {
		t.Errorf("MoveDown at the end moved to %q, want to stay on s3", got.Session.ID)
	}
	s.MoveUp()
	if got, _ := s.Selected(); got.Session.ID != "s2" {
		t.Errorf("after MoveUp, Selected() = %q, want s2", got.Session.ID)
	}
	s.MoveUp()
	s.MoveUp()
	if got, _ := s.Selected(); got.Session.ID != "s1" {
		t.Errorf("MoveUp at the start moved to %q, want to stay on s1", got.Session.ID)
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
