package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/ui"
)

// footer is the frame's last line, unstyled.
func footer(m *ui.Model) string {
	lines := frameLines(m)
	return strings.TrimSpace(stripSGR(lines[len(lines)-1]))
}

// After the leader the footer lists what the next key can be, which-key's and
// Helix's space mode's answer to "what was that key" (#439).
func TestFooter_ListsTheLeaderKeysWhileItIsArmed_issue439(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})

	press(m, ctrl('o'))

	got := footer(m)
	for _, want := range []string{"d diff", "f files", "g gate", "i tracker", "? keys", "q quit"} {
		if !strings.Contains(got, want) {
			t.Errorf("the armed footer lacks %q: %q", want, got)
		}
	}
	if strings.Contains(got, "z zoom") {
		t.Errorf("z is offered with no column to zoom: %q", got)
	}
}

// With a column open, the zoom is one of the things the next key can do.
func TestFooter_OffersTheZoomWithAColumnOpen_issue439(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})
	leader(m, key('d'))

	press(m, ctrl('o'))

	if got := footer(m); !strings.Contains(got, "z zoom") {
		t.Errorf("the armed footer does not offer the zoom: %q", got)
	}
}

// The hints are display only: the leader resolves as it always did, and the
// footer goes back to the keys of whatever has focus.
func TestFooter_GoesBackOnceTheLeaderResolves_issue439(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 30})

	leader(m, key('d'))

	if !m.ReviewOpen() {
		t.Fatal("ctrl+o d did not open the diff: the leader stopped routing")
	}
	if got := footer(m); strings.Contains(got, "f files") {
		t.Errorf("the hints outlived the leader: %q", got)
	}
}

// On a narrow window the hints give up whole entries, never the help key.
func TestFooter_TheHintsFitAnEightyColumnWindow_issue439(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 24})

	press(m, ctrl('o'))

	if got := footer(m); !strings.Contains(got, ui.DefaultLeader) || !strings.Contains(got, "? keys") {
		t.Errorf("the narrow armed footer lost the leader or the help key: %q", got)
	}
}
