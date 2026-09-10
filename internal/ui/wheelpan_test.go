package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// The review column's horizontal axis, driven by the mouse (#125). The
// vertical axis and the pointer geometry it shares live in wheel_test.go.

// fixtureWidth is the window these fixtures are sized against: wide enough
// that the review column has content columns to spare, narrow enough that the
// fixture lines still overflow it.
const fixtureWidth = 100

// overReviewAt is a window cell inside the review column of a window that
// wide, so a fixture sized for its content can still find the column. The
// no-argument overReview in wheel_test.go assumes DefaultWidth.
func overReviewAt(width int) (x, y int) {
	_, oy := ui.PaneOrigin()
	return width - ui.ReviewWidth(width, true) + 1, oy + 3
}

func wheelMod(x, y int, b tea.MouseButton, mod tea.KeyMod) tea.MouseWheelMsg {
	return tea.MouseWheelMsg{X: x, Y: y, Button: b, Mod: mod}
}

// modelWithWideDiff opens the diff on a one-file diff whose added line is far
// wider than the column, which is the state the operator reports the bug from.
func modelWithWideDiff(t *testing.T) *ui.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: wideDiff(t)}).fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: fixtureWidth, Height: 30})
	leader(m, key('d'))
	return m
}

// modelWithWideTree opens the file tree on a path deeper than the column is
// wide. The tree is the other view the operator named, and it clamps against a
// different width builder than the diff does.
func modelWithWideTree(t *testing.T) *ui.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn
	d.Files = (&fileLister{Paths: []string{"a/b/c/d/a-very-long-file-name-TREE_END.go"}}).fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: fixtureWidth, Height: 30})
	leader(m, key('f'))
	return m
}

// requirePannable proves the fixture is wider than the column using the
// keyboard path #94 already shipped, then returns it to the left edge. Without
// it every assertion below passes on a fixture that simply has nothing to pan,
// and reads as "the wheel works" when the wheel was never asked anything.
func requirePannable(t *testing.T, m *ui.Model) {
	t.Helper()
	press(m, key('l'))
	if m.ReviewColOffset() == 0 {
		t.Fatal("the fixture is not wider than the column; nothing here can prove a pan")
	}
	press(m, key('0'))
}

// Regression, issue #125: wheelDirection mapped only the vertical buttons and
// dropped everything else, so a trackpad's two-finger horizontal swipe - X11
// buttons 6 and 7 - did nothing at all. It is the axis that always arrives,
// because no terminal intercepts it the way they intercept shift.
func TestUpdate_ASidewaysWheelPansTheReviewColumn_issue125(t *testing.T) {
	m := modelWithWideDiff(t)
	requirePannable(t, m)
	x, y := overReviewAt(fixtureWidth)
	before := m.ReviewCursor()

	m.Update(wheel(x, y, tea.MouseWheelRight))

	if got := m.ReviewColOffset(); got != ui.PanStep {
		t.Errorf("ReviewColOffset() = %d after a right notch, want %d", got, ui.PanStep)
	}
	if got := m.ReviewCursor(); got != before {
		t.Errorf("the sideways notch moved the cursor to %d, want it left at %d", got, before)
	}

	m.Update(wheel(x, y, tea.MouseWheelLeft))

	if got := m.ReviewColOffset(); got != 0 {
		t.Errorf("ReviewColOffset() = %d after a left notch, want back at 0", got)
	}
}

// Regression, issue #125: scrollPane never read msg.Mod, so shift+wheel was
// indistinguishable from a plain wheel and moved the cursor down the diff
// instead of panning it. Best-effort by nature - Ghostty, kitty, xterm and
// Alacritty keep shift for text selection - which is why the buttons above
// carry the feature and this only adds to it.
func TestUpdate_ShiftAndTheWheelPanTheReviewColumn_issue125(t *testing.T) {
	m := modelWithWideDiff(t)
	requirePannable(t, m)
	x, y := overReviewAt(fixtureWidth)
	before := m.ReviewCursor()

	m.Update(wheelMod(x, y, tea.MouseWheelDown, tea.ModShift))

	if got := m.ReviewColOffset(); got != ui.PanStep {
		t.Errorf("ReviewColOffset() = %d after shift+wheel down, want %d", got, ui.PanStep)
	}
	if got := m.ReviewCursor(); got != before {
		t.Errorf("shift+wheel moved the cursor to %d, want it left at %d", got, before)
	}

	m.Update(wheelMod(x, y, tea.MouseWheelUp, tea.ModShift))

	if got := m.ReviewColOffset(); got != 0 {
		t.Errorf("ReviewColOffset() = %d after shift+wheel up, want back at 0", got)
	}
}

// A plain vertical notch must keep moving the cursor: the modifier is what
// picks the axis, so adding one must not cost the other (#107).
func TestUpdate_APlainNotchStillMovesTheReviewCursor_issue125(t *testing.T) {
	m := modelWithWideDiff(t)
	x, y := overReviewAt(fixtureWidth)
	before := m.ReviewCursor()

	m.Update(wheel(x, y, tea.MouseWheelDown))

	if m.ReviewCursor() == before {
		t.Errorf("a plain notch left the cursor at %d, want it moved", before)
	}
	if got := m.ReviewColOffset(); got != 0 {
		t.Errorf("a plain notch panned to %d, want the column left alone", got)
	}
}

// The state the operator is actually in: claude owns the keyboard while it
// edits, so h/l cannot be reached without first taking focus off the session.
// The wheel is answered by geometry rather than by focus, and must pan from
// there (#125).
func TestUpdate_TheSidewaysWheelPansWithTheColumnUnfocused_issue125(t *testing.T) {
	m := modelWithWideDiff(t)
	requirePannable(t, m)
	press(m, special(tea.KeyEscape)) // column open, keyboard back on the pane
	if m.ReviewFocused() {
		t.Fatal("esc left the review column focused; the test proves nothing")
	}

	x, y := overReviewAt(fixtureWidth)
	m.Update(wheel(x, y, tea.MouseWheelRight))

	if got := m.ReviewColOffset(); got != ui.PanStep {
		t.Errorf("ReviewColOffset() = %d with the column unfocused, want %d", got, ui.PanStep)
	}
}

// Over the session pane a sideways notch stays dropped: claude has no
// horizontal scroll, and inventing one would repeat the arrow-key corruption
// #107 fixed.
func TestUpdate_ASidewaysWheelOverTheSessionReachesNobody_issue125(t *testing.T) {
	m, fakes := modelWithFakes(t)
	x, y := overPane()

	for range ui.WheelNotchesPerPage * 2 {
		m.Update(wheel(x, y, tea.MouseWheelLeft))
		m.Update(wheel(x, y, tea.MouseWheelRight))
	}

	for id, f := range fakes {
		if len(f.Sent) != 0 || len(f.Msgs) != 0 {
			t.Errorf("%s got Sent=%q Msgs=%v from a sideways notch, want nothing", id, f.Sent, f.Msgs)
		}
	}
}

// The mouse must not walk the text off the screen where h/l cannot (#94), in
// the tree as well as the diff: the two clamp against different width builders,
// so one of them passing says nothing about the other.
func wheelToBothEdges(t *testing.T, m *ui.Model, view string) {
	t.Helper()
	requirePannable(t, m)
	x, y := overReviewAt(fixtureWidth)

	for range 200 {
		m.Update(wheel(x, y, tea.MouseWheelRight))
	}
	atEnd := m.ReviewColOffset()
	if atEnd == 0 {
		t.Fatalf("200 right notches never moved the %s at all", view)
	}

	m.Update(wheel(x, y, tea.MouseWheelRight))
	if got := m.ReviewColOffset(); got != atEnd {
		t.Errorf("the %s kept growing past its longest line: %d then %d", view, atEnd, got)
	}

	for range 200 {
		m.Update(wheel(x, y, tea.MouseWheelLeft))
	}
	if got := m.ReviewColOffset(); got != 0 {
		t.Errorf("the %s offset = %d after wheeling left off the edge, want 0", view, got)
	}
}

func TestUpdate_TheSidewaysWheelStopsAtBothEdgesOfTheDiff_issue125(t *testing.T) {
	wheelToBothEdges(t, modelWithWideDiff(t), "diff")
}

func TestUpdate_TheSidewaysWheelStopsAtBothEdgesOfTheTree_issue125(t *testing.T) {
	wheelToBothEdges(t, modelWithWideTree(t), "tree")
}

// Half of #125 is discoverability: h/l/0 have panned since #94, but neither
// review footer said so, so from the operator's seat the axis did not exist.
// treeFooter has the room; reviewFooter does not, and points at ctrl+o ?
// instead - which the width assertion in help_test.go holds both to.
func TestModel_theTreeFooterNamesThePanKeys_issue125(t *testing.T) {
	if got := ui.Footers(ui.DefaultLeader)["treeFooter"]; !strings.Contains(got, "h/l/0 pan") {
		t.Errorf("treeFooter does not name the pan keys: %q", got)
	}
}

// And the help modal is where a key that does not fit a footer lives (#103).
// It named h/l as panning "a wide diff" when one offset serves the tree and
// the preview too, and said nothing about the wheel at all.
func TestModel_helpNamesTheSidewaysWheel_issue125(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	press(m, ctrl('o'))
	press(m, key('?'))

	got := m.View().Content
	for _, want := range []string{"h / l", "wheel sideways", "left edge"} {
		if !strings.Contains(got, want) {
			t.Errorf("the help modal does not name %q:\n%s", want, got)
		}
	}
}
