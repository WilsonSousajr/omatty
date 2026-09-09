package ui

import (
	"math"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
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

// meterFull and meterEmpty are the cells: two glyphs, the filled ones
// warming from amber to green across the bar (#154), the empty ones muted.
const meterFull, meterEmpty = "▰", "▱"

// inputTotal is everything the prompt was fed: fresh input plus both cache
// halves. It is not t.In, because claude's transcript reports input_tokens as
// only the uncached remainder - at a 95% hit rate a whole session's In is a
// couple hundred tokens, and the rule read "154 in / 62.6k out" on a session
// that had sent 60k (#170). Output is not in it: this is the size of what went
// up, and the meter beside it says how much of that was cheap.
func inputTotal(t watcher.Tokens) int { return t.In + t.CacheRead + t.CacheWrite }

// cacheShare is CacheRead over everything the prompt was fed. A cache write is
// fresh input that also primed the cache, so it counts against the share, and
// output is not in it at all: the meter is about what a turn cost, not what it
// produced. ok is false with no input yet, so a session that has said nothing
// draws no meter rather than 0%.
func cacheShare(t watcher.Tokens) (float64, bool) {
	fed := inputTotal(t)
	if fed == 0 {
		return 0, false
	}
	return float64(t.CacheRead) / float64(fed), true
}

// renderMeter draws share as meterCells one-cell blocks, filled from the
// left, each filled cell its own colour on the ramp.
func renderMeter(share float64) string {
	filled := int(math.Round(share * meterCells))
	var b strings.Builder
	for i := range filled {
		b.WriteString(lipgloss.NewStyle().Foreground(meterCellColor(i)).Render(meterFull))
	}
	b.WriteString(mutedStyle.Render(strings.Repeat(meterEmpty, meterCells-filled)))
	return b.String()
}

// tokensPart is the usage segment whole: the meter and its percentage when
// there is input to measure, then the in/out counts (#39, #153). The header
// row takes the two halves separately so it can drop them one at a time
// (#177); this joins them for a reader that wants the whole.
func tokensPart(t watcher.Tokens) string {
	return dots(meterPart(t), countsPart(t))
}

// meterPart is the bar and its percentage, "" with no input to measure.
func meterPart(t watcher.Tokens) string {
	share, ok := cacheShare(t)
	if !ok {
		return ""
	}
	return renderMeter(share) + " " + mutedStyle.Render(strconv.Itoa(int(math.Round(share*100)))+"% cached")
}

// countsPart is the in/out counts, "in" being everything fed (#170), and ""
// for a session that has reported no tokens at all.
func countsPart(t watcher.Tokens) string {
	if t == (watcher.Tokens{}) {
		return ""
	}
	return mutedStyle.Render(KString(inputTotal(t)) + " in / " + KString(t.Out) + " out")
}
