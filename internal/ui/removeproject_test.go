package ui_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// recordRemoveProject is the named fake for Deps.RemoveProject.
type recordRemoveProject struct {
	Removed []string
	Err     error
}

func (r *recordRemoveProject) remove(name string) (registry.Project, error) {
	r.Removed = append(r.Removed, name)
	return registry.Project{Name: name}, r.Err
}

// modelWithRemoveProject opens on wstech's header, the one row x can forget.
func modelWithRemoveProject(t *testing.T, r *recordRemoveProject) *ui.Model {
	t.Helper()
	st := emptyProjectState()
	d := baseDeps(st, fakeTermsFor(st))
	d.RemoveProject = r.remove
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key(']'))
	return m
}

func TestModel_xOnAnEmptyHeaderAsksThenRemovesTheProject_issue159(t *testing.T) {
	r := &recordRemoveProject{}
	m := modelWithRemoveProject(t, r)

	leader(m, key('x'))
	if got := m.View().Content; !strings.Contains(got, `remove project "wstech"`) || !strings.Contains(got, "untouched") {
		t.Fatalf("x on a header did not ask about the project:\n%s", got)
	}
	pressAndSettle(m, key('y'))

	if len(r.Removed) != 1 || r.Removed[0] != "wstech" {
		t.Errorf("removed %v, want [wstech]", r.Removed)
	}
	if m.SidebarRows() != 2 || m.SelectedProject() != "omatty" {
		t.Errorf("sidebar has %d rows with %q selected; want omatty's two rows", m.SidebarRows(), m.SelectedProject())
	}
	if strings.Contains(m.View().Content, "wstech") {
		t.Error("wstech is still drawn after removal")
	}
}

func TestModel_removeProjectFailureKeepsTheRowAndShowsTheError_issue159(t *testing.T) {
	r := &recordRemoveProject{Err: errors.New("state.json is read-only")}
	m := modelWithRemoveProject(t, r)

	leader(m, key('x'))
	pressAndSettle(m, key('y'))

	got := m.View().Content
	if !strings.Contains(got, "wstech") || !strings.Contains(got, "read-only") {
		t.Errorf("after a failed removal the row or the error is missing:\n%s", got)
	}
}

func TestModel_removeProjectEscCancels_issue159(t *testing.T) {
	r := &recordRemoveProject{}
	m := modelWithRemoveProject(t, r)

	leader(m, key('x'))
	press(m, special(tea.KeyEscape))

	if len(r.Removed) != 0 || m.SidebarRows() != 3 {
		t.Errorf("esc removed %v / left %d rows; want nothing removed and 3 rows", r.Removed, m.SidebarRows())
	}
}

// The help must say x now does two things (#103's rule: every key in one list).
func TestModel_helpSaysXRemovesAnEmptyProject_issue159(t *testing.T) {
	m := modelWithRemoveProject(t, &recordRemoveProject{})
	leader(m, key('?'))
	if got := m.View().Content; !strings.Contains(got, "empty project") {
		t.Errorf("help does not mention removing an empty project:\n%s", got)
	}
}
