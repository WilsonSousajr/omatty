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
	// 10.0k, not the 2.0k this asserted when #153 shipped: "in" is everything
	// fed, so the 8000 served from cache is part of it (#170).
	if !strings.Contains(rule, "10.0k in / 500 out") {
		t.Errorf("the rule lost the counts: %q", rule)
	}
	if row := m.RowOf("s1"); strings.ContainsAny(row, "▰▱") {
		t.Errorf("the sidebar row grew a meter: %q", row)
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w != 100 {
			t.Errorf("line %d is %d cells, want 100: %q", i, w, line)
		}
	}
}

// "in" is everything the prompt was fed, not the uncached remainder claude's
// transcript calls input_tokens. At a 95% hit rate that field collapses to a
// couple hundred tokens, so the rule read "154 in / 62.6k out" on a session
// that had sent 60k - the opposite of what happened (#170).
func TestTokensPart_InIsEverythingFed_issue170(t *testing.T) {
	for _, tt := range []struct {
		name   string
		tokens watcher.Tokens
		want   string
	}{
		{"cache hides the input", watcher.Tokens{In: 154, CacheRead: 60_000, CacheWrite: 2_400, Out: 62_600}, "62.6k in / 62.6k out"},
		{"a cold turn is all fresh", watcher.Tokens{In: 2_000, Out: 500}, "2.0k in / 500 out"},
		{"a write counts as fed", watcher.Tokens{CacheWrite: 1_500, Out: 20}, "1.5k in / 20 out"},
	} {
		got := stripSGR(ui.TokensPart(tt.tokens))
		if !strings.Contains(got, tt.want) {
			t.Errorf("%s: TokensPart(%+v) = %q, want it to contain %q", tt.name, tt.tokens, got, tt.want)
		}
	}
}
