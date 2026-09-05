// Package fuzzy ranks strings against a subsequence query, for omatty's
// session switcher (#42) and project picker (#91).
//
// It is a package of its own rather than a file in internal/ui because it is
// pure: no bubbletea, no model state, nothing to fake. That makes it cheap to
// cover with table tests instead of driving key sequences through the model.
package fuzzy

import (
	"sort"
	"strings"
	"unicode"
)

// boundaryBonus is what a match at the start of a word is worth, in the same
// units as the gap it is subtracted from. Three is enough for "psf" to prefer
// "parser-fix" over "prompts-final" without letting one lucky boundary
// outweigh a much tighter match elsewhere.
const boundaryBonus = 3

// Match reports whether every rune of query appears in s in order, ignoring
// case, and scores the match - lower is better. A run of consecutive matches
// and a match on a word boundary both score better, so initials find the thing
// they initial. An empty query matches everything at score 0.
//
// The scan is greedy: it takes the leftmost occurrence of each query rune
// rather than searching for the best alignment. Match("ab", "a-x-a-b") scores
// 2 by consuming the first 'a', where aligning on the second scores 1. Two
// candidates whose best alignments differ can therefore rank in the wrong
// order. Left as is deliberately - the exhaustive version is exponential and
// the session titles this ranks are short - but it is a limit, not a property.
//
//	score, ok := fuzzy.Match("psf", "parser-fix") // matches, and beats "prompts-final"
func Match(query, s string) (int, bool) {
	q := []rune(strings.ToLower(query))
	if len(q) == 0 {
		return 0, true
	}
	target := []rune(s)
	lower := []rune(strings.ToLower(s))
	score, next, prev := 0, 0, -1
	for i := range lower {
		if lower[i] != q[next] {
			continue
		}
		score += cost(target, i, prev)
		prev, next = i, next+1
		if next == len(q) {
			return score, true
		}
	}
	return 0, false
}

// cost charges for the distance from the previous match and refunds a match
// that starts a word. It never goes below zero, so an exact match costs
// nothing and no target can score its way past one.
func cost(target []rune, i, prev int) int {
	gap := i - prev - 1
	if boundary(target, i) {
		gap -= boundaryBonus
	}
	return max(gap, 0)
}

// boundary reports whether i starts a word: the first rune, one following a
// separator, or the capital in camelCase.
func boundary(target []rune, i int) bool {
	if i == 0 {
		return true
	}
	before := target[i-1]
	if !unicode.IsLetter(before) && !unicode.IsDigit(before) {
		return true
	}
	return unicode.IsLower(before) && unicode.IsUpper(target[i])
}

// Rank returns the indices of the items matching query, best first, breaking a
// tie on the shorter item and then on input order - so an empty query leaves
// the list exactly as it was given.
//
// The length tiebreak is what makes typing a name in full find that name.
// Match scores the run of matched runes and charges nothing for what follows,
// so "main" scores 0 against both "main" and "maintenance" and the winner was
// whichever the sidebar happened to list first. Typing a session's exact title
// and pressing enter could jump somewhere else - and the switcher's own
// fixture has two sessions titled "main", so the tie is the common case, not
// an exotic one (#42).
//
//	for _, i := range fuzzy.Rank(q, labels) { /* items[i], best first */ }
func Rank(query string, items []string) []int {
	type hit struct{ index, score, length int }
	hits := make([]hit, 0, len(items))
	for i, s := range items {
		if score, ok := Match(query, s); ok {
			hits = append(hits, hit{i, score, len([]rune(s))})
		}
	}
	// Only once something has been typed. An empty query matches everything at
	// score 0, and there the given order *is* the answer: the switcher promises
	// that an empty query shows exactly what the sidebar shows, which sorting
	// by title length would quietly break.
	byLength := query != ""
	sort.SliceStable(hits, func(a, b int) bool {
		if hits[a].score != hits[b].score {
			return hits[a].score < hits[b].score
		}
		return byLength && hits[a].length < hits[b].length
	})
	ranked := make([]int, len(hits))
	for i, h := range hits {
		ranked[i] = h.index
	}
	return ranked
}
