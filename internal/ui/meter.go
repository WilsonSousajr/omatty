package ui

import (
	"math"
	"strconv"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// The token meter (#153, the second slice of #128): how much of what a
// session feeds claude comes back from cache, as a bar in the focused pane's
// rule beside the counts that were already there. A ratio says something
// worth money at a glance that two numbers do not. Display-only, like the
// lane: derived from the tailer's cumulative Tokens, never persisted.

// meterCells is the bar's width. Eight: the rule has the room the sidebar
// does not (the lane went to six for its title budget, #155), and an eighth
// per cell is a step a glance can read.
const meterCells = 8

// meterFull and meterEmpty are the cells. Not gradients (#128's constraint):
// two glyphs, one colour each, from the 256-colour palette.
const meterFull, meterEmpty = "▰", "▱"

// cacheShare is CacheRead over In + CacheRead + CacheWrite - everything the
// prompt was fed. A cache write is fresh input that also primed the cache, so
// it counts against the share, and output is not in it at all: the meter is
// about what a turn cost, not what it produced. ok is false with no input
// yet, so a session that has said nothing draws no meter rather than 0%.
func cacheShare(t watcher.Tokens) (float64, bool) {
	fed := t.In + t.CacheRead + t.CacheWrite
	if fed == 0 {
		return 0, false
	}
	return float64(t.CacheRead) / float64(fed), true
}

// renderMeter draws share as meterCells one-cell blocks, filled from the left.
func renderMeter(share float64) string {
	filled := int(math.Round(share * meterCells))
	return meterStyle.Render(strings.Repeat(meterFull, filled)) +
		mutedStyle.Render(strings.Repeat(meterEmpty, meterCells-filled))
}

// tokensPart is the rule's usage segment: the meter and its percentage when
// there is input to measure, then the in/out counts the rule carried before
// (#39, #153).
func tokensPart(t watcher.Tokens) string {
	counts := KString(t.In) + " in / " + KString(t.Out) + " out"
	share, ok := cacheShare(t)
	if !ok {
		return mutedStyle.Render(counts)
	}
	pct := strconv.Itoa(int(math.Round(share*100))) + "% cached"
	return renderMeter(share) + " " + mutedStyle.Render(pct+" · "+counts)
}
