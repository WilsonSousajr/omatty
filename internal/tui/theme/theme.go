// Package theme is the TUI's one palette and every style it draws with (ADR
// 0001, "tui/theme"; migration step 6.2, #653). A colour or a style written
// anywhere else in internal/tui is a second theme waiting to drift from this
// one, so internal/tui/app builds none of its own; a test holds it to that.
//
//	line := theme.Added.Render("+ x := 1")
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/domain/status"
)

// Palette. ANSI 256 indices so it degrades sanely on 16-colour terminals.
//
// The one exception, decided on 2026-09-08 (#154): the meter's warmth is a
// truecolor blend *between* these entries (ramp.go); the lane's fade was the
// other until #410 removed the lane. Ramps were not added as more indices
// because bubbletea detects the terminal's profile and quantises every
// colour it draws, so the 256- and 16-colour fallbacks are the renderer's,
// not ours - and a test asserts the ramp still reads as one after
// quantising. Everything that is not a ramp stays an index, and there is
// still one theme: the ramp's endpoints are the palette, so nothing here is
// a second set of colours.
//
// M8 (#175) made the palette eight indices and the rule one hue, one
// meaning: working states are text, amber is waiting and nothing else,
// green is done, red is error, muted is true-but-not-actionable, and the
// accent means focus alone. 75 replaced 39 as the accent because it is the
// nearest index to monocode's blue that is not neon.
var (
	ColorInk      = lipgloss.Color("253") // the focused header segment, the selected title
	ColorText     = lipgloss.Color("250") // titles, branches, working glyphs
	ColorMuted    = lipgloss.Color("245") // headers, ages, counts, keymap
	ColorHairline = lipgloss.Color("238") // every divider that is not focused
	ColorAccent   = lipgloss.Color("75")  // the focused hairline, the rail
	ColorAmber    = lipgloss.Color("214") // waiting, and nothing else
	ColorGreen    = lipgloss.Color("78")  // done, +added
	ColorRed      = lipgloss.Color("203") // error, −removed
)

// statusColors is the only place a status becomes a colour. Working states
// are text: working is the default, and the spinner already says busy
// (#410). Amber goes to waiting alone, so an amber glyph in the sidebar
// still answers "which of these needs me" (#175).
var statusColors = map[status.Status]color.Color{
	status.StatusIdle: ColorMuted, status.StatusThinking: ColorText, status.StatusTool: ColorText,
	status.StatusWaiting: ColorAmber, status.StatusDone: ColorGreen, status.StatusError: ColorRed,
	status.StatusExited: ColorMuted,
}

// StatusColor is a status's palette colour, the muted grey for one without.
//
//	c := theme.StatusColor(status.StatusWaiting) // amber
func StatusColor(s status.Status) color.Color {
	if c, ok := statusColors[s]; ok {
		return c
	}
	return ColorMuted
}

// The chrome's styles.
var (
	Header  = lipgloss.NewStyle().Foreground(ColorInk).Bold(true)
	Footer  = lipgloss.NewStyle().Foreground(ColorMuted)
	Error   = lipgloss.NewStyle().Foreground(ColorRed).Bold(true)
	Muted   = lipgloss.NewStyle().Foreground(ColorMuted)
	Text    = lipgloss.NewStyle().Foreground(ColorText)
	Accent  = lipgloss.NewStyle().Foreground(ColorAccent)
	Amber   = lipgloss.NewStyle().Foreground(ColorAmber)
	Strong  = lipgloss.NewStyle().Bold(true)
	Search  = lipgloss.NewStyle().Bold(true).Underline(true)
	Divider = lipgloss.NewStyle().Foreground(ColorHairline)
)

// Diff colours: added green, removed red, comments amber, the cursor row
// reversed so it reads at a glance in any palette (#21). A queued comment is
// amber because it is the operator's own pending move, the one meaning the
// rule gives amber (#175).
var (
	Added   = lipgloss.NewStyle().Foreground(ColorGreen)
	Removed = lipgloss.NewStyle().Foreground(ColorRed)
	Comment = lipgloss.NewStyle().Foreground(ColorAmber)
	Cursor  = lipgloss.NewStyle().Reverse(true)
	// AddedEmphasis and RemovedEmphasis are a pair's changed words: the
	// line's own hue on a darker ground of the same meaning - 22 a green, 52
	// a red - so the emphasis says "this part" without saying anything new.
	AddedEmphasis   = lipgloss.NewStyle().Foreground(ColorGreen).Background(lipgloss.Color("22"))
	RemovedEmphasis = lipgloss.NewStyle().Foreground(ColorRed).Background(lipgloss.Color("52"))
)

// Plain is the style that adds nothing.
//
//	s := theme.Plain().Render(line)
func Plain() lipgloss.Style { return lipgloss.NewStyle() }

// Clip cuts what it renders to width cells.
//
//	row := theme.Clip(40).Render(line)
func Clip(width int) lipgloss.Style { return lipgloss.NewStyle().MaxWidth(width) }

// Foreground colours text c: for a colour chosen per cell, as a status glyph
// or the meter's ramp is, where a fixed style would be one per colour.
//
//	cell := theme.Foreground(theme.StatusColor(st)).Render("●")
func Foreground(c color.Color) lipgloss.Style { return lipgloss.NewStyle().Foreground(c) }
