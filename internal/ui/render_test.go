package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// fakeTermsFor builds a fake terminal per session in st, for fixtures larger
// than the three fakeTerms knows.
func fakeTermsFor(st registry.State) map[string]termwrap.Terminal {
	terms := make(map[string]termwrap.Terminal, len(st.Sessions))
	for _, sess := range st.Sessions {
		terms[sess.ID] = termwrap.NewFake(sess.ID)
	}
	return terms
}

// Seven projects at the default 80x24 overflow the sidebar. The selected row
// past the fold was not drawn at all and the cursor moved onto rows nobody
// could see (#129).
func TestModel_SidebarShowsTheSelectedRowPastTheFold_issue129(t *testing.T) {
	st := sevenProjectState()
	m := ui.NewModel(baseDeps(st, fakeTermsFor(st)))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	for range 13 {
		leader(m, key('j'))
	}
	if m.Selected() != "p6-s1" {
		t.Fatalf("Selected() = %q, want p6-s1 after 13 moves", m.Selected())
	}

	view := m.View().Content

	if !strings.Contains(view, "» ") {
		t.Error("the sidebar shows no cursor marker: the selected row is off screen")
	}
	if !strings.Contains(view, "projects") {
		t.Error("the pinned header is missing")
	}
	if got := strings.Count(view, "\n") + 1; got != 24 {
		t.Errorf("the frame is %d lines, want 24", got)
	}

	for range 13 {
		leader(m, key('k'))
	}
	if m.Selected() != "p0-s0" || !strings.Contains(m.View().Content, "> p0") {
		t.Errorf("after 13 k presses Selected() = %q and the top project is not drawn", m.Selected())
	}
}
