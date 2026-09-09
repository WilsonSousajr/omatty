package ui_test

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// frameWidth measures a frame row in cells, escapes excluded.
func frameWidth(row string) int { return lipgloss.Width(row) }

const goFile = "package main\n\n// add adds\nfunc add(a int) int {\n\treturn a + 42\n}\n"

// openPreviewOf lists the tree with rel as its only file, previews it, and
// returns the frame.
func openPreviewOf(t *testing.T, rel, body string, width int) (*ui.Model, string) {
	t.Helper()
	m, _, lister, reader := modelWithTree(t)
	m.Update(tea.WindowSizeMsg{Width: width, Height: 30})
	lister.Paths = []string{rel}
	reader.Files[rel] = body
	leader(m, key('f'))
	pressAndSettle(m, special(tea.KeyEnter))
	if m.ReviewView() != ui.ViewPreview {
		t.Fatalf("view = %v after enter on %s, want the preview", m.ReviewView(), rel)
	}
	return m, m.View().Content
}

// previewRows are the review column's part of the frame lines carrying a
// preview gutter number: the sidebar and the pane on the same row carry
// their own colour and are not under test.
func previewRows(frame string) []string {
	var out []string
	for _, line := range strings.Split(frame, "\n") {
		if regexp.MustCompile(`│ +\d+  `).MatchString(stripSGR(line)) {
			out = append(out, reviewPart(line))
		}
	}
	return out
}

// reviewPart is the row from the review column's hairline on.
func reviewPart(row string) string { return row[strings.LastIndex(row, "│"):] }

// A Go preview carries colour, and stripping it gives exactly the plain
// rendering: the gutter, the tabs and the width are the plain line's (#197).
func TestModel_APreviewIsHighlightedAndReadsPlainWithoutTheColour_issue197(t *testing.T) {
	_, plain := openPreviewOf(t, "notes.zzz", goFile, 100)
	_, styled := openPreviewOf(t, "main.go", goFile, 100)

	plainRows := strings.Join(previewRows(plain), "\n")
	rows := previewRows(styled)
	if got := stripSGR(strings.Join(rows, "\n")); got != stripSGR(plainRows) {
		t.Errorf("highlighted rows read differently from the plain ones:\n%s\n---\n%s", got, stripSGR(plainRows))
	}
	if len(rows) < 6 {
		t.Fatalf("%d preview rows, want the whole file", len(rows))
	}
	coloured := 0
	for _, r := range rows {
		if strings.Contains(r, "\x1b[38;5;") {
			coloured++
		}
	}
	if coloured < 3 {
		t.Errorf("%d of %d preview rows carry colour, want keywords, a comment and a number coloured", coloured, len(rows))
	}
}

// Panning a highlighted line cuts by cell and never inside an escape: no bare
// "[38;5;" fragment is ever drawn.
func TestModel_PanningAHighlightedLineNeverShowsAnEscapeFragment_issue197(t *testing.T) {
	long := "package main\n\nvar s = \"" + strings.Repeat("word ", 30) + "\" // " + strings.Repeat("c", 80) + "\n"
	m, _ := openPreviewOf(t, "main.go", long, 100)

	for range 20 {
		press(m, key('l'))
		frame := m.View().Content
		for _, row := range strings.Split(frame, "\n") {
			plain := stripSGR(row)
			if strings.Contains(plain, "[38;5;") || strings.Contains(plain, "[0m") {
				t.Fatalf("at ColOffset %d a row shows an escape fragment: %q", m.ReviewColOffset(), row)
			}
		}
		for _, row := range strings.Split(frame, "\n") {
			if w := frameWidth(row); w != 100 {
				t.Fatalf("at ColOffset %d a row is %d cells wide, want 100: %q", m.ReviewColOffset(), w, row)
			}
		}
	}
	if m.ReviewColOffset() == 0 {
		t.Error("l never panned the highlighted preview")
	}
}

// A file over the highlight budget draws plain, and says so under its last
// line the way the truncation notice does.
func TestModel_AFileOverTheBudgetDrawsPlainWithANote_issue197(t *testing.T) {
	big := strings.Repeat("var x = 1 // "+strings.Repeat("y", 100)+"\n", 700) // ~80 KiB
	m, frame := openPreviewOf(t, "big.go", big, 100)

	if strings.Contains(frame, "\x1b[38;5;1") {
		t.Error("a file over the budget was highlighted")
	}
	for range 800 {
		press(m, key('j'))
	}
	lineWith(t, m.View().Content, "... not highlighted")
}

// A file chroma has no lexer for renders exactly as today: no colour, no note.
func TestModel_AnUnknownFileTypeRendersAsBefore_issue197(t *testing.T) {
	_, frame := openPreviewOf(t, "notes.zzz", goFile, 100)
	for _, row := range previewRows(frame) {
		if strings.Contains(row, "\x1b[38;5;") {
			t.Errorf("an unknown file type was coloured: %q", row)
		}
	}
	if strings.Contains(frame, "not highlighted") {
		t.Error("an unknown file type carries the over-budget note")
	}
}
