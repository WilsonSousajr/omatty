package ui_test

import (
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/watcher"
	"strings"
	"testing"
	"time"

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

	if !strings.Contains(stripSGR(view), "▎") {
		t.Error("the sidebar shows no cursor rail: the selected card is off screen")
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
	if m.Selected() != "p0-s0" || !strings.HasPrefix(stripSGR(frameLines(m)[2]), " p0") {
		t.Errorf("after 13 k presses Selected() = %q and the top project is not drawn", m.Selected())
	}
}

// F2's regression test: the lane is the first non-ASCII thing in the row
// expression, where len(age) was a byte count (#128, constraint 4).
func TestRenderRow_EveryRowIsExactlySidebarWidth_issue128(t *testing.T) {
	st := twoProjectState()
	st.Sessions[1].Title = "日本語のタイトルです"
	st.Sessions[2].Title = "t\u202eitle"
	m := ui.NewModel(baseDeps(st, fakeTermsFor(st)))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	status(m, "s1", watcher.PromptSubmitted, time.Now())
	status(m, "s3", watcher.PermissionRequested, time.Now())

	for i, line := range strings.Split(m.View().Content, "\n") {
		if w := lipgloss.Width(line); w != 100 {
			t.Errorf("line %d is %d cells, want 100: %q", i, w, line)
		}
	}
}
