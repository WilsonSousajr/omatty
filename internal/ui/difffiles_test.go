package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/ui"
)

// zoomedDiff is the sample diff zoomed at 200 columns, where the file list
// has room beside it.
func zoomedDiff(t *testing.T) *ui.Model {
	t.Helper()
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	leader(m, key('d'))
	leader(m, key('z'))
	return m
}

// Zoomed and wide, the diff has its files listed beside it with their counts,
// diffnav's file tree (#437).
func TestDiff_ZoomedListsItsFilesBesideIt_issue437(t *testing.T) {
	m := zoomedDiff(t)

	// Each name twice: once in the list, once on the diff's own header.
	body := stripSGR(m.View().Content)
	for _, want := range []string{"model.go", "new.txt"} {
		if n := strings.Count(body, want); n < 2 {
			t.Errorf("%q is drawn %d time(s), want the list's row as well as the diff's header:\n%s", want, n, body)
		}
	}
}

// The list follows the diff: ] moves the diff to new.txt, and the list marks
// new.txt as the file the cursor is in.
func TestDiff_TheListFollowsTheDiff_issue437(t *testing.T) {
	m := zoomedDiff(t)

	press(m, key(']'))

	if !strings.Contains(m.View().Content, ui.Bold("new.txt")) {
		t.Errorf("after ] the list does not mark new.txt:\n%s", stripSGR(m.View().Content))
	}
}

// And the diff follows the list: a click on a file there moves the diff to
// that file's header.
func TestDiff_AClickOnTheListMovesTheDiff_issue437(t *testing.T) {
	m := zoomedDiff(t)

	m.Update(clickAt(ui.SidebarWidth+3, reviewRowY(1))) // new.txt, the list's second row

	if row := cursorRow(m); !strings.Contains(row, "new.txt") {
		t.Errorf("a click on new.txt in the list left the diff on %q", row)
	}
}

// Unzoomed, the diff is drawn as it always was.
func TestDiff_NoFileListUnzoomed_issue437(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	leader(m, key('d'))

	if strings.Contains(m.View().Content, ui.Bold("model.go")) {
		t.Errorf("an unzoomed diff drew a file list")
	}
}
