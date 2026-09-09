package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func frameLines(m *ui.Model) []string { return strings.Split(m.View().Content, "\n") }

func TestFrame_DrawsNoBoxAnywhere_issue174(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	if view := m.View().Content; strings.ContainsAny(view, "╭╮╰╯") {
		t.Errorf("the frame still draws a rounded box:\n%s", view)
	}
}

// Line 0 is the header row, line 1 the rule with a joint under each
// hairline, and every body line has the hairline at the sidebar's edge.
func TestFrame_HeaderRuleAndHairlinesLineUp_issue174(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	lines := frameLines(m)

	head, rule := []rune(stripSGR(lines[0])), stripSGR(lines[1])
	if !strings.HasPrefix(string(head), " projects") || head[ui.SidebarWidth-1] != '│' {
		t.Errorf("header row = %q, want the sidebar title and a hairline at column %d", string(head), ui.SidebarWidth-1)
	}
	if want := strings.Repeat("─", ui.SidebarWidth-1) + "┼" + strings.Repeat("─", 100-ui.SidebarWidth); rule != want {
		t.Errorf("rule = %q\nwant   %q", rule, want)
	}
	for y := 2; y < len(lines)-1; y++ {
		if r := []rune(stripSGR(lines[y])); r[ui.SidebarWidth-1] != '│' {
			t.Errorf("line %d has %q at the sidebar's edge, want the hairline", y, string(r[ui.SidebarWidth-1]))
		}
	}
}

// The accent hairline stands on the left edge of whatever owns the keyboard.
func TestFrame_TheAccentHairlineFollowsTheKeyboardOwner_issue174(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	body := func() string { return frameLines(m)[3] }
	accent, plain := ui.HairlineCell(true), ui.HairlineCell(false)

	if l := body(); strings.Count(l, accent) != 1 || strings.Contains(l, plain) {
		t.Errorf("terminal focused: %q, want one accent hairline and no plain one", stripSGR(l))
	}
	leader(m, key('d')) // the review column opens focused
	if l := body(); strings.Count(l, accent) != 1 || strings.Index(l, plain) > strings.Index(l, accent) {
		t.Errorf("review focused: %q, want the accent on the pane/review hairline, the plain one on the left", stripSGR(l))
	}
	leader(m, key('?')) // a modal owns the keys
	if l := body(); strings.Index(l, accent) > strings.Index(l, plain) {
		t.Errorf("modal open: %q, want the accent on the pane's hairline", stripSGR(l))
	}
}

func TestFrame_NothingOwnsTheKeysOnAnEmptyRegistry_issue174(t *testing.T) {
	m := ui.NewModel(baseDeps(emptyState(), fakeTermsFor(emptyState())))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if strings.Contains(m.View().Content, ui.HairlineCell(true)) {
		t.Error("an empty registry draws an accent hairline, but nothing owns the keyboard")
	}
}

// The frame promise of #35, at the three smoke sizes, with the column open
// and closed: every line is exactly the window.
func TestFrame_EveryLineIsExactlyTheWindow_issue174(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {100, 30}, {120, 32}} {
		for _, open := range []bool{false, true} {
			m, _, _ := modelWithDiff(t)
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			if open {
				leader(m, key('d'))
			}
			assertFrameIs(t, m, size[0], size[1])
		}
	}
}

// assertFrameIs checks the frame is exactly w cells on every one of h lines.
func assertFrameIs(t *testing.T, m *ui.Model, w, h int) {
	t.Helper()
	lines := frameLines(m)
	if len(lines) != h {
		t.Errorf("%dx%d: %d lines, want %d", w, h, len(lines), h)
	}
	for i, l := range lines {
		if got := lipgloss.Width(l); got != w {
			t.Errorf("%dx%d line %d is %d cells: %q", w, h, i, got, stripSGR(l))
		}
	}
}

func TestHeaderRow_AndRuleRow_AreExactlyTheirWidths_issue174(t *testing.T) {
	for _, tt := range []struct {
		titles []string
		widths []int
	}{
		{[]string{"projects", "parser-fix"}, []int{27, 72}},
		{[]string{"projects", strings.Repeat("x", 90), "diff"}, []int{27, 44, 27}},
		{[]string{"", "日本語のタイトル", ""}, []int{27, 12, 5}},
	} {
		sum := len(tt.widths) - 1
		for _, w := range tt.widths {
			sum += w
		}
		if got := ui.HeaderRow(tt.titles, tt.widths, 1); lipgloss.Width(got) != sum {
			t.Errorf("HeaderRow(%v) is %d cells, want %d: %q", tt.widths, lipgloss.Width(got), sum, stripSGR(got))
		}
		if got := ui.RuleRow(tt.widths); lipgloss.Width(got) != sum || strings.Count(stripSGR(got), "┼") != len(tt.widths)-1 {
			t.Errorf("RuleRow(%v) = %q, want %d cells with a joint per hairline", tt.widths, stripSGR(got), sum)
		}
	}
}
