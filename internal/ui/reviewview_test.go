package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func lineWith(t *testing.T, view, needle string) string {
	t.Helper()
	for _, l := range strings.Split(view, "\n") {
		if strings.Contains(l, needle) {
			return l
		}
	}
	t.Fatalf("no line contains %q:\n%s", needle, view)
	return ""
}

func TestModel_ReviewShowsFileHeadersWithCountsAndHunkHeaders_issue21(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	leader(m, key('d'))

	view := m.View().Content

	lineWith(t, view, "internal/ui/model.go +2 -1")
	lineWith(t, view, "new.txt +2 -0")
	lineWith(t, view, "@@ -10,4 +10,5 @@")
	if !strings.Contains(lineWith(t, view, "diff ·"), "2 files · 0 comments") {
		t.Errorf("title lacks the counts:\n%s", view)
	}
}

func TestModel_ReviewPrefixesLinesWithTheirSign_issue21(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))

	view := m.View().Content

	// Tabs are widened to four spaces so lipgloss measures what is drawn.
	if !strings.Contains(lineWith(t, view, "b := 2"), "-    b := 2") {
		t.Error("removed line lacks its - prefix")
	}
	if !strings.Contains(lineWith(t, view, "b := 3"), "+    b := 3") {
		t.Error("added line lacks its + prefix")
	}
}

func TestModel_ReviewFrameNeverExceedsTheWindow_issue21(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 4)
	typeNote(m, "a note that is quite long and will need to be cut to fit the narrow column")

	for i, l := range strings.Split(m.View().Content, "\n") {
		if w := lipgloss.Width(l); w > 100 {
			t.Errorf("line %d is %d wide: %q", i, w, l)
		}
	}
}

func TestModel_ReviewScrollsToKeepTheCursorVisible_issue21(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 10}) // 7 pane rows, 6 entry rows
	leader(m, key('d'))

	down(m, 11) // the last row, "file" in new.txt

	view := m.View().Content
	if !strings.Contains(view, "file") || strings.Contains(view, "@@ -10,4") {
		t.Errorf("view did not scroll to the cursor:\n%s", view)
	}
}

func TestModel_FooterSwapsToTheReviewKeymapWhileFocused_issue21(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))

	lines := strings.Split(strings.TrimRight(m.View().Content, "\n"), "\n")
	last := lines[len(lines)-1]
	for _, want := range []string{"c comment", "S submit", "esc back"} {
		if !strings.Contains(last, want) {
			t.Errorf("review footer lacks %q: %q", want, last)
		}
	}
	press(m, special(tea.KeyEscape))
	lines = strings.Split(strings.TrimRight(m.View().Content, "\n"), "\n")
	if !strings.Contains(lines[len(lines)-1], ui.Leader+" d diff") {
		t.Errorf("main footer lacks the diff key: %q", lines[len(lines)-1])
	}
}

func TestModel_ReviewOfACleanTreeSaysSo_issue21(t *testing.T) {
	m, _, rec := modelWithDiff(t)
	rec.Diff = review.Diff{}

	leader(m, key('d'))

	if !strings.Contains(m.View().Content, "no changes") {
		t.Errorf("empty diff renders nothing helpful:\n%s", m.View().Content)
	}
}

// The border says where the next keystroke lands, so moving focus must change
// which box is highlighted.
func TestModel_TerminalBorderDimsWhileTheReviewHasFocus_issue21(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	before := m.View().Content

	leader(m, key('d'))

	if m.View().Content == before {
		t.Error("focus moved but nothing about the frame changed")
	}
}

// An orphan is rendered where it floats, at the top of its file, and says so:
// the note is still yours to act on even though its line went away.
func TestModel_OrphanedCommentIsMarkedMoved_issue22(t *testing.T) {
	m, _, rec := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 4)
	typeNote(m, "gone now")
	changed := strings.Replace(sampleDiff, "+\tb := 3", "+\tb := 99", 1)
	d, err := review.ParseDiff(strings.NewReader(changed))
	if err != nil {
		t.Fatal(err)
	}
	rec.Diff = d

	_, cmd := m.Update(key('r'))
	deliver(m, cmd)

	view := m.View().Content
	orphan := lineWith(t, view, "gone now")
	if !strings.Contains(orphan, "moved") {
		t.Errorf("orphan is not marked moved: %q", orphan)
	}
	if strings.Index(view, "gone now") > strings.Index(view, "@@ -10,4") {
		t.Errorf("orphan does not float above its file's hunks:\n%s", view)
	}
}
