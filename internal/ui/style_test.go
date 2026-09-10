package ui_test

import (
	"slices"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// One hue, one meaning (#175): amber is bound to waiting and to nothing
// else, and the accent means focus alone, so it appears in no status.
func TestStatusColors_AmberMeansWaitingAloneAndAccentMeansFocusAlone_issue175(t *testing.T) {
	for _, s := range ui.AllStatuses() {
		isAmber := sameRGB(ui.StatusColor(s), ui.AmberColor())
		if isAmber != (s == watcher.StatusWaiting) {
			t.Errorf("status %s amber=%v; amber must mean waiting and only waiting", s, isAmber)
		}
		if sameRGB(ui.StatusColor(s), ui.AccentColor()) {
			t.Errorf("status %s is drawn in the accent, which means focus alone", s)
		}
	}
}

// Working states earn no colour: the lane's height already says how busy.
func TestStatusColors_WorkingStatesAreTextColoured_issue175(t *testing.T) {
	for _, s := range []watcher.Status{watcher.StatusThinking, watcher.StatusTool} {
		if !sameRGB(ui.StatusColor(s), ui.TextColor()) {
			t.Errorf("status %s = %v, want the text colour", s, ui.StatusColor(s))
		}
	}
}

// Every status has a glyph and a colour, so a new status cannot render "-".
func TestStatusTables_CoverEveryStatus_issue175(t *testing.T) {
	for _, s := range ui.AllStatuses() {
		if ui.StatusColor(s) == nil {
			t.Errorf("status %s has no colour", s)
		}
	}
	if len(ui.StatusGlyphs()) != len(ui.AllStatuses()) {
		t.Errorf("%d glyphs for %d statuses", len(ui.StatusGlyphs()), len(ui.AllStatuses()))
	}
}

// Measured with both libraries omatty draws through: lipgloss.Width is what
// the renderer pads with, runewidth what discover.truncate cuts with. A glyph
// that is two cells in either misaligns every sidebar row (#128).
func TestStatusGlyphs_AreOneCellWide_issue128(t *testing.T) {
	for _, g := range ui.StatusGlyphs() {
		if lipgloss.Width(g) != 1 || runewidth.StringWidth(g) != 1 {
			t.Errorf("glyph %q is %d/%d cells wide, want 1", g, lipgloss.Width(g), runewidth.StringWidth(g))
		}
	}
	if !slices.Contains(ui.StatusGlyphs(), "◐") {
		t.Error("the working glyph ◐ is missing (#175)")
	}
}

func TestLaneCells_AreOneCellWide_issue128(t *testing.T) {
	for _, c := range ui.LaneBlocks() {
		if lipgloss.Width(c) != 1 || runewidth.StringWidth(c) != 1 {
			t.Errorf("lane cell %q is %d/%d cells wide, want 1", c, lipgloss.Width(c), runewidth.StringWidth(c))
		}
	}
}
