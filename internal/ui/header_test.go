package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func headerOf(m *ui.Model) string { return stripSGR(frameLines(m)[0]) }

// The right side collapses in steps: the counts, then the meter, then the
// branch; last, the left side is clipped. Widths chosen around the parts:
// crumb 19, branch 4, state 12, meter 19, counts 20.
func TestCollapse_DropsTheRightSideInTheSpecsOrder_issue177(t *testing.T) {
	p := ui.HeaderParts{Crumb: "omatty ▎ parser-fix", Branch: "main", State: "● waiting 4m",
		Meter: "▰▰▰▰▰▱▱▱ 62% cached", Counts: "60.2k in / 62.6k out"}
	for _, tt := range []struct {
		width      int
		want, drop string
	}{
		{100, "60.2k in", ""},
		{70, "62% cached", "60.2k in"},
		{45, "main", "cached"},
		{38, "waiting", "main"},
		{20, "omatty", "waiting"},
	} {
		got := ui.Collapse(tt.width, p)
		if lipgloss.Width(got) != tt.width || !strings.Contains(got, tt.want) || (tt.drop != "" && strings.Contains(got, tt.drop)) {
			t.Errorf("Collapse(%d) = %q (%d cells): want %q kept, %q dropped", tt.width, got, lipgloss.Width(got), tt.want, tt.drop)
		}
	}
}

func TestHeader_ReadsProjectTitleBranchStatusAndUsage_issue177(t *testing.T) {
	m, _, _ := modelWithEvents(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: fixedNow.Add(-4 * time.Minute)})
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.UsageUpdated, At: fixedNow,
		Tokens: watcher.Tokens{In: 2000, CacheRead: 8000, Out: 500}})

	head := headerOf(m)

	for _, want := range []string{" projects · 2", "│ omatty ▎ main · ● waiting 4m", "▰▰▰▰▰▰▱▱ 80% cached · 10.0k in / 500 out"} {
		if !strings.Contains(head, want) {
			t.Errorf("header %q lacks %q", head, want)
		}
	}
	if !strings.HasSuffix(strings.TrimRight(head, " "), "out") || lipgloss.Width(frameLines(m)[0]) != 120 {
		t.Errorf("the usage is not right-aligned on a 120-cell row: %q", head)
	}
}

// At 60 columns the pane is 32: the counts and the meter go, the status
// stays, and the footer's exit key is still first.
func TestHeader_CollapsesAtSixtyColumnsAndTheExitKeyStays_issue177(t *testing.T) {
	m, _, _ := modelWithEvents(t)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: fixedNow})
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.UsageUpdated, At: fixedNow, Tokens: watcher.Tokens{In: 2000, Out: 500}})
	head := headerOf(m)
	if strings.Contains(head, "cached") || strings.Contains(head, " in /") || !strings.Contains(head, "waiting") {
		t.Errorf("header at 60 = %q, want the usage collapsed and the status kept", head)
	}
	if !strings.HasPrefix(stripSGR(footerOf(m)), " ctrl+o q quit") {
		t.Errorf("footer at 60 = %q, the exit key moved", stripSGR(footerOf(m)))
	}
}

func TestHeader_NamesEachModalSurface_issue177(t *testing.T) {
	for _, tt := range []struct {
		open tea.KeyPressMsg
		want string
	}{
		{key('n'), "new session"}, {key('N'), "new worktree session"}, {key('R'), "rename"},
		{key('x'), "confirm"}, {key('/'), "switch"}, {key('?'), "keys"},
	} {
		m, _ := modelWithFakes(t)
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
		leader(m, tt.open)
		if head := headerOf(m); !strings.Contains(head, "│ "+tt.want) {
			t.Errorf("after %q the header reads %q, want %q", tt.open.Keystroke(), head, tt.want)
		}
	}
	if names := ui.ModalNames(); len(names) != 8 {
		t.Errorf("%d modal names, want 8 (register project and adopt session need a scan to open)", len(names))
	}
}

func TestHeader_IsEmptyForThePaneWithNoSession_issue177(t *testing.T) {
	m := ui.NewModel(baseDeps(emptyState(), fakeTermsFor(emptyState())))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if head := headerOf(m); strings.TrimSpace(strings.TrimPrefix(head, " projects · 0")) != "│" {
		t.Errorf("header = %q, want the projects count, the hairline and nothing else", head)
	}
}

// The breadcrumb reads the branch the card's poll found (#180).
func TestHeader_CarriesTheBranchOncePolled_issue177(t *testing.T) {
	m, _ := modelWithStat(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(ui.RepoStatMsg{SessionID: "s1", Stat: review.Stat{Branch: "main"}})
	if head := headerOf(m); !strings.Contains(head, "main · main") {
		t.Errorf("header %q does not read <title> · <branch> for s1 (titled main on branch main)", head)
	}
}
