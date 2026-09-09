package ui

import (
	"image/color"

	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// The two ramps (#154): the lane fades its older cells toward grey, so
// colour says how long ago as height says what; the meter warms from amber
// to green left to right, so a short bar reads amber and a full one ends
// green. Both are truecolor blends between palette entries. bubbletea
// detects the terminal's profile at startup and its renderer converts every
// colour to it, so a 256- or 16-colour terminal sees the same ramp quantised
// - and the test that quantises them is what keeps each one legible there.

// blend is the colour t of the way from a to b, channel by channel in sRGB.
// Straight sRGB rather than a perceptual space: the endpoints are palette
// entries a few hundred hue-degrees apart at most, and the step count is six
// or eight, so a fancier space would change nothing a terminal can show.
func blend(a, b color.Color, t float64) color.Color {
	ar, ag, ab, _ := a.RGBA()
	br, bg, bb, _ := b.RGBA()
	mix := func(x, y uint32) uint8 {
		return uint8((float64(x>>8)*(1-t) + float64(y>>8)*t) + 0.5)
	}
	return color.RGBA{R: mix(ar, br), G: mix(ag, bg), B: mix(ab, bb), A: 0xff}
}

// laneFadeMax is how far the oldest lane cell goes toward the muted grey.
// Short of all the way: a cell that reached grey would read as exited.
const laneFadeMax = 0.7

// laneCellColor is a lane cell's colour for its status and age, age 0 being
// the newest. The newest cell is the status colour untouched - it is what
// answers "which of these needs me now", so it never dims; each older cell
// steps further toward grey.
func laneCellColor(s watcher.Status, age int) color.Color {
	base := glyphColor(s)
	if age <= 0 {
		return base
	}
	t := float64(age) / float64(laneCells-1) * laneFadeMax
	return blend(base, colorMuted, t)
}

// laneCellStyle colours one lane cell.
func laneCellStyle(s watcher.Status, age int) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(laneCellColor(s, age))
}

// rampWarm and rampCool are the meter's two ends. Named rather than looked up
// through the status map, so changing what a status means cannot recolour the
// meter (#175).
var rampWarm, rampCool = colorAmber, colorGreen

// meterCellColor is the colour of the i-th filled meter cell, warming from
// amber to green across the bar.
func meterCellColor(i int) color.Color {
	t := float64(i) / float64(meterCells-1)
	return blend(rampWarm, rampCool, t)
}

// glyphColor is a status's palette colour, the muted grey for one without.
func glyphColor(s watcher.Status) color.Color {
	if c, ok := statusColors[s]; ok {
		return c
	}
	return colorMuted
}
