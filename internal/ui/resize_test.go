package ui_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// modalOpeners is every keystroke sequence that opens a surface owning the
// keyboard. M4 adds three more (rename, confirm, switcher/picker); each adds a
// row here rather than writing its own resize test. Issue #95 is a class of
// bug, not one path: every such surface makes the terminal "not focused", and
// a surface that skips this case re-creates the bug silently.
var modalOpeners = []struct {
	name string
	keys []tea.KeyPressMsg
}{
	{"prompt", []tea.KeyPressMsg{ctrl('o'), key('n')}},
	{"rename", []tea.KeyPressMsg{ctrl('o'), shift('r', "R")}},
}

// Regression, issue #95: resizing the window while a modal surface owned the
// keyboard reached no terminal at all, because resizeFocused asked
// focusedTerminal, which is deliberately nil while a surface is open. Closing
// the surface did not re-issue a resize either, so the PTY kept its old size
// and claude painted into the wrong box - the #51/#73/#75 symptom, reached by
// a fourth path.
func TestModel_ResizeBehindAnOpenModalStillReachesTheSelectedTerminal_issue95(t *testing.T) {
	for _, tc := range modalOpeners {
		t.Run(tc.name, func(t *testing.T) {
			m, fakes := modelWithFakes(t)
			m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
			for _, k := range tc.keys {
				press(m, k)
			}

			m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

			// Asserted against PTYSize rather than the literal 90x36: the
			// numbers live in layout.go and this test must not drift from
			// them (issue #75).
			wantW, wantH := ui.PTYSize(120, 40, false)
			f := fakes["s1"]
			if f.Width != wantW || f.Height != wantH {
				t.Fatalf("selected terminal is %dx%d behind an open %s, want %dx%d",
					f.Width, f.Height, tc.name, wantW, wantH)
			}
			// And it is already right when the pane comes back, rather than
			// waiting for the next j/k to correct it - the operator-visible
			// symptom in the issue.
			press(m, special(tea.KeyEscape))
			if f.Width != wantW || f.Height != wantH {
				t.Errorf("selected terminal is %dx%d after closing the %s, want %dx%d",
					f.Width, f.Height, tc.name, wantW, wantH)
			}
		})
	}
}
