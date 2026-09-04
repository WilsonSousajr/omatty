// Presenting candidates and picking from them. Both are pure, and both live
// here rather than in cmd/omatty so the subcommand stays a construction plus
// one typed call (invariant 10).

package discover

import (
	"fmt"
	"strconv"
	"strings"
	"time"
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
func ago(now, then time.Time) string {
	days := int(now.Sub(then).Hours() / 24)
	switch {
	case days <= 0:
		return "today"
	case days == 1:
		return "yesterday"
	case days < 30:
		return strconv.Itoa(days) + " days ago"
	}
	return strconv.Itoa(days/30) + " months ago"
}

// Choose resolves a selection typed by the operator - "1 3", "1,3", or "all" -
// into the candidates it names. An empty selection chooses nothing, which is
// how you back out: discovery proposes, and nothing is registered without
// being asked for (invariant 9).
//
//	picked, err := discover.Choose(cands, "1 3")
func Choose(cands []Candidate, selection string) ([]Candidate, error) {
	fields := strings.FieldsFunc(selection, func(r rune) bool { return r == ',' || r == ' ' })
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
