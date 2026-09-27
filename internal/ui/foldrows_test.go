package ui_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// foldedState is twoProjectState with omatty folded: two sessions behind one
// header, then api-svc open beneath it.
func foldedState() registry.State {
	st := twoProjectState()
	st.Projects[0].Collapsed = true
	return st
}

func TestSidebarRows_aFoldedProjectDrawsOnlyItsHeader_issue505(t *testing.T) {
	rows := ui.SidebarRows(foldedState(), map[string]watcher.Status{"s2": watcher.StatusWaiting})

	if len(rows) != 3 {
		t.Fatalf("got %d rows, want header omatty, header api-svc, s3: %+v", len(rows), rows)
	}
	h := rows[0]
	if h.Project != "omatty" || h.Session != nil {
		t.Fatalf("row 0 = %+v, want omatty's header", h)
	}
	if len(h.Folded) != 2 || h.Folded[0].ID != "s1" || h.Folded[1].ID != "s2" {
		t.Errorf("Folded = %+v, want s1 and s2 in order", h.Folded)
	}
	if h.Status != watcher.StatusWaiting {
		t.Errorf("folded header status = %q, want the loudest hidden one, waiting", h.Status)
	}
	if len(rows[1].Folded) != 0 {
		t.Errorf("an unfolded header carries Folded = %+v, want none", rows[1].Folded)
	}
}

// The header's status is the one a person must see first. waiting outranks
// error outranks done outranks work outranks rest.
func TestFoldStatus_picksTheLoudest_issue505(t *testing.T) {
	cases := []struct {
		in   []watcher.Status
		want watcher.Status
	}{
		{[]watcher.Status{watcher.StatusIdle, watcher.StatusThinking}, watcher.StatusThinking},
		{[]watcher.Status{watcher.StatusTool, watcher.StatusDone}, watcher.StatusDone},
		{[]watcher.Status{watcher.StatusDone, watcher.StatusError}, watcher.StatusError},
		{[]watcher.Status{watcher.StatusError, watcher.StatusWaiting, watcher.StatusIdle}, watcher.StatusWaiting},
		{[]watcher.Status{watcher.StatusExited, watcher.StatusIdle}, watcher.StatusIdle},
	}
	for _, c := range cases {
		st := foldedState()
		status := map[string]watcher.Status{"s1": c.in[0], "s2": c.in[1]}
		if len(c.in) > 2 {
			st.Sessions = append(st.Sessions, registry.Session{ID: "s4", Project: "omatty"})
			status["s4"] = c.in[2]
		}
		if got := ui.SidebarRows(st, status)[0].Status; got != c.want {
			t.Errorf("loudest of %v = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSidebar_cursorLandsOnAFoldedHeader_issue505(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(foldedState(), nil))

	if got, ok := s.Selected(); !ok || got.Session.ID != "s3" {
		t.Fatalf("Selected() = %+v (ok=%v), want s3, the first visible session", got, ok)
	}
	s.MoveDown()
	if p, ok := s.SelectedFold(); !ok || p != "omatty" {
		t.Errorf("after wrapping, SelectedFold() = %q, %v; want omatty", p, ok)
	}
	if _, ok := s.SelectedHeader(); ok {
		t.Error("SelectedHeader() is true on a folded header; it must mean an empty project only")
	}
	if s.CursorProject() != "omatty" {
		t.Errorf("CursorProject() = %q, want omatty", s.CursorProject())
	}
}

func TestSidebar_bracketsLandOnAFoldedHeader_issue505(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(foldedState(), nil))
	s.PrevProject()
	if p, ok := s.SelectedFold(); !ok || p != "omatty" {
		t.Fatalf("[ from s3 = %q, %v; want the folded header", p, ok)
	}
	s.NextProject()
	if got, ok := s.Selected(); !ok || got.Session.ID != "s3" {
		t.Errorf("] from the folded header = %+v, want s3", got)
	}
	s.PrevProject()
	if p, ok := s.SelectedFold(); !ok || p != "omatty" {
		t.Errorf("[ back = %q, %v; want the folded header", p, ok)
	}
}

func TestSidebar_SelectedFoldIsFalseOnAnEmptyHeaderAndASession_issue505(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(emptyProjectState(), nil))
	if _, ok := s.SelectedFold(); ok {
		t.Error("SelectedFold() true on a session row")
	}
	s.NextProject()
	if _, ok := s.SelectedFold(); ok {
		t.Error("SelectedFold() true on an empty project's header")
	}
}

// Folding away the selected session must leave the cursor in its project,
// on the header, rather than wherever reset lands.
func TestSidebar_SetRowsFoldingTheSelectionKeepsTheProject_issue505(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(twoProjectState(), nil))
	s.MoveDown() // s2
	s.SetRows(ui.SidebarRows(foldedState(), nil))

	if p, ok := s.SelectedFold(); !ok || p != "omatty" {
		t.Errorf("SelectedFold() = %q, %v; want omatty's header", p, ok)
	}
}
