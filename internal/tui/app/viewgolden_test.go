package app_test

import (
	"fmt"
	"path/filepath"
	"testing"
)

// goldenSizes are the smoke test's window and the narrowest common one.
var goldenSizes = [][2]int{{120, 30}, {80, 24}}

// renderScene builds sc at w x h and returns the frame bubbletea would draw.
func renderScene(t *testing.T, sc scene, w, h int) string {
	t.Helper()
	return sc.build(t, w, h).View().Content
}

// Regression net, issue #620: pins every main screen's frame before ADR 0001
// splits internal/ui into tui/app and screens. The .golden keeps the SGR, so a
// theme move that changes one colour fails; the .plain.golden strips it so a
// reviewer can read the diff.
func TestView_mainScreensArePinned_issue620(t *testing.T) {
	for _, sc := range scenes {
		for _, size := range goldenSizes {
			name := fmt.Sprintf("%s_%dx%d", sc.name, size[0], size[1])
			t.Run(name, func(t *testing.T) {
				frame := renderScene(t, sc, size[0], size[1])
				assertGolden(t, filepath.Join("view", name+".golden"), []byte(frame))
				assertGolden(t, filepath.Join("view", name+".plain.golden"), []byte(stripSGR(frame)))
			})
		}
	}
}

// A golden that differs between two renders, or with the terminal's colour
// environment, would fail at random and teach everyone to rerun -update.
func TestView_goldensAreDeterministic_issue620(t *testing.T) {
	for _, env := range [][2]string{{"NO_COLOR", "1"}, {"TERM", "dumb"}, {"COLORTERM", "truecolor"}} {
		t.Run(env[0], func(t *testing.T) {
			t.Setenv(env[0], env[1])
			for _, sc := range scenes {
				a, b := renderScene(t, sc, 120, 30), renderScene(t, sc, 120, 30)
				if a != b {
					t.Errorf("%s renders differently twice under %s=%s", sc.name, env[0], env[1])
				}
				assertGolden(t, filepath.Join("view", sc.name+"_120x30.golden"), []byte(a))
			}
		})
	}
}
