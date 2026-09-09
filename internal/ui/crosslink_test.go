package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// modelGoLines is internal/ui/model.go as the preview reader serves it: long
// enough that line 12 is not clamped to the top by a 30-row window's clamp.
func modelGoLines() string {
	var b strings.Builder
	for i := 1; i <= 60; i++ {
		b.WriteString("line ")
		b.WriteString(string(rune('0' + i%10)))
		b.WriteString("\n")
	}
	return b.String()
}

// modelWithLinks is modelWithTree with the diff's modified file readable, on
// a window short enough that opening at line 12 puts line 12 on top.
func modelWithLinks(t *testing.T) (*ui.Model, *previewReader) {
	t.Helper()
	m, _, _, reader := modelWithTree(t)
	reader.Files["internal/ui/model.go"] = modelGoLines()
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 14})
	return m, reader
}

// sampleDiff's entries: 0 model.go header, 1 hunk, 2 context (new 10),
// 3 removed (old 11), 4 added (new 11), 5 added c := 4 (new 12), ...
const addedLineEntry = 5

// o on a hunk line opens the preview of that file with the line's number on
// the top row, so the operator reads the whole file around the change (#200).
func TestModel_OOnAHunkLineOpensThePreviewAtThatLine_issue200(t *testing.T) {
	m, reader := modelWithLinks(t)
	leader(m, key('d'))
	down(m, addedLineEntry)

	pressAndSettle(m, key('o'))

	if m.ReviewView() != ui.ViewPreview || len(reader.Read) != 1 || reader.Read[0] != "internal/ui/model.go" {
		t.Fatalf("view=%v read=%v; want a preview of model.go", m.ReviewView(), reader.Read)
	}
	rows := previewRows(m.View().Content)
	if len(rows) == 0 || !strings.Contains(stripSGR(rows[0]), "  12  line") {
		t.Errorf("first preview row = %q, want line 12 on top", rows)
	}
}

func TestModel_OOnAFileHeaderOpensThePreviewAtTheTop_issue200(t *testing.T) {
	m, _ := modelWithLinks(t)
	leader(m, key('d'))

	pressAndSettle(m, key('o'))

	rows := previewRows(m.View().Content)
	if m.ReviewView() != ui.ViewPreview || len(rows) == 0 || !strings.Contains(stripSGR(rows[0]), "   1  line") {
		t.Errorf("view=%v first row=%q; want the preview from line 1", m.ReviewView(), rows)
	}
}

// A removed line has no new number; its old number is where it was.
func TestModel_OOnARemovedLineUsesTheOldNumber_issue200(t *testing.T) {
	m, _ := modelWithLinks(t)
	leader(m, key('d'))
	down(m, 3) // removed b := 2, old line 11

	pressAndSettle(m, key('o'))

	rows := previewRows(m.View().Content)
	if len(rows) == 0 || !strings.Contains(stripSGR(rows[0]), "  11  line") {
		t.Errorf("first preview row = %q, want old line 11 on top", rows)
	}
}

// o in the preview goes back to the diff on the line the preview was opened
// at, so o, o is a round trip; a preview opened at the top lands on the
// file's header.
func TestModel_ORoundTripsBetweenTheDiffAndThePreview_issue200(t *testing.T) {
	m, _ := modelWithLinks(t)
	leader(m, key('d'))
	down(m, addedLineEntry)
	pressAndSettle(m, key('o'))

	pressAndSettle(m, key('o'))

	if m.ReviewView() != ui.ViewDiff || m.ReviewCursor() != addedLineEntry {
		t.Errorf("view=%v cursor=%d after o, o; want the diff on entry %d", m.ReviewView(), m.ReviewCursor(), addedLineEntry)
	}
}

func TestModel_OInAPreviewOpenedFromTheTreeLandsOnTheFileHeader_issue200(t *testing.T) {
	m, _ := modelWithLinks(t)
	leader(m, key('f'))
	press(m, key('j')) // ui/
	press(m, key('j')) // model.go
	pressAndSettle(m, special(tea.KeyEnter))
	if m.ReviewView() != ui.ViewPreview {
		t.Fatalf("view = %v, want the preview of model.go", m.ReviewView())
	}

	pressAndSettle(m, key('o'))

	if m.ReviewView() != ui.ViewDiff || m.ReviewCursor() != 0 {
		t.Errorf("view=%v cursor=%d; want the diff on model.go's header", m.ReviewView(), m.ReviewCursor())
	}
}

// A file the diff does not touch has nowhere to jump to; the footer says so
// and the preview stays.
func TestModel_OInAPreviewWithNoHunksExplainsInTheFooter_issue200(t *testing.T) {
	m, _ := modelWithLinks(t)
	leader(m, key('f'))
	foldToGoMod(m)
	pressAndSettle(m, special(tea.KeyEnter))

	pressAndSettle(m, key('o'))

	if m.ReviewView() != ui.ViewPreview {
		t.Errorf("view = %v, want the preview kept", m.ReviewView())
	}
	lineWith(t, m.View().Content, "not in this diff")
}

// From the diff, o then esc must land on a listed tree, not #131's spinner:
// a column opened with ctrl+o d has never listed.
func TestModel_OFromTheDiffThenEscLandsOnAListedTree_issue200(t *testing.T) {
	m, _ := modelWithLinks(t)
	leader(m, key('d'))
	pressAndSettle(m, key('o'))

	press(m, special(tea.KeyEscape))

	view := m.View().Content
	if m.ReviewView() != ui.ViewTree || strings.Contains(view, "listing files") {
		t.Errorf("view=%v after o, esc:\n%s", m.ReviewView(), view)
	}
	lineWith(t, view, "model.go")
}

func TestModel_TheLinkKeyIsDocumented_issue200(t *testing.T) {
	foot := ui.Footers(ui.DefaultLeader)
	if !strings.Contains(foot["reviewFooter"], "o open") || !strings.Contains(foot["treeFooter"], "o diff") {
		t.Errorf("footers do not name o: %q / %q", foot["reviewFooter"], foot["treeFooter"])
	}
}
