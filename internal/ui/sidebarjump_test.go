package ui_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

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
