package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// Palette. ANSI 256 indices so it degrades sanely on 16-colour terminals.
var (
	colorFocused = lipgloss.Color("39")  // blue
	colorBlurred = lipgloss.Color("240") // grey
	colorMuted   = lipgloss.Color("245")
	colorFooter  = lipgloss.Color("245")
)

// statusColors gives each status a colour; the glyph alone is hard to scan.
var statusColors = map[registry.Status]color.Color{
	registry.StatusThinking: lipgloss.Color("214"), // amber
	registry.StatusTool:     lipgloss.Color("39"),  // blue
	registry.StatusWaiting:  lipgloss.Color("203"), // red - needs you
	registry.StatusDone:     lipgloss.Color("78"),  // green
	registry.StatusError:    lipgloss.Color("196"),
	registry.StatusExited:   lipgloss.Color("240"),
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
func glyphStyle(s registry.Status) lipgloss.Style {
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
var statusGlyphs = map[registry.Status]string{
	registry.StatusThinking: "*", registry.StatusTool: "@", registry.StatusWaiting: "!",
	registry.StatusDone: "+", registry.StatusError: "x", registry.StatusExited: "∅",
}

func statusGlyph(s registry.Status) string {
	if g, ok := statusGlyphs[s]; ok {
		return g
	}
	return "-"
}
