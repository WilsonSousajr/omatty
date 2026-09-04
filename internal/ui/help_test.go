package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// Every leader key must be reachable from inside omatty. The footer shows a
// working subset because it is truncated to the window (#30); this is where
// the rest lives (#103).
func TestModel_helpListsEveryLeaderKey_issue103(t *testing.T) {
	m, _ := modelWithFakes(t)

	press(m, ctrl('o'))
	press(m, key('?'))

	got := m.View().Content
	for _, k := range []string{"j / k", "/", "n", "N", "a", "R", "x", "r", "d", "f", "?", "q"} {
		if !strings.Contains(got, ui.Leader+" "+k) {
			t.Errorf("the help modal does not list %q:\n%s", ui.Leader+" "+k, got)
		}
	}
}

// Regression, issue #103: the help modal first closed on *any* key. A modal
// makes the terminal unfocused, so the router never arms the leader while one
// is open - which meant the ctrl+o of `ctrl+o q` closed the help and the q
// went straight to Claude as text. The M4 smoke test found a literal q sitting
// in the pane. esc closes it; nothing else does.
func TestModel_helpClosesOnEscAndSwallowsTheLeader_issue103(t *testing.T) {
	m, fakes := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('?'))

	press(m, ctrl('o'))
	press(m, key('q'))

	if got := m.View().Content; !strings.Contains(got, "esc to close") {
		t.Errorf("the help modal closed on the leader:\n%s", got)
	}
	if n := len(fakes["s1"].Msgs); n != 0 {
		t.Errorf("%d keys reached the PTY while help was open, want 0", n)
	}
	press(m, special(tea.KeyEscape))
	if got := m.View().Content; strings.Contains(got, "esc to close") {
		t.Errorf("esc did not close the help modal:\n%s", got)
	}
}

// Regression, issue #103: the footer was 114 columns and truncated at 100, so
// `ctrl+o f files` was invisible and M4's four new keys would have taken it to
// 183. It is now a subset that fits, with the key that reaches the rest of the
// keymap early enough that truncation cannot reach it.
func TestModel_footerFitsTheDefaultWindow_issue103(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: ui.DefaultWidth, Height: ui.DefaultHeight})

	lines := strings.Split(m.View().Content, "\n")
	last := lines[len(lines)-1]

	if w := lipgloss.Width(last); w > ui.DefaultWidth {
		t.Errorf("footer is %d columns wide, want at most %d:\n%s", w, ui.DefaultWidth, last)
	}
	// The help key must survive whatever truncation there is, or the rest of
	// the keymap is unreachable.
	if !strings.Contains(last, ui.Leader+" ?") {
		t.Errorf("the footer does not show the help key at %d columns:\n%s", ui.DefaultWidth, last)
	}
}

// Issue #28, for the help modal.
func TestModel_ctrlCQuitsWhileHelpIsOpen_issue28(t *testing.T) {
	m, _ := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('?'))

	_, cmd := m.Update(ctrl('c'))

	if !isQuit(cmd) {
		t.Error("ctrl+c while the help modal is open did not quit")
	}
}
