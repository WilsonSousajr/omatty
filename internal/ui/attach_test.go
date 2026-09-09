package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// a on a file writes exactly one bracketed @path with a trailing space and
// no carriage return to the session's terminal, then hands the keys back so
// the operator keeps typing in claude's composer (#199, invariant 8).
func TestModel_AAttachesTheSelectedFileAsAPathReference_issue199(t *testing.T) {
	m, fakes, _, _ := modelWithTree(t)
	leader(m, key('f'))
	foldToGoMod(m)

	pressAndSettle(m, key('a'))

	if got := strings.Join(fakes["s1"].Sent, "|"); got != "\x1b[200~@go.mod \x1b[201~" {
		t.Errorf("sent %q, want one bracketed @go.mod with a trailing space and no CR", got)
	}
	if m.ReviewFocused() {
		t.Error("focus stayed on the column; the next keystroke should reach claude")
	}
	for _, other := range []string{"s2", "s3"} {
		if len(fakes[other].Sent) != 0 {
			t.Errorf("%s received %q", other, fakes[other].Sent)
		}
	}
}

// A directory row attaches as @dir/, so claude reads it as a directory.
func TestModel_AOnADirectoryAttachesItWithATrailingSlash_issue199(t *testing.T) {
	m, fakes, _, _ := modelWithTree(t)
	leader(m, key('f')) // cursor on internal/ (#194)

	pressAndSettle(m, key('a'))

	if got := strings.Join(fakes["s1"].Sent, "|"); got != "\x1b[200~@internal/ \x1b[201~" {
		t.Errorf("sent %q, want @internal/ bracketed", got)
	}
}

// The preview offers the same key for the file it shows.
func TestModel_AInThePreviewAttachesTheShownFile_issue199(t *testing.T) {
	m, fakes, _, _ := modelWithTree(t)
	leader(m, key('f'))
	foldToGoMod(m)
	pressAndSettle(m, special(tea.KeyEnter))
	if m.ReviewView() != ui.ViewPreview {
		t.Fatalf("view = %v, want the preview", m.ReviewView())
	}

	pressAndSettle(m, key('a'))

	if got := strings.Join(fakes["s1"].Sent, "|"); got != "\x1b[200~@go.mod \x1b[201~" {
		t.Errorf("sent %q, want @go.mod bracketed", got)
	}
	if m.ReviewFocused() {
		t.Error("focus stayed on the column")
	}
}

// A session with no terminal gets a footer error, not a panic (#76's rule).
func TestModel_AWithNoTerminalExplainsInTheFooter_issue199(t *testing.T) {
	terms, _ := fakeTerms(t)
	delete(terms, "s1")
	terms["s1"] = nil
	lister := &fileLister{Paths: []string{"go.mod"}}
	d := baseDeps(twoProjectState(), terms)
	d.Diff = (&diffRecorder{Diff: sampleDiffParsed(t)}).fn
	d.Files = lister.fn
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('f'))

	pressAndSettle(m, key('a'))

	if !strings.Contains(m.View().Content, "no terminal") {
		t.Errorf("footer does not explain the missing terminal:\n%s", m.View().Content)
	}
	if !m.ReviewFocused() {
		t.Error("a failed attach dropped the column's focus")
	}
}

// The key is where the operator looks for it (#103).
func TestModel_TheAttachKeyIsDocumented_issue199(t *testing.T) {
	if got := ui.Footers(ui.DefaultLeader)["treeFooter"]; !strings.Contains(got, "a attach") {
		t.Errorf("treeFooter does not name the attach key: %q", got)
	}
	m, _, _, _ := modelWithTree(t)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	leader(m, key('?'))
	lineWith(t, m.View().Content, "@path")
}
