package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// typeNote presses c on the current row, types text and presses enter.
func typeNote(m *ui.Model, text string) {
	press(m, key('c'))
	for _, r := range text {
		press(m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	press(m, special(tea.KeyEnter))
}

// down moves the review cursor n rows.
func down(m *ui.Model, n int) {
	for range n {
		press(m, key('j'))
	}
}

func TestModel_JAndKMoveTheReviewCursorWithinBounds_issue22(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))

	press(m, key('k'))
	if m.ReviewCursor() != 0 {
		t.Errorf("k at the top moved the cursor to %d", m.ReviewCursor())
	}
	down(m, 3)
	if m.ReviewCursor() != 3 {
		t.Errorf("after three j: cursor = %d, want 3", m.ReviewCursor())
	}
	down(m, 100)
	if m.ReviewCursor() != 11 {
		t.Errorf("cursor ran past the last row: %d, want 11 (12 rows without comments)", m.ReviewCursor())
	}
}

// Row 4 is "+\tb := 3" (file, hunk, a, -b, +b). A note there is anchored on
// that content and rendered beneath the line.
func TestModel_CQueuesANoteOnTheCurrentLine_issue22(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 4)

	typeNote(m, "Use a match here")

	if m.PendingComments() != 1 {
		t.Fatalf("pending = %d, want 1", m.PendingComments())
	}
	view := m.View().Content
	bLine := strings.Index(view, "b := 3")
	note := strings.Index(view, ">> Use a match here")
	if bLine < 0 || note < 0 || note < bLine {
		t.Errorf("note is not rendered beneath its line:\n%s", view)
	}
}

// KeyPressMsg.Text carries the typed character with modifiers applied, so a
// capital or a space must land as itself, not as the keystroke name.
func TestModel_NoteEditorTakesTextNotKeystrokeNames_issue22(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 4)
	press(m, key('c'))

	press(m, tea.KeyPressMsg{Code: 'f', Mod: tea.ModShift, Text: "F"})
	press(m, tea.KeyPressMsg{Code: tea.KeySpace, Text: " "})
	press(m, key('x'))

	if !strings.Contains(m.View().Content, "note: F x_") {
		t.Errorf("editor shows the wrong buffer:\n%s", m.View().Content)
	}
}

func TestModel_EscDiscardsTheNoteAndEnterRefusesAnEmptyOne_issue22(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 4)

	press(m, key('c'))
	press(m, special(tea.KeyEnter))
	if !strings.Contains(m.View().Content, "note: _") {
		t.Error("enter on an empty note closed the editor; it should stay open")
	}
	press(m, special(tea.KeyEscape))
	if m.PendingComments() != 0 || strings.Contains(m.View().Content, "note: ") {
		t.Error("esc did not discard the note")
	}
	if !m.ReviewFocused() {
		t.Error("esc in the editor should return to the pane, not the terminal")
	}
}

func TestModel_CDoesNothingOnAHeaderRow_issue22(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))

	press(m, key('c')) // row 0 is the file header
	press(m, key('x'))

	if strings.Contains(m.View().Content, "note:") {
		t.Error("the editor opened on a file header")
	}
}

func TestModel_DDeletesTheCommentUnderTheCursor_issue22(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 4)
	typeNote(m, "drop me")
	down(m, 1) // onto the comment row beneath the line

	press(m, key('d'))

	if m.PendingComments() != 0 {
		t.Errorf("pending = %d after d on the comment, want 0", m.PendingComments())
	}
}

// d on a diff line is not a delete: only a comment row can be deleted, or the
// operator would lose notes by pressing d in the wrong place.
func TestModel_DOnADiffLineKeepsTheComments_issue22(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 4)
	typeNote(m, "keep me")

	press(m, key('d'))

	if m.PendingComments() != 1 {
		t.Errorf("pending = %d after d on a diff line, want 1", m.PendingComments())
	}
}

// Claude edited the file while the operator was reading: the reload changes
// the diff, and the comment follows its content rather than its line number.
func TestModel_CommentsSurviveAReload_issue22(t *testing.T) {
	m, _, rec := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 4)
	typeNote(m, "keep me")
	shifted := strings.Replace(sampleDiff, "@@ -10,4 +10,5 @@", "@@ -30,4 +31,5 @@", 1)
	d, err := review.ParseDiff(strings.NewReader(shifted))
	if err != nil {
		t.Fatal(err)
	}
	rec.Diff = d

	_, cmd := m.Update(key('r'))
	deliver(m, cmd)

	if m.PendingComments() != 1 || !strings.Contains(m.View().Content, ">> keep me") {
		t.Errorf("comment lost across reload:\n%s", m.View().Content)
	}
}

func TestScrollOffset_KeepsTheCursorVisible(t *testing.T) {
	cases := []struct{ cursor, offset, rows, want int }{
		{0, 0, 10, 0},
		{5, 0, 10, 0},
		{10, 0, 10, 1},
		{25, 3, 10, 16},
		{2, 8, 10, 2},
		{3, 0, 0, 0},
	}
	for _, c := range cases {
		if got := ui.ScrollOffset(c.cursor, c.offset, c.rows); got != c.want {
			t.Errorf("ScrollOffset(%d, %d, %d) = %d, want %d",
				c.cursor, c.offset, c.rows, got, c.want)
		}
	}
}
