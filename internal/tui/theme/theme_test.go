package theme_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/tui/theme"
)

// One hue, one meaning (#175): amber is waiting and nothing else, and a
// status with no colour of its own reads muted rather than vanishing.
func TestStatusColor_issue175(t *testing.T) {
	if theme.StatusColor(status.StatusWaiting) != theme.ColorAmber {
		t.Error("waiting is not amber")
	}
	if theme.StatusColor(status.Status("no-such-status")) != theme.ColorMuted {
		t.Error("a status with no colour of its own is not muted")
	}
}

// Clip cuts to the width it is given, in cells.
func TestClip_cutsToWidth(t *testing.T) {
	if got := theme.Clip(3).Render("abcdef"); got != "abc" {
		t.Errorf("Clip(3) = %q, want abc", got)
	}
	if got := theme.Plain().Render("x"); got != "x" {
		t.Errorf("Plain().Render(x) = %q, want x unchanged", got)
	}
	if got := theme.Foreground(theme.ColorRed).Render("x"); !strings.Contains(got, "203") {
		t.Errorf("Foreground(red).Render(x) = %q, want the red index 203", got)
	}
}
