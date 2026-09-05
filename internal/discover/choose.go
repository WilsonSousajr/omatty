// Presenting candidates and picking from them. Both are pure, and both live
// here rather than in cmd/omatty so the subcommand stays a construction plus
// one typed call (invariant 10).

package discover

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// List renders candidates as numbered lines for `omatty discover`, newest
// first, each with when it was last used.
//
//	for _, line := range discover.List(cands, time.Now()) { report(line) }
func List(cands []Candidate, now time.Time) []string {
	lines := make([]string, 0, len(cands))
	for i, c := range cands {
		lines = append(lines, fmt.Sprintf("%2d  %-24s %s  (%s)",
			i+1, c.Name, c.Root, ago(now, c.LastUsed)))
	}
	return lines
}

// ago is a coarse "how long since", enough to tell this week from last year.
//
// Prose, where ui.AgeString is glyphs ("3d"): this renders into a CLI list an
// operator reads once, that one into a sidebar column measured in cells. Same
// question, two registers - so they stay separate rather than one of them
// learning a mode.
func ago(now, then time.Time) string {
	days := int(now.Sub(then).Hours() / 24)
	switch {
	case days <= 0:
		return "today"
	case days == 1:
		return "yesterday"
	case days < 30:
		return plural(days, "day")
	case days < 365:
		return plural(days/30, "month")
	}
	return plural(days/365, "year")
}

// plural renders the count with its unit agreeing. Without it every repository
// last used between 30 and 59 days ago read "1 months ago" - and that is the
// first bucket past the day count, so it was the common one (#91).
func plural(n int, unit string) string {
	out := strconv.Itoa(n) + " " + unit
	if n != 1 {
		out += "s"
	}
	return out + " ago"
}

// Choose resolves a selection typed by the operator - "1 3", "1,3", or "all" -
// into the candidates it names. An empty selection chooses nothing, which is
// how you back out: discovery proposes, and nothing is registered without
// being asked for (invariant 9).
//
//	picked, err := discover.Choose(cands, "1 3")
func Choose(cands []Candidate, selection string) ([]Candidate, error) {
	// Any whitespace, not just the space bar: a tab-separated answer, or one
	// pasted back out of the rendered table, parsed as a single field and was
	// rejected as "not a number" - and the error exits, discarding the scan
	// that produced the list (#91).
	fields := strings.FieldsFunc(selection, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
	if len(fields) == 0 {
		return nil, nil
	}
	if len(fields) == 1 && strings.EqualFold(fields[0], "all") {
		return cands, nil
	}
	picked := make([]Candidate, 0, len(fields))
	for _, f := range fields {
		c, err := at(cands, f)
		if err != nil {
			return nil, err
		}
		picked = append(picked, c)
	}
	return picked, nil
}

// at resolves one 1-based entry from the printed list.
func at(cands []Candidate, field string) (Candidate, error) {
	n, err := strconv.Atoi(field)
	if err != nil {
		return Candidate{}, fmt.Errorf(
			"discover: %q is not a number; want numbers from the list, or `all`", field)
	}
	if n < 1 || n > len(cands) {
		return Candidate{}, fmt.Errorf(
			"discover: %d is out of range; the list has %d entries", n, len(cands))
	}
	return cands[n-1], nil
}
