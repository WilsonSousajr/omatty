package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// footerOf is the frame's last line.
func footerOf(m *ui.Model) string {
	lines := strings.Split(strings.TrimRight(m.View().Content, "\n"), "\n")
	return lines[len(lines)-1]
}

// The keymap is 77 cells with its leading space and the facts 22, so 120
// columns hold both; 100 would not, and that case is the drop test below.
func TestFooter_CarriesTheSessionCountAndTheWaitingCountOnTheRight_issue178(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	status(m, "s3", watcher.PermissionRequested, time.Now())

	got := footerOf(m)

	plain := stripSGR(got)
	if !strings.HasPrefix(plain, " ctrl+o q quit") || !strings.HasSuffix(plain, "3 sessions · 1 waiting") {
		t.Errorf("footer = %q, want the keys left and the facts right", plain)
	}
	if lipgloss.Width(got) != 120 {
		t.Errorf("footer is %d cells, want 120", lipgloss.Width(got))
	}
	if !strings.Contains(got, ui.Amber("1 waiting")) {
		t.Errorf("the waiting count is not amber: %q", got)
	}
}

func TestFooter_OmitsWaitingWhenNobodyWaits_issue178(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	plain := stripSGR(footerOf(m))
	if !strings.HasSuffix(plain, "3 sessions") || strings.Contains(plain, "waiting") {
		t.Errorf("footer = %q, want it to end in the session count alone", plain)
	}
}

// The keymap is 77 cells with its leading space; at 80 columns the facts do
// not fit and are dropped whole, so the exit key never moves (#28, #30).
func TestFooter_DropsTheFactsBeforeTheKeys_issue178(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	plain := stripSGR(footerOf(m))
	if strings.Contains(plain, "sessions") || !strings.HasPrefix(plain, " ctrl+o q quit") {
		t.Errorf("footer = %q at 80 columns, want the keys alone", plain)
	}
	if lipgloss.Width(footerOf(m)) != 80 {
		t.Errorf("footer is %d cells, want 80", lipgloss.Width(footerOf(m)))
	}
}

func TestFooter_OneSessionIsSingular_issue178(t *testing.T) {
	st := twoProjectState()
	st.Sessions = st.Sessions[:1]
	m := ui.NewModel(baseDeps(st, fakeTermsFor(st)))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if plain := stripSGR(footerOf(m)); !strings.HasSuffix(plain, "1 session") {
		t.Errorf("footer = %q, want it to end in \"1 session\"", plain)
	}
}
