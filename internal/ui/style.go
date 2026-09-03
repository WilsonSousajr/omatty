package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// Palette. ANSI 256 indices so it degrades sanely on 16-colour terminals.
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
	c := colorBlurred
	if focused {
		c = colorFocused
	}
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(c)
}

// glyphStyle colours a status glyph.
func glyphStyle(s watcher.Status) lipgloss.Style {
	if c, ok := statusColors[s]; ok {
		return lipgloss.NewStyle().Foreground(c)
	}
	return lipgloss.NewStyle().Foreground(colorMuted)
}

var (
	headerStyle = lipgloss.NewStyle().Foreground(colorFocused).Bold(true)
	footerStyle = lipgloss.NewStyle().Foreground(colorFooter)
	errorStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	mutedStyle  = lipgloss.NewStyle().Foreground(colorMuted)
)

// statusGlyphs pairs each status with its one-column marker; a status not
// listed renders "-".
var statusGlyphs = map[watcher.Status]string{
	watcher.StatusThinking: "*", watcher.StatusTool: "@", watcher.StatusWaiting: "!",
	watcher.StatusDone: "+", watcher.StatusError: "x", watcher.StatusExited: "∅",
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
