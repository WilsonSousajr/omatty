package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// emptyProjectState is the report in #158: one project with a session and a
// freshly discovered one with none.
func emptyProjectState() registry.State {
	return registry.State{
		Projects: []registry.Project{
			{Name: "omatty", Root: "/p/omatty"},
			{Name: "wstech", Root: "/p/wstech"},
		},
		Sessions: []registry.Session{{ID: "s1", Project: "omatty", Title: "main"}},
	}
}

// modelWithEmptyProject mirrors modelWithCreate over emptyProjectState, so a
// test can create the first session in wstech.
func modelWithEmptyProject(t *testing.T, c *recordCreate) *ui.Model {
	t.Helper()
	st := emptyProjectState()
	d := baseDeps(st, fakeTermsFor(st))
	d.Create = c.fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

// Regression, issue #158: ] never reached wstech and SelectedProject() could
// never return it.
func TestModel_anEmptyProjectIsReachableAndSelectable_issue158(t *testing.T) {
	m := modelWithEmptyProject(t, &recordCreate{})

	leader(m, key(']'))

	if got := m.SelectedProject(); got != "wstech" {
		t.Fatalf("SelectedProject() after ] = %q, want wstech", got)
	}
	if got := m.Selected(); got != "" {
		t.Errorf("Selected() = %q on a header, want \"\"", got)
	}
}

func TestModel_theFirstSessionInAnEmptyProjectLandsUnderItsHeader_issue158(t *testing.T) {
	c := &recordCreate{}
	m := modelWithEmptyProject(t, c)
	leader(m, key(']'))

	leader(m, key('n'))
	press(m, key('z'))
	press(m, special(tea.KeyEnter))

	if c.Project != "wstech" {
		t.Fatalf("create() got project %q, want wstech", c.Project)
	}
	if m.Selected() != "created" || m.SelectedProject() != "wstech" {
		t.Errorf("cursor on %q in %q after creating; want created in wstech", m.Selected(), m.SelectedProject())
	}
}

func TestModel_theHeaderShowsTheMarkerAndThePaneNamesTheProject_issue158(t *testing.T) {
	m := modelWithEmptyProject(t, &recordCreate{})
	leader(m, key(']'))

	got := m.View().Content

	if !strings.Contains(got, "» wstech") {
		t.Errorf("the selected header carries no marker:\n%s", got)
	}
	if !strings.Contains(got, "no sessions in wstech - press ctrl+o n") {
		t.Errorf("the pane does not say which project is empty:\n%s", got)
	}
	for i, line := range strings.Split(got, "\n") {
		if w := lipgloss.Width(line); w != 100 {
			t.Errorf("line %d is %d cells, want 100: %q", i, w, line)
		}
	}
}

// One row per key, as #95's table does: none of these may panic or open a
// surface on a header. x is not in the table: #159 gave it a job there, and
// removeproject_test.go covers it.
func TestModel_sessionKeysOnAHeaderDoNothing_issue158(t *testing.T) {
	for _, k := range []rune{'R', 'r', 'd', 'f'} {
		m := modelWithEmptyProject(t, &recordCreate{})
		leader(m, key(']'))

		leader(m, key(k))
		got := m.View().Content

		if m.Prompt().Active || strings.Contains(got, "archive session") || strings.Contains(got, "rename session") {
			t.Errorf("ctrl+o %c on a header opened a surface:\n%s", k, got)
		}
		if m.SelectedProject() != "wstech" {
			t.Errorf("ctrl+o %c moved the cursor to %q", k, m.SelectedProject())
		}
	}
}

// The no-sessions-anywhere case still creates in the first project.
func TestModel_promptCreatesInTheOnlyProjectWhenNoSessionExists_issue158(t *testing.T) {
	c := &recordCreate{}
	st := registry.State{Projects: []registry.Project{{Name: "solo", Root: "/p/solo"}}}
	d := baseDeps(st, fakeTermsFor(st))
	d.Create = c.fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	leader(m, key('n'))
	press(m, special(tea.KeyEnter))

	if c.Project != "solo" {
		t.Errorf("create() got project %q, want solo", c.Project)
	}
}

// A click on an empty header selects it; a click on a header with sessions
// still does nothing (#45).
func TestModel_clickingAnEmptyHeaderSelectsIt_issue158(t *testing.T) {
	m := modelWithEmptyProject(t, &recordCreate{})

	// rows: omatty, s1, wstech - the header is the third row of the list.
	m.Update(clickAt(3, sidebarRowY(2)))
	if m.SelectedProject() != "wstech" {
		t.Errorf("click on the empty header selected %q, want wstech", m.SelectedProject())
	}

	m.Update(clickAt(3, sidebarRowY(0)))
	if m.SelectedProject() != "wstech" || m.Selected() != "" {
		t.Errorf("click on omatty's header (which has sessions) moved the cursor to %q/%q",
			m.SelectedProject(), m.Selected())
	}
}

// Archive the only session of a project, then create a new one there. Fails
// before #158: the cursor left api-svc and it was unreachable.
func TestModel_archivingTheLastSessionThenCreatingThereWorks_issue158(t *testing.T) {
	r, c := &recordArchive{State: worktreeState()}, &recordCreate{}
	terms, _ := fakeTerms(t)
	d := baseDeps(worktreeState(), terms)
	d.Archive, d.TailStop, d.RemoveWorktree = r.archive, r.stopTail, r.removeWorktree
	d.Create = c.fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	openArchive(t, m, "s3") // api-svc's only session
	pressAndSettle(m, key('y'))

	if got := m.SelectedProject(); got != "api-svc" || m.Selected() != "" {
		t.Fatalf("after archiving s3 the cursor is on %q in %q; want api-svc's header", m.Selected(), got)
	}
	leader(m, key('n'))
	press(m, special(tea.KeyEnter))
	if c.Project != "api-svc" {
		t.Errorf("create() got project %q, want api-svc", c.Project)
	}
}
