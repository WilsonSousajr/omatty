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

// ListSessions renders adoptable sessions as numbered lines for `omatty adopt`,
// newest first, each with its title and when it was last used (#122).
//
//	for _, line := range discover.ListSessions(cands, time.Now()) { report(line) }
//
// The title rather than the id leads, because a uuid tells the operator nothing
// about which session it is; the id is shown too, since it is what `--resume`
// takes and what appears in an error.
func ListSessions(cands []SessionCandidate, now time.Time) []string {
	lines := make([]string, 0, len(cands))
	for i, c := range cands {
		lines = append(lines, fmt.Sprintf("%2d  %-40s %s  (%s)",
			i+1, c.Title, shortID(c.ID), ago(now, c.LastUsed)))
	}
	return lines
}

// shortID is the leading block of a uuid, which is enough to tell two sessions
// apart in a list without spending forty columns on one.
func shortID(id string) string {
	if i := strings.IndexByte(id, '-'); i > 0 {
		return id[:i]
	}
	return id
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
	return pick(cands, selection)
}

// ChooseSessions is Choose over adoptable sessions, for `omatty adopt` (#122).
// The grammar is identical on purpose: an operator who has learnt one list has
// learnt the other.
//
//	picked, err := discover.ChooseSessions(cands, "1 3")
func ChooseSessions(cands []SessionCandidate, selection string) ([]SessionCandidate, error) {
	return pick(cands, selection)
}

// pick resolves a typed selection into the entries it names.
//
// Generic over the candidate type so the grammar exists once. The two callers
// differ only in what a row is, and a policy written twice drifts - the reason
// RegisterAll exists rather than a loop in cmd and another in ui (#91, #122).
func pick[T any](cands []T, selection string) ([]T, error) {
	indices, all, err := parseSelection(selection, len(cands))
	if err != nil {
		return nil, err
	}
	if all {
		return cands, nil
	}
	picked := make([]T, 0, len(indices))
	for _, i := range indices {
		picked = append(picked, cands[i])
	}
	return picked, nil
}

// parseSelection turns "1 3", "1,3" or "all" into zero-based indices.
//
// It returns nothing for an empty selection, which is how you back out:
// discovery and adoption both propose, and nothing is registered without being
// asked for (invariant 9).
//
// One bad field rejects the whole selection rather than registering the good
// half: a partial answer to "which of these" is not an answer.
func parseSelection(selection string, n int) (indices []int, all bool, err error) {
	// Any whitespace, not just the space bar: a tab-separated answer, or one
	// pasted back out of the rendered table, parsed as a single field and was
	// rejected as "not a number" - and the error exits, discarding the scan
	// that produced the list (#91).
	fields := strings.FieldsFunc(selection, func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
	if len(fields) == 0 {
		return nil, false, nil
	}
	if len(fields) == 1 && strings.EqualFold(fields[0], "all") {
		return nil, true, nil
	}
	for _, f := range fields {
		i, err := indexOf(f, n)
		if err != nil {
			return nil, false, err
		}
		indices = append(indices, i)
	}
	return indices, false, nil
}

// indexOf resolves one 1-based entry from the printed list.
func indexOf(field string, n int) (int, error) {
	i, err := strconv.Atoi(field)
	if err != nil {
		return 0, fmt.Errorf(
			"discover: %q is not a number; want numbers from the list, or `all`", field)
	}
	if i < 1 || i > n {
		return 0, fmt.Errorf(
			"discover: %d is out of range; the list has %d entries", i, n)
	}
	return i - 1, nil
}
