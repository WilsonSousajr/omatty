package ui_test

import (
	"image/color"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
	"github.com/charmbracelet/colorprofile"
)

// rgb8 is a colour as three 8-bit channels, for comparison and luminance.
func rgb8(c color.Color) (r, g, b uint8) {
	r16, g16, b16, _ := c.RGBA()
	return uint8(r16 >> 8), uint8(g16 >> 8), uint8(b16 >> 8)
}

// distance is how far apart two colours are in RGB, enough to say whether a
// fade walks toward its target one step at a time. Not luminance: the muted
// grey is brighter than the waiting red, so a fade toward it is a loss of
// saturation, not of light.
func distance(a, b color.Color) float64 {
	ar, ag, ab := rgb8(a)
	br, bg, bb := rgb8(b)
	dr, dg, db := float64(ar)-float64(br), float64(ag)-float64(bg), float64(ab)-float64(bb)
	return dr*dr + dg*dg + db*db
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

// The newest cell is the status colour itself, untouched: it is what answers
// "which of these needs me now", so it never dims (#154, the one thing the
// operator asked to keep).
func TestLaneCellColor_TheNewestCellIsTheFullStatusColour_issue154(t *testing.T) {
	for _, s := range []watcher.Status{watcher.StatusWaiting, watcher.StatusThinking, watcher.StatusDone} {
		if got := ui.LaneCellColor(s, 0); !sameRGB(got, ui.StatusColor(s)) {
			t.Errorf("newest %s cell = %v, want the status colour %v", s, got, ui.StatusColor(s))
		}
	}
}

// Older cells fade toward the muted grey, monotonically, and the oldest is
// still not grey: a fully faded cell would be indistinguishable from exited.
func TestLaneCellColor_OlderCellsFadeTowardGreyButNotInto_issue154(t *testing.T) {
	prev := distance(ui.StatusColor(watcher.StatusWaiting), ui.MutedColor())
	for age := 1; age < ui.LaneCells(); age++ {
		d := distance(ui.LaneCellColor(watcher.StatusWaiting, age), ui.MutedColor())
		if d >= prev {
			t.Errorf("age %d is %.0f from grey, age %d was %.0f; the fade is not monotonic", age, d, age-1, prev)
		}
		prev = d
	}
	if oldest := ui.LaneCellColor(watcher.StatusWaiting, ui.LaneCells()-1); sameRGB(oldest, ui.MutedColor()) {
		t.Error("the oldest cell is exactly the muted grey; it should still carry a trace of its status")
	}
}

// The meter warms from amber to green left to right, so a short bar reads
// amber and a full one ends green.
func TestMeterCellColor_RampsFromAmberToGreen_issue154(t *testing.T) {
	first, last := ui.MeterCellColor(0), ui.MeterCellColor(ui.MeterCells()-1)
	if !sameRGB(first, ui.StatusColor(watcher.StatusThinking)) || !sameRGB(last, ui.StatusColor(watcher.StatusDone)) {
		t.Errorf("meter ends = %v / %v, want thinking's amber and done's green", first, last)
	}
	for i := 1; i < ui.MeterCells(); i++ {
		if sameRGB(ui.MeterCellColor(i), ui.MeterCellColor(i-1)) {
			t.Errorf("meter cells %d and %d share a colour in truecolor", i-1, i)
		}
	}
}

// Every ramp still reads as a ramp when it quantises (#154): at least three
// distinct colours across the lane's fade and the meter's warmth on a
// 256-colour terminal, and at least two on a 16-colour one.
func TestRamps_StillReadAfterQuantising_issue154(t *testing.T) {
	ramps := map[string][]color.Color{}
	for age := range ui.LaneCells() {
		ramps["lane fade"] = append(ramps["lane fade"], ui.LaneCellColor(watcher.StatusWaiting, age))
	}
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

// The rendered lane carries one colour per cell rather than one for the row:
// a lane of six thinking cells has six different SGRs in it.
func TestModel_TheLaneFadesCellByCell_issue154(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	for range ui.LaneCells() {
		status(m, "s1", watcher.PromptSubmitted, time.Now())
	}

	lane := m.LaneOf("s1")

	if n := strings.Count(lane, "\x1b["); n < ui.LaneCells() {
		t.Errorf("lane %q has %d SGR sequences, want one per cell (%d)", lane, n, ui.LaneCells())
	}
}
