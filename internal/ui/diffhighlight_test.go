package ui_test

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/ui"
)

var fgCode = regexp.MustCompile(`\x1b\[[0-9;]*38;5;([0-9]+)`)

// highlightedDiff is the sample diff open at a width that shows whole lines, the
// cursor moved off the rows under test (reverse video draws them plain).
func highlightedDiff(t *testing.T) *ui.Model {
	t.Helper()
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
	leader(m, key('d'))
	press(m, key('G'))
	return m
}

// A Go line is syntax-coloured while its sign keeps the diff's meaning: the +
// green, and the text in more than one colour (#435). Before, the whole row
// was one green.
func TestDiff_ALineIsSyntaxColouredAndItsSignKeepsItsMeaning_issue435(t *testing.T) {
	m := highlightedDiff(t)

	row := lineWith(t, m.View().Content, "c := 4")
	if !strings.Contains(row, ui.Added("+")) {
		t.Errorf("the added line's sign is not green: %q", row)
	}
	colours := map[string]bool{}
	for _, c := range fgCode.FindAllStringSubmatch(row[strings.LastIndex(row, "│"):], -1) {
		colours[c[1]] = true
	}
	if len(colours) < 2 {
		t.Errorf("the added Go line is drawn in %d colour(s), want the syntax's several: %q", len(colours), row)
	}
}

// In a removed/added pair the words that changed stand out: "2" became "3",
// and that is what a reader has to find (#435, delta).
func TestDiff_TheChangedWordsOfAPairAreEmphasised_issue435(t *testing.T) {
	m := highlightedDiff(t)

	body := m.View().Content
	if !strings.Contains(body, ui.EmphasisRemoved("2")) || !strings.Contains(body, ui.EmphasisAdded("3")) {
		t.Errorf("the changed words of the b := pair are not emphasised:\n%q", body)
	}
	if strings.Contains(body, ui.EmphasisAdded("4")) {
		t.Errorf("an added line with no removed partner was emphasised")
	}
}

// A file chroma has no colours for keeps the diff's whole-line colour.
func TestDiff_AFileWithoutSyntaxKeepsItsLineColour_issue435(t *testing.T) {
	m := highlightedDiff(t)

	if row := lineWith(t, m.View().Content, "fresh"); !strings.Contains(row, ui.Added("fresh")) || !strings.Contains(row, ui.Added("+")) {
		t.Errorf("new.txt's added line lost its green: %q", row)
	}
}
