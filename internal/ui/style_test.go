package ui_test

import (
	"slices"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/WilsonSousajr/omatty/internal/ui"
)

// Measured with both libraries omatty draws through: lipgloss.Width is what
// the renderer pads with, runewidth what discover.truncate cuts with. A glyph
// that is two cells in either misaligns every sidebar row (#128).
func TestStatusGlyphs_AreOneCellWide_issue128(t *testing.T) {
	for _, g := range ui.StatusGlyphs() {
		if lipgloss.Width(g) != 1 || runewidth.StringWidth(g) != 1 {
			t.Errorf("glyph %q is %d/%d cells wide, want 1", g, lipgloss.Width(g), runewidth.StringWidth(g))
		}
	}
	if !slices.Contains(ui.StatusGlyphs(), "●") {
		t.Error("the M2 glyph set is not restored")
	}
}
