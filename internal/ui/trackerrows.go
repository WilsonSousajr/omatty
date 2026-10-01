package ui

import (
	"strconv"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
	"github.com/WilsonSousajr/omatty/internal/domain/fuzzy"
)

// trackerRows is the project's open issues, then its open pull requests under a
// labelled rule. One flat slice, so the cursor and the window index the rows a
// person sees rather than two lists and an offset between them.
func (m *Model) trackerRows() []trackerRow {
	project := m.review.Tracker.Project
	rows := make([]trackerRow, 0, len(m.issues[project]))
	for _, is := range m.issues[project] {
		row := trackerRow{
			Kind: rowIssue, Number: is.Number, Label: firstLabel(is.Labels),
			Title: is.Title, Updated: is.Updated,
		}
		if m.matchesFilter(row, is.Labels) {
			rows = append(rows, row)
		}
	}
	prs := m.openPRs(project)
	if len(prs) == 0 {
		return rows
	}
	return append(m.withRules(project, rows), prs...)
}

// withRules heads the issues and the pull requests that follow them. The rule
// goes with its list: one with nothing under it says a list is there when it is
// not (#399). With both lists on screen each is named (#432); a lone list needs
// no name - unless its forge keeps no issues, which it says over the list it
// does keep, where the issues would have been (#460).
func (m *Model) withRules(project string, issues []trackerRow) []trackerRow {
	if len(issues) > 0 {
		issues = append([]trackerRow{{Kind: rowRule, Title: "issues"}}, issues...)
		return append(issues, trackerRow{Kind: rowRule, Title: m.label(project).Change + "s"})
	}
	if m.noTracker[project] {
		return []trackerRow{{Kind: rowRule, Title: m.label(project).Short + "s · issues elsewhere"}}
	}
	return issues
}

// matchesFilter reports whether a row survives the filter line. The haystack is
// what the row shows - its number, its title and its labels - because a filter
// over text the operator cannot see is a filter they cannot predict. All of the
// labels, not just the drawn one: `M14` is how a milestone is asked for.
func (m *Model) matchesFilter(row trackerRow, labels []string) bool {
	query := m.review.Filter.Query
	if query == "" || m.review.View == ViewDiff {
		return true
	}
	hay := strconv.Itoa(row.Number) + " " + row.Title + " " + strings.Join(labels, " ")
	_, ok := fuzzy.Match(query, hay)
	return ok
}

// openPRs is the project's open pull requests as rows, narrowed by the filter.
// Only the open ones: the map holds the finished for the card that says "merged"
// (#310), and the tracker answers what is open.
func (m *Model) openPRs(project string) []trackerRow {
	var rows []trackerRow
	for _, pr := range m.prs[project] {
		if pr.State != forge.Open {
			continue
		}
		row := trackerRow{
			Kind: rowPR, Number: pr.Number, Label: m.prMark(pr),
			Title: pr.Title, Updated: pr.Updated, Sigil: m.label(project).Sigil,
		}
		if m.matchesFilter(row, nil) {
			rows = append(rows, row)
		}
	}
	return rows
}

// prMark is a pull request's three cells in the label column: whether it is a
// draft, its CI, its review - one column each, a blank where there is nothing
// to say, so the three read as columns down the list (#432, gh-dash).
func (m *Model) prMark(pr forge.PR) string {
	draft := " "
	if pr.Draft {
		draft = m.glyphs.mark(markDraft)
	}
	ci := m.ciMark(pr)
	if ci == "" {
		ci = " "
	}
	review := " "
	if s, ok := reviewState(pr.Review); ok {
		review = m.glyphs.mark(s)
	}
	return draft + ci + review
}

func firstLabel(labels []string) string {
	if len(labels) == 0 {
		return ""
	}
	return labels[0]
}
