// Footers that shorten by rank (#426).
//
// Every footer was one string cut at the window's right edge. That is how keys
// fell off the end (#103, #198) and how the tracker's footer, at 87 columns,
// lost its help key on an 80-column window without a test noticing. A footer is
// now a list of entries, each ranked: a narrow window gives up whole entries,
// lowest first, so no key is ever drawn half-cut - where "b brow" reads as a
// key that does not exist - and the help key, which reaches every key that was
// given up, is never among them.

package ui

import (
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
)

// footerKey is one footer entry and how long it survives a narrow window:
// higher survives longer, and keepKey never goes.
type footerKey struct {
	text string
	rank int
}

// keepKey is the rank of an entry no width takes away: the exit key and the
// help key, the two a person must always be able to find.
const keepKey = 100

// keySep is the gap between two entries.
const keySep = "  "

// joinKeys lays the entries out in order, the whole footer.
func joinKeys(keys []footerKey) string {
	texts := make([]string, len(keys))
	for i, k := range keys {
		texts[i] = k.text
	}
	return strings.Join(texts, keySep)
}

// fitKeys is the footer in width cells: whole entries given up, weakest first
// and the rightmost of equals first, until the rest fit. When even the kept
// entries do not fit, the caller's fitLine cuts them; nothing else can.
func fitKeys(keys []footerKey, width int) string {
	keys = slices.Clone(keys)
	for lipgloss.Width(joinKeys(keys)) > width {
		weakest := weakestKey(keys)
		if weakest < 0 {
			break
		}
		keys = slices.Delete(keys, weakest, weakest+1)
	}
	return joinKeys(keys)
}

// weakestKey is the index of the entry to give up next, or -1 when every one
// left is kept.
func weakestKey(keys []footerKey) int {
	weakest := -1
	for i, k := range keys {
		if k.rank < keepKey && (weakest < 0 || k.rank <= keys[weakest].rank) {
			weakest = i
		}
	}
	return weakest
}
