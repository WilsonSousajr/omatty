package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// nerdPass, nerdFail and nerdMissing are the Nerd Font glyphs #425 draws when
// [ui] icons = "nerd": Font Awesome's check, times and question, in the
// private use area every Nerd Font patches.
const (
	nerdPass    = "\uf00c"
	nerdFail    = "\uf00d"
	nerdMissing = "\uf128"
)

// A verdict is coloured on the gate face, one hue for one meaning: green
// passed, red failed, and a missing tool amber - the operator's move, never red,
// because it says nothing about the code (invariant 12's corollary).
func TestGate_VerdictsAreColoured_issue425(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", gateReport(gate.Pass, gate.Fail, gate.Missing, gate.Pending))
	leader(m, key('g'))
	press(m, key('G')) // the cursor row is drawn plain in reverse; move it off the others

	body := m.View().Content
	for _, want := range []string{ui.Added("✓"), ui.Removed("✗"), ui.Amber("?")} {
		if !strings.Contains(body, want) {
			t.Errorf("the gate face does not draw %q:\n%s", want, body)
		}
	}
}

// With icons = "nerd" every state glyph comes from the Nerd Font set at once:
// the gate face, the card's gate strip, and a pull request's CI mark.
func TestGlyphs_NerdIconsSwapEveryStateGlyph_issue425(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.UseNerdIcons()
	m.SetGateReport("s1", gateReport(gate.Pass, gate.Fail, gate.Missing))
	leader(m, key('g'))

	body := stripSGR(m.View().Content)
	for _, want := range []string{nerdPass, nerdFail, nerdMissing} {
		if !strings.Contains(body, want) {
			t.Errorf("the gate face does not draw the Nerd Font glyph %q:\n%s", want, body)
		}
	}
	if card := strings.Join(m.CardOf("s1"), "\n"); !strings.Contains(stripSGR(card), nerdPass+nerdFail+nerdMissing) {
		t.Errorf("the card's gate strip is not drawn in Nerd Font glyphs:\n%s", card)
	}
	if strings.Contains(body, "✓") || strings.Contains(body, "✗") {
		t.Errorf("a plain glyph survived the swap:\n%s", body)
	}
}

// The tracker's CI mark swaps with the rest.
func TestGlyphs_NerdIconsReachTheTrackersCIMark_issue425(t *testing.T) {
	m, _, fp := trackerModel(t)
	m.UseNerdIcons()
	fp.Lists["/p/omatty"][0].CI = forge.CIFailing
	openTracker(m)

	if body := stripSGR(m.View().Content); !strings.Contains(body, nerdFail) {
		t.Errorf("a failing pull request's CI mark is not the Nerd Font glyph:\n%s", body)
	}
}

// Deps carries the choice from the config file into the model, so the key the
// operator set is the glyph set they see.
func TestGlyphs_NerdIconsComeThroughDeps_issue425(t *testing.T) {
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.NerdIcons = true
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m.SetGateReport("s1", gateReport(gate.Pass))

	if card := stripSGR(strings.Join(m.CardOf("s1"), "\n")); !strings.Contains(card, nerdPass) {
		t.Errorf("Deps.NerdIcons did not reach the card:\n%s", card)
	}
}
