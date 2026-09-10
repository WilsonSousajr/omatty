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
	{"confirm", []tea.KeyPressMsg{ctrl('o'), key('x')}},
	{"switcher", []tea.KeyPressMsg{ctrl('o'), key('/')}},
	{"picker", []tea.KeyPressMsg{ctrl('o'), key('a')}},
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
			//
			// The window changes size again while the surface is open, so
			// closing it has something to be right about. Pressing esc alone
			// asserted the value the check above had just read, and could not
			// fail for the bug (#95).
			m.Update(tea.WindowSizeMsg{Width: 140, Height: 44})
			press(m, special(tea.KeyEscape))
			wantW, wantH = ui.PTYSize(140, 44, false)
			if f.Width != wantW || f.Height != wantH {
				t.Errorf("selected terminal is %dx%d after closing the %s, want %dx%d",
					f.Width, f.Height, tc.name, wantW, wantH)
			}
		})
	}
}

// The same class, with the review column open: ptySize reads m.review.Open,
// which is the one piece of state that changes the expected answer. Every
// other case here runs with the column closed, so a regression that dropped
// the review term from the calculation - leaving claude painting 37 columns
// wider than its pane - would keep the whole table green (#95, #21).
func TestModel_ResizeBehindAModalAccountsForTheReviewColumn_issue95(t *testing.T) {
	m, fakes := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	press(m, ctrl('o'))
	press(m, key('d')) // open the review column
	press(m, ctrl('o'))
	press(m, key('n')) // and a modal over it

	m.Update(tea.WindowSizeMsg{Width: 140, Height: 44})

	wantW, wantH := ui.PTYSize(140, 44, true)
	f := fakes["s1"]
	if f.Width != wantW || f.Height != wantH {
		t.Errorf("selected terminal is %dx%d with the review column open, want %dx%d",
			f.Width, f.Height, wantW, wantH)
	}
}

// Regression, issue #95: a wider window lowers the ceiling on the review
// column's horizontal pan, and nothing re-clamped it. An offset left over from
// a narrow window made panLine drop every row shorter than it, so the column
// rendered blank until h, l or 0 was pressed.
func TestModel_ResizeReclampsTheReviewColumnsPan_issue95(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	press(m, ctrl('o'))
	press(m, key('d'))
	for range 40 { // pan hard right, to whatever the narrow window allows
		press(m, key('l'))
	}
	narrow := m.ReviewColOffset()

	m.Update(tea.WindowSizeMsg{Width: 200, Height: 40})

	if got := m.ReviewColOffset(); got > narrow {
		t.Errorf("ReviewColOffset() = %d after widening, want at most the old %d", got, narrow)
	}
}
