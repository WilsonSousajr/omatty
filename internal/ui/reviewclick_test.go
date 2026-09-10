package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// The review column's geometry at the 100x30 fixture window, pinned as
// literals the way sidebarLineY is (#45): the column is the last 28 cells,
// its hairline at 72, its content 73..99; the rule is window row 1 and the
// first content row is 2.
const (
	reviewHairlineX = 72
	reviewCloseX    = 99 // the last cell of the window: the × on the rule
	reviewRuleY     = 1
	reviewTopY      = 2
)

func reviewRowY(row int) int { return reviewTopY + row }

// A left click on the × closes the column from either focus state, keeping
// its content the way the leader does (#124, #168).
func TestUpdate_AClickOnTheCloseGlyphClosesTheColumn_issue168(t *testing.T) {
	for _, focused := range []bool{true, false} {
		m, fakes, _ := modelWithDiff(t)
		leader(m, key('d'))
		if !focused {
			press(m, special(tea.KeyEscape))
		}

		_, cmd := m.Update(clickAt(reviewCloseX, reviewRuleY))
		settle(m, cmd)

		if m.ReviewOpen() {
			t.Errorf("focused=%v: the column is still open after a click on ×", focused)
		}
		w, h := ui.PTYSize(100, 30, false)
		if f := fakes["s1"]; f.Width != w || f.Height != h {
			t.Errorf("focused=%v: terminal is %dx%d after the close, want %dx%d back", focused, f.Width, f.Height, w, h)
		}
		leader(m, key('d'))
		if !strings.Contains(m.View().Content, "internal/ui/model.go") {
			t.Errorf("focused=%v: the reopened column lost its diff", focused)
		}
	}
}

// A click on a diff row moves the review cursor there and gives the column
// the keys, the way j/k need them; the sidebar's cursor is left alone.
func TestUpdate_AClickOnAReviewRowMovesTheCursor_issue168(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	press(m, special(tea.KeyEscape))

	m.Update(clickAt(reviewHairlineX+5, reviewRowY(5)))

	if m.ReviewCursor() != 5 || !m.ReviewFocused() {
		t.Errorf("cursor=%d focused=%v after a click on row 5, want 5 and focused", m.ReviewCursor(), m.ReviewFocused())
	}
	if m.Selected() != "s1" {
		t.Errorf("Selected() = %q, want the sidebar untouched", m.Selected())
	}
}

// The row hit under a scrolled column is the row drawn there, not the row
// at that index: the offset case sidebarRowAt exists for (#129).
func TestUpdate_TheRowHitUnderAScrolledColumnIsTheRowDrawnThere_issue168(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 10}) // 7 content rows for 12 entries
	leader(m, key('d'))
	down(m, 9) // cursor 9, so the window starts at entry 3

	m.Update(clickAt(reviewHairlineX+5, reviewRowY(0)))

	if m.ReviewCursor() != 3 {
		t.Errorf("cursor = %d after a click on the first drawn row, want 3", m.ReviewCursor())
	}
}

// A click past the last entry moves nothing.
func TestUpdate_AClickBelowTheLastReviewRowIsIgnored_issue168(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 2)

	m.Update(clickAt(reviewHairlineX+5, reviewRowY(20)))

	if m.ReviewCursor() != 2 {
		t.Errorf("cursor = %d after a click on an empty row, want 2 unchanged", m.ReviewCursor())
	}
}

// The tree is a list with a cursor too: a click on a row lands there, and
// enter then acts on it.
func TestUpdate_AClickOnATreeRowMovesTheTreeCursor_issue168(t *testing.T) {
	m, _, _, reader := modelWithTree(t)
	leader(m, key('f'))

	m.Update(clickAt(reviewHairlineX+5, reviewRowY(2))) // internal/, ui/, model.go
	pressAndSettle(m, special(tea.KeyEnter))

	if m.ReviewView() != ui.ViewPreview || len(reader.Read) != 1 || reader.Read[0] != "internal/ui/model.go" {
		t.Errorf("view=%v read=%v after a click on row 2 and enter; want a preview of model.go", m.ReviewView(), reader.Read)
	}
}

// A click over the column while a modal is open is dropped, as it is for the
// sidebar: the row under the pointer is not on screen.
func TestUpdate_AClickOverTheColumnBehindAModalIsDropped_issue168(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	leader(m, key('?'))

	m.Update(clickAt(reviewHairlineX+5, reviewRowY(5)))
	m.Update(clickAt(reviewCloseX, reviewRuleY))

	if m.ReviewCursor() != 0 || !m.ReviewOpen() {
		t.Errorf("cursor=%d open=%v after clicks behind a modal, want 0 and still open", m.ReviewCursor(), m.ReviewOpen())
	}
}

// The column's rule says whose it is, so omatty's diff and the diff claude
// draws inside its own pane are told apart at a glance; the pane's rule
// carries neither the label nor a close glyph, since that pane never closes.
func TestFrame_TheReviewRuleIsLabelledAndClosableAndThePanesIsNot_issue168(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))

	rows := strings.Split(m.View().Content, "\n")
	rule := []rune(stripSGR(rows[reviewRuleY])) // runes: a dash is three bytes

	if len(rule) != 100 {
		t.Fatalf("rule row is %d cells, want 100: %q", len(rule), string(rule))
	}
	if rule[reviewCloseX] != '×' {
		t.Errorf("the last cell is %q, want the close glyph: %q", rule[reviewCloseX], string(rule))
	}
	if column := string(rule[reviewHairlineX:]); !strings.Contains(column, "review") {
		t.Errorf("the column's rule is not labelled: %q", column)
	}
	if pane := string(rule[:reviewHairlineX]); strings.Contains(pane, "×") || strings.Contains(pane, "review") {
		t.Errorf("the pane's rule gained the column's affordances: %q", pane)
	}
}

// The rule with a closable segment is still exactly its segments wide, at
// every width including one too narrow for the label, and the × is the
// closable segment's last cell and nowhere else (#35, #174, #168).
func TestRuleRow_AClosableSegmentKeepsTheRowExactlyItsSegments_issue168(t *testing.T) {
	for _, widths := range [][]int{{27, 72}, {27, 44, 27}, {27, 20, 23}, {27, 100, 5}, {27, 172, 51}} {
		sum := len(widths) - 1
		for _, w := range widths {
			sum += w
		}
		last := len(widths) - 1
		got := []rune(stripSGR(ui.RuleRowClosable(widths, last)))
		if len(got) != sum || strings.Count(string(got), "×") != 1 || got[sum-1] != '×' {
			t.Errorf("RuleRowClosable(%v) = %q: want %d cells with one × in the last", widths, string(got), sum)
		}
		if plain := ui.RuleRowClosable(widths, -1); strings.Contains(plain, "×") || strings.Contains(plain, "review") {
			t.Errorf("RuleRowClosable(%v, -1) carries the affordances with no closable segment", widths)
		}
	}
}
