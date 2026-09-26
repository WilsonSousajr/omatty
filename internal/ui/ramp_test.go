package ui_test

import (
	"image/color"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/charmbracelet/colorprofile"
)

// rgb8 is a colour as three 8-bit channels, for comparison and luminance.
func rgb8(c color.Color) (r, g, b uint8) {
	r16, g16, b16, _ := c.RGBA()
	return uint8(r16 >> 8), uint8(g16 >> 8), uint8(b16 >> 8)
}

func sameRGB(a, b color.Color) bool {
	ar, ag, ab := rgb8(a)
	br, bg, bb := rgb8(b)
	return ar == br && ag == bg && ab == bb
}

func TestBlend_EndsAreTheEndpointsAndTheMiddleIsBetween_issue154(t *testing.T) {
	from, to := color.RGBA{R: 255, A: 255}, color.RGBA{B: 255, A: 255}
	if !sameRGB(ui.Blend(from, to, 0), from) || !sameRGB(ui.Blend(from, to, 1), to) {
		t.Errorf("Blend at 0 / 1 = %v / %v, want the endpoints", ui.Blend(from, to, 0), ui.Blend(from, to, 1))
	}
	r, g, b := rgb8(ui.Blend(from, to, 0.5))
	if r < 120 || r > 135 || b < 120 || b > 135 || g != 0 {
		t.Errorf("Blend at 0.5 = (%d,%d,%d), want roughly (128,0,128)", r, g, b)
	}
}

// The meter warms from amber to green left to right, so a short bar reads
// amber and a full one ends green.
func TestMeterCellColor_RampsFromAmberToGreen_issue154(t *testing.T) {
	warm, cool := ui.MeterRamp()
	first, last := ui.MeterCellColor(0), ui.MeterCellColor(ui.MeterCells()-1)
	if !sameRGB(first, warm) || !sameRGB(last, cool) {
		t.Errorf("meter ends = %v / %v, want the ramp's amber and green", first, last)
	}
	if !sameRGB(warm, ui.AmberColor()) {
		t.Error("the meter no longer starts at amber (#175)")
	}
	for i := 1; i < ui.MeterCells(); i++ {
		if sameRGB(ui.MeterCellColor(i), ui.MeterCellColor(i-1)) {
			t.Errorf("meter cells %d and %d share a colour in truecolor", i-1, i)
		}
	}
}

// The ramp still reads as a ramp when it quantises (#154): at least three
// distinct colours across the meter's warmth on a 256-colour terminal, and
// at least two on a 16-colour one. The lane's fade was the other ramp here
// until #410 removed the lane.
func TestRamps_StillReadAfterQuantising_issue154(t *testing.T) {
	ramps := map[string][]color.Color{}
	for i := range ui.MeterCells() {
		ramps["meter"] = append(ramps["meter"], ui.MeterCellColor(i))
	}
	for name, ramp := range ramps {
		for _, tt := range []struct {
			profile colorprofile.Profile
			want    int
		}{{colorprofile.ANSI256, 3}, {colorprofile.ANSI, 2}} {
			if n := distinct(tt.profile, ramp); n < tt.want {
				t.Errorf("%s quantised to %v has %d distinct colours, want at least %d", name, tt.profile, n, tt.want)
			}
		}
	}
}

func distinct(p colorprofile.Profile, ramp []color.Color) int {
	seen := map[[3]uint8]bool{}
	for _, c := range ramp {
		r, g, b := rgb8(p.Convert(c))
		seen[[3]uint8{r, g, b}] = true
	}
	return len(seen)
}
