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
//
// M8 (#175) made the palette eight indices and the rule one hue, one
// meaning: working states are text, amber is waiting and nothing else,
// green is done, red is error, muted is true-but-not-actionable, and the
// accent means focus alone. 75 replaced 39 as the accent because it is the
// nearest index to monocode's blue that is not neon.
var (
	colorInk      = lipgloss.Color("253") // the focused header segment, the selected title
	colorText     = lipgloss.Color("250") // titles, branches, working glyphs
	colorMuted    = lipgloss.Color("245") // headers, ages, counts, keymap
	colorHairline = lipgloss.Color("238") // every divider that is not focused
	colorAccent   = lipgloss.Color("75")  // the focused hairline, the rail
	colorAmber    = lipgloss.Color("214") // waiting, and nothing else
	colorGreen    = lipgloss.Color("78")  // done, +added
	colorRed      = lipgloss.Color("203") // error, −removed
)

// statusColors is the only place a status becomes a colour. Working states
// are text: working is the default, and the lane's height already says how
// busy. Amber goes to waiting alone, so the newest amber cell in any lane
// still answers "which of these needs me" (#175).
var statusColors = map[watcher.Status]color.Color{
	watcher.StatusIdle: colorMuted, watcher.StatusThinking: colorText, watcher.StatusTool: colorText,
	watcher.StatusWaiting: colorAmber, watcher.StatusDone: colorGreen, watcher.StatusError: colorRed,
	watcher.StatusExited: colorMuted,
}

// glyphStyle colours a status glyph.
func glyphStyle(s watcher.Status) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(glyphColor(s))
}

var (
	headerStyle = lipgloss.NewStyle().Foreground(colorInk).Bold(true)
	footerStyle = lipgloss.NewStyle().Foreground(colorMuted)
	errorStyle  = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	mutedStyle  = lipgloss.NewStyle().Foreground(colorMuted)
	textStyle   = lipgloss.NewStyle().Foreground(colorText)
	accentStyle = lipgloss.NewStyle().Foreground(colorAccent)
)

// statusGlyphs pairs each status with its one-column marker (#175). The
// filled dot is waiting, the loudest glyph for the loudest state; the shapes
// differ enough to read with colour off. All are Geometric Shapes or
// Mathematical Operators, so no font draws them two cells wide - the gear and
// the pause sign #128 warned about are gone. ○ ◐ ◆ ● are East Asian
// Ambiguous like the lane cells and the rail: RUNEWIDTH_EASTASIAN=1 doubles
// all of them or none. A status not listed renders "-".
var statusGlyphs = map[watcher.Status]string{
	watcher.StatusIdle: "○", watcher.StatusThinking: "◐", watcher.StatusTool: "◆",
	watcher.StatusWaiting: "●", watcher.StatusDone: "✓", watcher.StatusError: "✕",
	watcher.StatusExited: "∅",
}

func statusGlyph(s watcher.Status) string {
	if g, ok := statusGlyphs[s]; ok {
		return g
	}
	return "-"
}

// Diff colours: added green, removed red, comments amber, the cursor row
// reversed so it reads at a glance in any palette (#21). A queued comment is
// amber because it is the operator's own pending move, the one meaning the
// rule gives amber (#175).
var (
	addedStyle   = lipgloss.NewStyle().Foreground(colorGreen)
	removedStyle = lipgloss.NewStyle().Foreground(colorRed)
	commentStyle = lipgloss.NewStyle().Foreground(colorAmber)
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
