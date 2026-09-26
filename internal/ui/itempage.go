// A tracker item that reads like a page (#433): a bold title, a muted by-line,
// a pull request's checks in the gate's row style, and light markdown.
//
// Light, and deliberately not a renderer. M5 cut a markdown renderer for the
// preview and the argument holds: these are a handful of line rules on text
// that is already wrapped, and on text #483 has already made plain, so no
// styling here can carry an author's escape sequence to the terminal.

package ui

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// strongStyle is a body's **strong** run: bold, and no hue - the palette's
// colours each already mean something (#175).
var strongStyle = lipgloss.NewStyle().Bold(true)

// strongRun is **text**, the one inline mark worth drawing.
var strongRun = regexp.MustCompile(`\*\*([^*]+)\*\*`)

// styleLines applies style to every line.
func styleLines(lines []string, style func(...string) string) []string {
	for i, l := range lines {
		lines[i] = style(l)
	}
	return lines
}

// markdownLines is a body as the item draws it: each line wrapped to w, a
// fenced block muted with its fences dropped, a heading bold without its #s, a
// bullet drawn as one, and **strong** runs bold.
func markdownLines(body string, w int) []string {
	var out []string
	fenced := false
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			fenced = !fenced
			continue
		}
		style, text := blockStyle(line, fenced)
		out = append(out, styleLines(wrapBlock(text, w), style)...)
	}
	return out
}

// blockStyle is how one body line is drawn, and its text with any marker it
// replaces: code is muted and untouched, a heading bold without its #s, a
// bullet's - or * a •.
func blockStyle(line string, fenced bool) (func(...string) string, string) {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]
	switch {
	case fenced:
		return mutedStyle.Render, line
	case strings.HasPrefix(trimmed, "#"):
		return headerStyle.Render, strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
	case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
		return strongRuns, indent + "• " + trimmed[2:]
	}
	return strongRuns, line
}

// strongRuns draws a line's **strong** runs bold and the rest as it is.
func strongRuns(parts ...string) string {
	return strongRun.ReplaceAllStringFunc(strings.Join(parts, ""), func(run string) string {
		return strongStyle.Render(strings.Trim(run, "*"))
	})
}

// itemChecks is a pull request's checks under a count, failing first then
// running then passing, each in the gate's row shape: mark, name, duration
// right-aligned (#425, #428). Nothing for an issue, or a PR with no checks.
func (m *Model) itemChecks(item forge.Detail) []string {
	if len(item.Checks) == 0 {
		return nil
	}
	checks := append([]forge.Check(nil), item.Checks...)
	sort.SliceStable(checks, func(i, j int) bool { return checkRank(checks[i].State) < checkRank(checks[j].State) })
	nameW := 0
	for _, c := range checks {
		nameW = max(nameW, lipgloss.Width(c.Name))
	}
	lines := []string{mutedStyle.Render("checks · " + checkCounts(checks))}
	for _, c := range checks {
		s := checkMark(c.State)
		lines = append(lines, m.glyphs.cell(s)+" "+padRight(c.Name, nameW)+" "+padLeft(durationText(c.Took), 6))
	}
	return append(lines, "")
}

// checkRank orders checks the way a reader wants them: what broke first.
func checkRank(s forge.CIState) int {
	switch s {
	case forge.CIFailing:
		return 0
	case forge.CIRunning:
		return 1
	}
	return 2
}

// checkMark is a check's state as the one vocabulary's mark.
func checkMark(s forge.CIState) markState {
	switch s {
	case forge.CIFailing:
		return markFail
	case forge.CIRunning:
		return markRunning
	}
	return markPass
}

// checkCounts is "1 failing · 6 passed · 1 running", each part only when it is
// not zero.
func checkCounts(checks []forge.Check) string {
	counts := map[forge.CIState]int{}
	for _, c := range checks {
		counts[c.State]++
	}
	var parts []string
	for _, p := range []struct {
		s    forge.CIState
		word string
	}{{forge.CIFailing, "failing"}, {forge.CIPassing, "passed"}, {forge.CIRunning, "running"}} {
		if n := counts[p.s]; n > 0 {
			parts = append(parts, strconv.Itoa(n)+" "+p.word)
		}
	}
	return strings.Join(parts, " · ")
}
