// One vocabulary for state (#425).
//
// A verdict was a glyph in three places that each picked their own: the gate
// face and the card's gate strip shared verdictMark, a pull request's CI had
// ciMark, and neither was coloured on the gate face even though the palette
// already said what green, red and amber mean. Every state omatty draws as a
// mark is now one row here - a glyph per glyph set, and one colour - so the
// same state looks the same wherever it appears.

package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/gate"
)

// markState is a state omatty draws as a one-cell mark.
type markState int

const (
	markPass markState = iota
	markFail
	markMissing
	markRunning
	markPending
	markCancelled
	markConflict // a pull request that cannot merge as it stands
)

// glyphSet is one glyph per markState. The two sets are fixed tables, chosen
// once when the model is built; nothing writes to either afterwards.
type glyphSet map[markState]string

// plainGlyphs are what every terminal font has, and the default. Pending and
// cancelled share the dot: both are "never ran", which is all the mark says.
//
// Missing is deliberately not the fail glyph. A tool that is not installed is
// a statement about the machine, not about the code (invariant 12), and a card
// that said otherwise would send someone to fix code that was never broken.
// (This was verdictMark's rule, the card strip's table before #425.)
var plainGlyphs = glyphSet{
	markPass: "✓", markFail: "✗", markMissing: "?", markRunning: "◍",
	markPending: "·", markCancelled: "·", markConflict: "⚠",
}

// nerdGlyphs are Font Awesome's icons as every Nerd Font patches them, for
// [ui] icons = "nerd". Opt-in because without the font each is a tofu box.
var nerdGlyphs = glyphSet{
	markPass: "", markFail: "", markMissing: "", markRunning: "",
	markPending: "", markCancelled: "", markConflict: "",
}

// markColors is the one place a mark becomes a colour, on the palette's rule
// of one hue, one meaning (#175): green passed, red failed, amber is the
// operator's move - a tool to install, a conflict to resolve - and never an
// error, because a missing tool says nothing about the code (invariant 12).
var markColors = map[markState]color.Color{
	markPass: colorGreen, markFail: colorRed, markMissing: colorAmber, markConflict: colorAmber,
	markRunning: colorText, markPending: colorMuted, markCancelled: colorMuted,
}

// glyphsFor is the set the config asked for.
func glyphsFor(nerd bool) glyphSet {
	if nerd {
		return nerdGlyphs
	}
	return plainGlyphs
}

// mark is s's glyph, uncoloured: for a row the cursor draws in reverse, where
// a colour run inside would end the reverse early, and for the card, which M8
// keeps uncoloured.
func (g glyphSet) mark(s markState) string { return g[s] }

// cell is s's glyph in its colour.
func (g glyphSet) cell(s markState) string {
	return lipgloss.NewStyle().Foreground(markColors[s]).Render(g[s])
}

// verdictState is the mark a gate step's verdict draws as.
func verdictState(v gate.Verdict) markState {
	switch v {
	case gate.Pass:
		return markPass
	case gate.Fail:
		return markFail
	case gate.Missing:
		return markMissing
	case gate.Running:
		return markRunning
	case gate.Cancelled:
		return markCancelled
	}
	return markPending
}

// ciState is a pull request's CI as one mark, by precedence: failing, then a
// conflict or a branch behind, then running, then passing; false when the pull
// request has no checks at all.
func ciState(pr forge.PR) (markState, bool) {
	switch {
	case pr.CI == forge.CIFailing:
		return markFail, true
	case pr.Conflict:
		return markConflict, true
	case pr.CI == forge.CIRunning:
		return markRunning, true
	case pr.CI == forge.CIPassing:
		return markPass, true
	}
	return 0, false
}
