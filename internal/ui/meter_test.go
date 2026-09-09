package ui_test

import (
	"regexp"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
	"github.com/mattn/go-runewidth"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// stripSGR drops colour sequences so a test can compare the cells themselves.
func stripSGR(s string) string { return sgr.ReplaceAllString(s, "") }

// The meter is cache-read over everything claude was fed; a cache write is
// fresh input that also primed the cache, so it counts against the share.
func TestRenderMeter_Table_issue153(t *testing.T) {
	for _, tt := range []struct {
		tokens watcher.Tokens
		want   string // filled cells, then empty
	}{
		{watcher.Tokens{In: 100}, "▱▱▱▱▱▱▱▱"},
		{watcher.Tokens{CacheRead: 100}, "▰▰▰▰▰▰▰▰"},
		{watcher.Tokens{In: 20, CacheRead: 80}, "▰▰▰▰▰▰▱▱"},
		{watcher.Tokens{In: 10, CacheWrite: 10, CacheRead: 80}, "▰▰▰▰▰▰▱▱"},
		{watcher.Tokens{In: 50, CacheRead: 50}, "▰▰▰▰▱▱▱▱"},
	} {
		got := ui.RenderMeter(tt.tokens)
		if plain := stripSGR(got); plain != tt.want {
			t.Errorf("RenderMeter(%+v) = %q, want %q", tt.tokens, plain, tt.want)
		}
		if lipgloss.Width(got) != ui.MeterCells() {
			t.Errorf("RenderMeter(%+v) is %d cells wide, want %d", tt.tokens, lipgloss.Width(got), ui.MeterCells())
		}
	}
}

// No input at all draws no meter: a session that has said nothing is not 0%
// cached, it is nothing yet.
func TestRenderMeter_NoInputDrawsNothing_issue153(t *testing.T) {
	if got := ui.RenderMeter(watcher.Tokens{Out: 5}); got != "" {
		t.Errorf("RenderMeter with no input = %q, want \"\"", got)
	}
}

// Both glyphs must be one cell by both measures, as the lane's are (#128).
func TestMeterGlyphs_AreOneCellWide_issue153(t *testing.T) {
	for _, g := range ui.MeterGlyphs() {
		if lipgloss.Width(g) != 1 || runewidth.StringWidth(g) != 1 {
			t.Errorf("meter glyph %q is %d/%d cells wide, want 1", g, lipgloss.Width(g), runewidth.StringWidth(g))
		}
	}
}

func TestModel_TheRuleCarriesTheCacheMeter_issue153(t *testing.T) {
	m, _, _ := modelWithEvents(t)

	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.UsageUpdated, At: fixedNow,
		Tokens: watcher.Tokens{In: 2000, CacheRead: 8000, Out: 500}})

	lines := strings.Split(m.View().Content, "\n")
	rule := stripSGR(lines[0])
	if !strings.Contains(rule, "▰▰▰▰▰▰▱▱") || !strings.Contains(rule, "80% cached") {
		t.Errorf("the rule does not carry the meter and its share: %q", rule)
	}
	if !strings.Contains(rule, "2.0k in / 500 out") {
		t.Errorf("the rule lost the counts: %q", rule)
	}
	if card := strings.Join(m.CardOf("s1"), ""); strings.ContainsAny(card, "▰▱") {
		t.Errorf("the sidebar card grew a meter: %q", card)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 100 {
			t.Errorf("line %d is %d cells, want 100: %q", i, w, line)
		}
	}
}
