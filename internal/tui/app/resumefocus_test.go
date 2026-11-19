package app_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// `omatty --resume <id>` opens on the row claude's exit hint named (#815),
// not on the first row of the sidebar.
func TestModel_opensOnTheFocusedSession_issue815(t *testing.T) {
	st := twoProjectState()
	d := baseDeps(st, fakeTermsFor(st))
	d.Focus = "s3"

	m := app.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if got := m.Selected(); got != "s3" {
		t.Errorf("Selected() = %q, want s3, the session --resume named", got)
	}
}

// Without --resume the boot is unchanged: the first session.
func TestModel_opensOnTheFirstSessionWithoutAFocus_issue815(t *testing.T) {
	st := twoProjectState()
	m := app.NewModel(baseDeps(st, fakeTermsFor(st)))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if got := m.Selected(); got != "s1" {
		t.Errorf("Selected() = %q, want s1", got)
	}
}
