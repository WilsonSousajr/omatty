package theme_test

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/tui/theme"
)

// Issue #763: the ground survives each reset inside the row, so no span of it
// falls back to the terminal's background, and the row is padded to width.
func TestUnderCursor_groundSurvivesResets_issue763(t *testing.T) {
	styled := theme.Added.Render("+") + " " + theme.Removed.Render("x") + " y"

	got := theme.UnderCursor(styled, 12)

	if w := lipgloss.Width(got); w != 12 {
		t.Errorf("width = %d, want 12", w)
	}
	if n, resets := strings.Count(got, "48;5;237"), strings.Count(styled, "\x1b[m")+strings.Count(styled, "\x1b[0m"); n != resets+1 {
		t.Errorf("the ground is set %d times for %d resets, want once more than the resets: %q", n, resets, got)
	}
	if !strings.Contains(got, "38;5;") {
		t.Errorf("the row lost its foregrounds: %q", got)
	}
}

// Issue #763: a row already wider than width is not padded.
func TestUnderCursor_noPadPastWidth_issue763(t *testing.T) {
	if got := theme.UnderCursor("abcdef", 3); lipgloss.Width(got) != 6 {
		t.Errorf("width = %d, want the row's own 6", lipgloss.Width(got))
	}
}
