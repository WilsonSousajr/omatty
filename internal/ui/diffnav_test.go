package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/ui"
)

// openWideDiff is the sample diff - model.go then new.txt, one hunk each - open
// wide enough for whole headers, the cursor on the first file's header.
func openWideDiff(t *testing.T) *ui.Model {
	t.Helper()
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	leader(m, key('d'))
	return m
}

// ] and [ walk the files, n and N the hunks: two levels, as diffview and
// diffnav have them (#436).
func TestDiff_BracketsWalkFilesAndNWalksHunks_issue436(t *testing.T) {
	m := openWideDiff(t)

	press(m, key(']'))
	if row := cursorRow(m); !strings.Contains(row, "new.txt") {
		t.Errorf("] landed on %q, want new.txt's header", row)
	}
	press(m, key('['))
	if row := cursorRow(m); !strings.Contains(row, "model.go") {
		t.Errorf("[ landed on %q, want model.go's header", row)
	}
	press(m, key('n'))
	if row := cursorRow(m); !strings.Contains(row, "@@ -10,4") {
		t.Errorf("n landed on %q, want model.go's hunk", row)
	}
	press(m, key('n'))
	if row := cursorRow(m); !strings.Contains(row, "@@ -0,0") {
		t.Errorf("n again landed on %q, want new.txt's hunk", row)
	}
	press(m, key('N'))
	if row := cursorRow(m); !strings.Contains(row, "@@ -10,4") {
		t.Errorf("N landed on %q, want model.go's hunk again", row)
	}
}

// Each file header carries its counts and its place in the diff.
func TestDiff_AFileHeaderSaysWhereItIs_issue436(t *testing.T) {
	m := openWideDiff(t)

	if row := plainLineWith(t, m.View().Content, "model.go +2 -1"); !strings.Contains(row, "1/2") {
		t.Errorf("model.go's header does not say 1/2: %q", row)
	}
	if row := plainLineWith(t, m.View().Content, "new.txt +2 -0"); !strings.Contains(row, "2/2") {
		t.Errorf("new.txt's header does not say 2/2: %q", row)
	}
}

// enter on a header folds the file to that one row, and enter again opens it.
func TestDiff_EnterFoldsAFileToItsHeader_issue436(t *testing.T) {
	m := openWideDiff(t)

	press(m, special(tea.KeyEnter))
	body := stripSGR(m.View().Content)
	if strings.Contains(body, "b := 3") || !strings.Contains(body, "model.go") || !strings.Contains(body, "fresh") {
		t.Errorf("enter did not fold model.go alone to its header:\n%s", body)
	}
	if row := cursorRow(m); !strings.Contains(row, "folded") {
		t.Errorf("a folded header does not say so: %q", row)
	}
	press(m, special(tea.KeyEnter))
	if !strings.Contains(stripSGR(m.View().Content), "b := 3") {
		t.Errorf("enter again did not unfold model.go")
	}
}
