package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// typeInto presses each rune of s as a plain key.
func typeInto(m *ui.Model, s string) {
	for _, r := range s {
		press(m, key(r))
	}
}

// / opens a filter line; typing narrows the listing as it goes and the title
// says what the listing is narrowed to (#198).
func TestModel_SlashFiltersTheTreeLive_issue198(t *testing.T) {
	m, _, _, _ := modelWithTree(t)
	leader(m, key('f'))

	press(m, key('/'))
	typeInto(m, "mod")

	view := m.View().Content
	lineWith(t, view, "model.go")
	lineWith(t, view, "go.mod")
	if strings.Contains(view, "render.go") || strings.Contains(view, "new.txt") {
		t.Errorf("rows that do not match /mod are still listed:\n%s", view)
	}
	lineWith(t, view, "files · ")
	lineWith(t, view, "/mod")
	lineWith(t, view, "filter: mod_")
}

// enter keeps the filter and hands the keys back to the list, so j moves the
// cursor instead of typing; a second / edits the same query.
func TestModel_EnterKeepsTheFilterAndHandsKeysBack_issue198(t *testing.T) {
	m, _, _, reader := modelWithTree(t)
	leader(m, key('f'))
	press(m, key('/'))
	typeInto(m, "go.mod")
	press(m, special(tea.KeyEnter))

	if strings.Contains(m.View().Content, "filter:") {
		t.Fatal("the filter line is still open after enter")
	}
	lineWith(t, m.View().Content, "/go.mod")
	press(m, key('j')) // to go.mod, the only file left
	pressAndSettle(m, special(tea.KeyEnter))

	if m.ReviewView() != ui.ViewPreview || len(reader.Read) != 1 || reader.Read[0] != "go.mod" {
		t.Errorf("view=%v read=%v; want j to have moved onto go.mod and enter to preview it",
			m.ReviewView(), reader.Read)
	}
}

// esc on the filter line clears it and gives the full listing back, folds
// and all; a match inside a folded directory was shown while filtering.
func TestModel_EscClearsTheFilterAndRestoresTheFolds_issue198(t *testing.T) {
	m, _, _, _ := modelWithTree(t)
	leader(m, key('f'))
	press(m, special(tea.KeyEnter)) // fold internal/
	if strings.Contains(m.View().Content, "model.go") {
		t.Fatal("internal/ did not fold")
	}

	press(m, key('/'))
	typeInto(m, "model")
	lineWith(t, m.View().Content, "model.go")

	press(m, special(tea.KeyEscape))

	view := m.View().Content
	if strings.Contains(view, "model.go") || strings.Contains(view, "/model") {
		t.Errorf("esc did not clear the filter and restore the fold:\n%s", view)
	}
	lineWith(t, view, "▸ internal/")
	if !m.ReviewFocused() {
		t.Error("esc on the filter line dropped the column's focus instead of just the filter")
	}
}

// With a kept filter in force, esc in the list clears it before it leaves
// the column: the operator sees the narrowing lifted, not the column gone.
func TestModel_EscInTheListClearsAKeptFilterFirst_issue198(t *testing.T) {
	m, _, _, _ := modelWithTree(t)
	leader(m, key('f'))
	press(m, key('/'))
	typeInto(m, "mod")
	press(m, special(tea.KeyEnter))

	press(m, special(tea.KeyEscape))
	if !m.ReviewFocused() || strings.Contains(m.View().Content, "/mod") {
		t.Fatal("the first esc should clear the filter and keep focus")
	}
	press(m, special(tea.KeyEscape))
	if m.ReviewFocused() {
		t.Error("the second esc should leave the column")
	}
}

func TestModel_ANothingMatchesFilterSaysSo_issue198(t *testing.T) {
	m, _, _, _ := modelWithTree(t)
	leader(m, key('f'))
	press(m, key('/'))
	typeInto(m, "zzz")
	lineWith(t, m.View().Content, "no files match")
}

// The keys are where the operator looks for them (#103).
func TestModel_TheFilterKeyIsDocumented_issue198(t *testing.T) {
	if got := ui.Footers(ui.DefaultLeader)["treeFooter"]; !strings.Contains(got, "/ filter") {
		t.Errorf("treeFooter does not name the filter key: %q", got)
	}
	m, _, _, _ := modelWithTree(t)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	leader(m, key('?'))
	lineWith(t, m.View().Content, "filter the tree")
}
