package app_test

import (
	"slices"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"

	"github.com/WilsonSousajr/omatty/internal/service/status"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// One hue, one meaning (#175): amber is bound to waiting and to nothing
// else, and the accent means focus alone, so it appears in no status.
func TestStatusColors_AmberMeansWaitingAloneAndAccentMeansFocusAlone_issue175(t *testing.T) {
	for _, s := range app.AllStatuses() {
		isAmber := sameRGB(app.StatusColor(s), app.AmberColor())
		if isAmber != (s == status.StatusWaiting) {
			t.Errorf("status %s amber=%v; amber must mean waiting and only waiting", s, isAmber)
		}
		if sameRGB(app.StatusColor(s), app.AccentColor()) {
			t.Errorf("status %s is drawn in the accent, which means focus alone", s)
		}
	}
}

// Working states earn no colour: the spinner already says busy (#410).
func TestStatusColors_WorkingStatesAreTextColoured_issue175(t *testing.T) {
	for _, s := range []status.Status{status.StatusThinking, status.StatusTool} {
		if !sameRGB(app.StatusColor(s), app.TextColor()) {
			t.Errorf("status %s = %v, want the text colour", s, app.StatusColor(s))
		}
	}
}

// Every status has a glyph and a colour, so a new status cannot render "-".
func TestStatusTables_CoverEveryStatus_issue175(t *testing.T) {
	for _, s := range app.AllStatuses() {
		if app.StatusColor(s) == nil {
			t.Errorf("status %s has no colour", s)
		}
	}
	if len(app.StatusGlyphs()) != len(app.AllStatuses()) {
		t.Errorf("%d glyphs for %d statuses", len(app.StatusGlyphs()), len(app.AllStatuses()))
	}
}

// Measured with both libraries omatty draws through: lipgloss.Width is what
// the renderer pads with, runewidth what discover.truncate cuts with. A glyph
// that is two cells in either misaligns every sidebar row (#128).
func TestStatusGlyphs_AreOneCellWide_issue128(t *testing.T) {
	for _, g := range app.StatusGlyphs() {
		if lipgloss.Width(g) != 1 || runewidth.StringWidth(g) != 1 {
			t.Errorf("glyph %q is %d/%d cells wide, want 1", g, lipgloss.Width(g), runewidth.StringWidth(g))
		}
	}
	if !slices.Contains(app.StatusGlyphs(), "◐") {
		t.Error("the working glyph ◐ is missing (#175)")
	}
}
