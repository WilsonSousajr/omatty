package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/ui"
)

// hairlines is how many column hairlines the header row draws: two with the
// session pane between the sidebar and the column, one when the column is
// zoomed over the pane.
func hairlines(m *ui.Model) int { return strings.Count(stripSGR(frameLines(m)[0]), "│") }

// ctrl+o z widens the column over the session pane: the header loses the pane's
// segment, the column's content runs from the sidebar to the edge, and its rule
// says it is zoomed (#427).
func TestZoom_LeaderZWidensTheColumnOverThePane_issue427(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	if hairlines(m) != 2 {
		t.Fatalf("before zooming the header should hold three segments: %q", frameLines(m)[0])
	}

	leader(m, key('z'))

	if hairlines(m) != 1 {
		t.Errorf("zoomed, the header still draws the session pane: %q", stripSGR(frameLines(m)[0]))
	}
	if rule := stripSGR(frameLines(m)[1]); !strings.Contains(rule, "─ diff · zoomed ─") {
		t.Errorf("the zoomed rule does not say so: %q", rule)
	}
	if body := m.View().Content; strings.Contains(body, "session one") {
		t.Errorf("the session pane is still drawn under the zoom:\n%s", body)
	}
	for i, line := range frameLines(m) {
		if w := lipgloss.Width(line); w != 100 {
			t.Errorf("zoomed frame line %d is %d cells, want the window's 100", i, w)
		}
	}
}

// The session keeps running at its own size underneath: resizing the PTY would
// make claude reflow for a view it is not even drawn in.
func TestZoom_TheSessionKeepsItsSize_issue427(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))
	w, h := fakes["s1"].Width, fakes["s1"].Height

	leader(m, key('z'))

	if fakes["s1"].Width != w || fakes["s1"].Height != h {
		t.Errorf("zooming resized the session to %dx%d, want it left at %dx%d", fakes["s1"].Width, fakes["s1"].Height, w, h)
	}
}

// ctrl+o z again, esc, and closing the column all bring the split back; a
// column closed while zoomed opens unzoomed.
func TestZoom_EveryWayOutRestoresTheSplit_issue427(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	leader(m, key('z'))
	leader(m, key('z'))
	if hairlines(m) != 2 {
		t.Errorf("a second ctrl+o z did not restore the split")
	}

	leader(m, key('z'))
	press(m, special(tea.KeyEscape))
	if hairlines(m) != 2 {
		t.Errorf("esc handed the keys to a session the zoom still hides")
	}

	leader(m, key('d')) // close the column it zoomed
	leader(m, key('d')) // and open it again, with the keys
	if hairlines(m) != 2 || strings.Contains(stripSGR(frameLines(m)[1]), "zoomed") {
		t.Errorf("a column closed while zoomed reopened zoomed")
	}
}

// With no column open there is nothing to zoom, and the footer says what would
// open one rather than doing nothing silently.
func TestZoom_WithNoColumnSaysWhatToOpen_issue427(t *testing.T) {
	m, _, _ := modelWithDiff(t)

	leader(m, key('z'))

	if hairlines(m) != 1 || !strings.Contains(m.View().Content, "nothing to zoom") {
		t.Errorf("ctrl+o z with no column open did not say so:\n%s", m.View().Content)
	}
}

// A click lands on the row drawn under it in the zoomed column too.
func TestZoom_AClickSelectsTheRowUnderIt_issue427(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	leader(m, key('z'))

	m.Update(clickAt(ui.SidebarWidth+5, reviewRowY(5)))

	if m.ReviewCursor() != 5 {
		t.Errorf("a click on zoomed row 5 left the cursor at %d", m.ReviewCursor())
	}
}
