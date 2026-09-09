package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// Palette. ANSI 256 indices so it degrades sanely on 16-colour terminals.
//
// The one exception, decided on 2026-09-08 (#154): the lane's fade and the
// meter's warmth are truecolor blends *between* these entries (ramp.go).
// They were not added as more indices because bubbletea detects the
// terminal's profile and quantises every colour it draws, so the 256- and
// 16-colour fallbacks are the renderer's, not ours - and a test asserts each
// ramp still reads as one after quantising. Everything that is not a ramp
// stays an index, and there is still one theme: the ramps' endpoints are the
// palette, so nothing here is a second set of colours.
var (
	colorFocused = lipgloss.Color("39")  // blue
	colorBlurred = lipgloss.Color("240") // grey
	colorMuted   = lipgloss.Color("245")
	colorFooter  = lipgloss.Color("245")
)

// statusColors gives each status a colour; the glyph alone is hard to scan.
var statusColors = map[watcher.Status]color.Color{
	watcher.StatusThinking: lipgloss.Color("214"), // amber
	watcher.StatusTool:     lipgloss.Color("39"),  // blue
	watcher.StatusWaiting:  lipgloss.Color("203"), // red - needs you
	watcher.StatusDone:     lipgloss.Color("78"),  // green
	watcher.StatusError:    lipgloss.Color("196"),
	watcher.StatusExited:   lipgloss.Color("240"),
}

// paneBox draws a rounded border; focused panes are coloured, others grey.
func paneBox(focused bool) lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(borderColor(focused))
}

// borderColor is the pane border's colour: focused or blurred.
func borderColor(focused bool) color.Color {
	if focused {
		return colorFocused
	}
	return colorBlurred
}

// borderStyle draws the rule a pane box's title sits in, in the border's
// own colour, so the top of the box matches its other three sides (#128).
func borderStyle(focused bool) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(borderColor(focused))
}

// glyphStyle colours a status glyph.
func glyphStyle(s watcher.Status) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(glyphColor(s))
}

var (
	headerStyle = lipgloss.NewStyle().Foreground(colorFocused).Bold(true)
	footerStyle = lipgloss.NewStyle().Foreground(colorFooter)
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	mutedStyle  = lipgloss.NewStyle().Foreground(colorMuted)
)

// statusGlyphs pairs each status with its one-column marker - the set the
// roadmap's M2 approved, restored by #128; a status not listed renders "-".
// ● is East Asian Ambiguous, like the rounded border and the lane cells: a
// terminal set to RUNEWIDTH_EASTASIAN=1 doubles all of them or none. ⚙ and
// ⏸ are pictographic; if a font draws them two cells wide, ⋯ and ▮ are the
// neutral fallbacks.
var statusGlyphs = map[watcher.Status]string{
	watcher.StatusThinking: "●", watcher.StatusTool: "⚙", watcher.StatusWaiting: "⏸",
	watcher.StatusDone: "✓", watcher.StatusError: "✗", watcher.StatusExited: "∅",
}

func statusGlyph(s watcher.Status) string {
	if g, ok := statusGlyphs[s]; ok {
		return g
	}
	return "-"
}

// Diff colours: added green, removed red, comments amber, the cursor row
// reversed so it reads at a glance in any palette (#21).
var (
	addedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	removedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	commentStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	cursorStyle  = lipgloss.NewStyle().Reverse(true)
)

// entryStyle colours a review row by what it is; a line row is coloured by
// what the diff did to it.
func entryStyle(e review.Entry, d review.Diff) lipgloss.Style {
	switch e.Kind {
	case review.EntryFile:
		return headerStyle
	case review.EntryHunk:
		return mutedStyle
	case review.EntryComment, review.EntryOrphan:
		return commentStyle
	}
	return lineStyle(d.LineAt(e.Pos).Kind)
}

func lineStyle(k review.LineKind) lipgloss.Style {
	switch k {
	case review.LineAdded:
		return addedStyle
	case review.LineRemoved:
		return removedStyle
	}
	return lipgloss.NewStyle()
}
