package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// The 256-colour SGR each change hue renders as, straight from style.go's
// palette: amber 214, green 78, red 203.
const (
	sgrAmber = "\x1b[38;5;214m"
	sgrGreen = "\x1b[38;5;78m"
	sgrRed   = "\x1b[38;5;203m"
)

// deletedAndRenamed is a diff the fixture lacks: one file gone, one moved.
func deletedAndRenamed() review.Diff {
	return review.Diff{Files: []review.File{
		{Path: "gone.go", Status: review.FileDeleted},
		{Path: "internal/ui/render.go", OldPath: "internal/ui/old.go", Status: review.FileRenamed},
	}}
}

// Each kind of change shows its letter in the hue the diff already gives that
// state, and the directory above rolls up to M (#196).
func TestModel_TreeRowsCarryTheKindOfChange_issue196(t *testing.T) {
	m, _, _, _ := modelWithTree(t)
	leader(m, key('f'))

	view := m.View().Content
	if row := lineWith(t, view, "M model.go"); !strings.Contains(row, sgrAmber) {
		t.Errorf("modified row is not amber: %q", row)
	}
	if row := lineWith(t, view, "A new.txt"); !strings.Contains(row, sgrGreen) {
		t.Errorf("added row is not green: %q", row)
	}
	lineWith(t, view, "M ▾ internal/")

	m.Update(ui.DiffLoadedMsg{SessionID: "s1", Diff: deletedAndRenamed()})

	view = m.View().Content
	if row := lineWith(t, view, "R render.go"); !strings.Contains(row, sgrAmber) {
		t.Errorf("renamed row is not amber: %q", row)
	}
	if strings.Contains(view, "M model.go") {
		t.Error("model.go still marked after a diff that no longer touches it")
	}
}

// A deleted file is in the diff but not in the listing; it gets a row once
// the listing is rebuilt, in red, and enter on it explains rather than
// failing to read a file that is not there.
func TestModel_ADeletedFileIsARowAndEnterOnItDoesNotError_issue196(t *testing.T) {
	m, _, lister, reader := modelWithTree(t)
	leader(m, key('f'))
	m.Update(ui.DiffLoadedMsg{SessionID: "s1", Diff: deletedAndRenamed()})
	_, cmd := m.Update(ui.FilesLoadedMsg{SessionID: "s1", Paths: lister.Paths})
	deliver(m, cmd)

	view := m.View().Content
	if row := lineWith(t, view, "D gone.go"); !strings.Contains(row, sgrRed) {
		t.Errorf("deleted row is not red: %q", row)
	}

	press(m, special(tea.KeyEnter)) // fold internal/
	press(m, key('j'))              // go.mod
	press(m, key('j'))              // gone.go
	pressAndSettle(m, special(tea.KeyEnter))

	if m.ReviewView() != ui.ViewPreview || len(reader.Read) != 0 {
		t.Fatalf("view=%v read=%v; want a preview that asked the reader nothing", m.ReviewView(), reader.Read)
	}
	lineWith(t, m.View().Content, "deleted in this session")
}

// The legend replaces the old * explanation where the operator looks for it.
func TestModel_HelpExplainsTheChangeLetters_issue196(t *testing.T) {
	m, _, _, _ := modelWithTree(t)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})

	leader(m, key('?'))

	lineWith(t, m.View().Content, "M A D R")
}
