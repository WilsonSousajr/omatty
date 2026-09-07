package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// The pane box. btop draws a panel's name in its top rule rather than on a
// row below it; so does omatty now, which is a row of content back in every
// pane at once - aesthetic and denser being the same change here (#128).
// lipgloss v2 has no border title, so the rule is built here and the style's
// own top border is turned off. Its width has to be exact: fitBlock's promise
// is that no line of the frame exceeds the window (#35).

// ruleChrome is what topRule spends on anything but the title: two corners,
// a lead-in dash, a space each side of the title, one closing dash.
const ruleChrome = 2 + 1 + 2 + 1

// topRule is the box's first line:
//
//	╭─ parser-fix · ● thinking 4m ───────────╮
//
// w is the box's CONTENT width, so the rule is w+2 cells - what lipgloss
// draws for the other three sides. A title too long for the rule is cut
// rather than pushing the corner off the window.
func topRule(focused bool, title string, w int) string {
	rule := borderStyle(focused)
	if title == "" || w < ruleChrome {
		return rule.Render("╭" + strings.Repeat("─", w) + "╮")
	}
	title = clip(title, w+2-ruleChrome)
	fill := w + 2 - ruleChrome - lipgloss.Width(title)
	return rule.Render("╭─ ") + headerStyle.Render(title) + rule.Render(" "+strings.Repeat("─", fill+1)+"╮")
}

// titledBox renders body inside a box whose top rule carries title.
func titledBox(focused bool, title string, w int, body string) string {
	box := paneBox(focused).BorderTop(false)
	return topRule(focused, title, w) + "\n" + box.Render(body)
}

// clip cuts s to width without padding it. fitLine cannot do this job: it
// pads, and a padded title would fill the rule with spaces.
func clip(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}
