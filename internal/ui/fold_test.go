package ui_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// recordFold is the named fake for Deps.Fold: it records every persisted
// fold, and fails every one when Err is set.
type recordFold struct {
	Calls []string
	Err   error
}

func (r *recordFold) fold(project string, collapsed bool) error {
	verb := "unfold "
	if collapsed {
		verb = "fold "
	}
	r.Calls = append(r.Calls, verb+project)
	return r.Err
}

// modelWithFold opens over st with the fold and create fakes wired in.
func modelWithFold(t *testing.T, st registry.State, f *recordFold, c *recordCreate) *ui.Model {
	t.Helper()
	d := baseDeps(st, fakeTermsFor(st))
	d.Fold = f.fold
	d.Create = c.fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

func tab() tea.KeyPressMsg { return special(tea.KeyTab) }

func TestModel_leaderTabFoldsTheProjectAndPersistsIt_issue505(t *testing.T) {
	f := &recordFold{}
	m := modelWithFold(t, twoProjectState(), f, &recordCreate{})

	leader(m, tab())

	if len(f.Calls) != 1 || f.Calls[0] != "fold omatty" {
		t.Fatalf("persisted %v, want [fold omatty]", f.Calls)
	}
	if m.SidebarRows() != 3 || m.SelectedProject() != "omatty" || m.Selected() != "" {
		t.Errorf("after folding: %d rows, cursor in %q on %q; want 3 rows and omatty's header",
			m.SidebarRows(), m.SelectedProject(), m.Selected())
	}
	got := stripSGR(m.View().Content)
	if !strings.Contains(got, "▸ omatty") || strings.Contains(got, "parser-fix") {
		t.Errorf("the folded project still draws its sessions, or no ▸:\n%s", got)
	}
}

func TestModel_leaderTabOnAFoldedHeaderUnfoldsOntoTheFirstSession_issue505(t *testing.T) {
	f := &recordFold{}
	m := modelWithFold(t, twoProjectState(), f, &recordCreate{})
	leader(m, tab())

	leader(m, tab())

	if len(f.Calls) != 2 || f.Calls[1] != "unfold omatty" {
		t.Fatalf("persisted %v, want fold then unfold omatty", f.Calls)
	}
	if m.SidebarRows() != 5 || m.Selected() != "s1" {
		t.Errorf("after unfolding: %d rows on %q; want 5 rows and s1", m.SidebarRows(), m.Selected())
	}
	if got := stripSGR(m.View().Content); !strings.Contains(got, "▾ omatty") {
		t.Errorf("an open project with sessions draws no ▾:\n%s", got)
	}
}

// The registry write comes first: a fold that could not be saved must not
// appear to have happened.
func TestModel_aFoldThatCannotBeSavedChangesNothing_issue505(t *testing.T) {
	f := &recordFold{Err: errors.New("state.json is read-only")}
	m := modelWithFold(t, twoProjectState(), f, &recordCreate{})

	leader(m, tab())

	if m.SidebarRows() != 5 || m.Selected() != "s1" {
		t.Errorf("a failed fold left %d rows on %q; want 5 and s1", m.SidebarRows(), m.Selected())
	}
	if got := m.View().Content; !strings.Contains(got, "read-only") {
		t.Errorf("the save error is not shown:\n%s", got)
	}
}

func TestModel_leaderTabOnAnEmptyProjectDoesNothing_issue505(t *testing.T) {
	f := &recordFold{}
	m := modelWithFold(t, emptyProjectState(), f, &recordCreate{})
	leader(m, key(']'))

	leader(m, tab())

	if len(f.Calls) != 0 || m.SelectedProject() != "wstech" {
		t.Errorf("tab on an empty project persisted %v and moved to %q", f.Calls, m.SelectedProject())
	}
}

// A folded header is a header the cursor rests on, but its project is not
// empty: ctrl+o x must not offer to forget it.
func TestModel_xOnAFoldedHeaderDoesNotOfferRemoval_issue505(t *testing.T) {
	st := foldedState()
	m := modelWithFold(t, st, &recordFold{}, &recordCreate{})
	leader(m, key('['))

	leader(m, key('x'))

	if got := m.View().Content; strings.Contains(got, "remove project") {
		t.Errorf("x on a folded header offered to remove the project:\n%s", got)
	}
}

func TestModel_theFoldedHeaderShowsItsCountAndThePaneSaysHowToUnfold_issue505(t *testing.T) {
	m := modelWithFold(t, foldedState(), &recordFold{}, &recordCreate{})
	leader(m, key('['))

	got := stripSGR(m.View().Content)

	if !strings.Contains(got, "▎ ▸ omatty") || !strings.Contains(got, " 2 ○") {
		t.Errorf("the folded header lacks the rail, ▸, or its count and glyph:\n%s", got)
	}
	if !strings.Contains(got, "omatty is folded") || strings.Contains(got, "no sessions in") {
		t.Errorf("the pane does not say the project is folded:\n%s", got)
	}
}

func TestModel_theSwitcherReachesAFoldedSessionAndUnfoldsIt_issue505(t *testing.T) {
	f := &recordFold{}
	m := modelWithFold(t, foldedState(), f, &recordCreate{})

	openSwitcher(m, "parser")
	press(m, special(tea.KeyEnter))

	if m.Selected() != "s2" {
		t.Fatalf("switcher landed on %q, want s2 behind the fold", m.Selected())
	}
	if len(f.Calls) != 1 || f.Calls[0] != "unfold omatty" || m.SidebarRows() != 5 {
		t.Errorf("persisted %v with %d rows; want omatty unfolded", f.Calls, m.SidebarRows())
	}
}

func TestModel_creatingInAFoldedProjectUnfoldsIt_issue505(t *testing.T) {
	f, c := &recordFold{}, &recordCreate{}
	m := modelWithFold(t, foldedState(), f, c)
	leader(m, key('['))

	leader(m, key('n'))
	press(m, key('z'))
	press(m, special(tea.KeyEnter))

	if c.Project != "omatty" || m.Selected() != "created" {
		t.Fatalf("created in %q, cursor on %q; want created in omatty", c.Project, m.Selected())
	}
	if len(f.Calls) != 1 || f.Calls[0] != "unfold omatty" {
		t.Errorf("persisted %v, want [unfold omatty]", f.Calls)
	}
}

func TestModel_aClickOnAProjectHeaderFoldsAndUnfoldsIt_issue505(t *testing.T) {
	f := &recordFold{}
	m := modelWithFold(t, twoProjectState(), f, &recordCreate{})

	m.Update(clickAt(4, sidebarLineY(0)))
	if len(f.Calls) != 1 || f.Calls[0] != "fold omatty" || m.SidebarRows() != 3 {
		t.Fatalf("header click persisted %v with %d rows; want omatty folded", f.Calls, m.SidebarRows())
	}
	m.Update(clickAt(4, sidebarLineY(0)))
	if len(f.Calls) != 2 || f.Calls[1] != "unfold omatty" || m.SidebarRows() != 5 {
		t.Errorf("second click persisted %v with %d rows; want omatty unfolded", f.Calls, m.SidebarRows())
	}
}

// An empty project's header has nothing to fold; a click there still selects
// it, as #158 made it.
func TestModel_aClickOnAnEmptyHeaderStillSelectsIt_issue505(t *testing.T) {
	f := &recordFold{}
	m := modelWithFold(t, emptyProjectState(), f, &recordCreate{})

	m.Update(clickAt(4, sidebarLineY(1+ui.CardLines())))

	if len(f.Calls) != 0 || m.SelectedProject() != "wstech" {
		t.Errorf("click on wstech persisted %v and selected %q; want no fold and wstech", f.Calls, m.SelectedProject())
	}
}
