package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// gateHelp is ctrl+o ? pressed with the gate on show.
func gateHelp(t *testing.T) *ui.Model {
	t.Helper()
	m, _, _ := modelWithDiff(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: helpFitsHeight})
	m.SetGateReport("s1", gateReport(gate.Pass))
	leader(m, key('g'))
	leader(m, key('?'))
	return m
}

// ? on a face opens on that face's keys: the same key teaches whatever is in
// front of you (#438, lazygit, diffview's g?).
func TestHelp_OpensOnTheFaceYouAreIn_issue438(t *testing.T) {
	m := gateHelp(t)

	if first := stripSGR(frameLines(m)[2]); !strings.Contains(first, "in the gate") {
		t.Errorf("help on the gate opened on %q, want the gate's own section", first)
	}
}

// Styled: a section's title bold, a key in the accent, so the list scans.
func TestHelp_IsStyled_issue438(t *testing.T) {
	m := gateHelp(t)

	body := m.View().Content
	if !strings.Contains(body, ui.Bold("in the gate")) {
		t.Errorf("a section title is not bold:\n%s", body)
	}
	if !strings.Contains(body, "\x1b[38;5;75menter") {
		t.Errorf("a key is not drawn in the accent:\n%s", body)
	}
}

// / filters the keymap by key or by what it does; esc lifts the filter before
// it closes the modal, as the tree's does.
func TestHelp_SlashFiltersTheKeymap_issue438(t *testing.T) {
	m := gateHelp(t)

	press(m, key('/'))
	typeInto(m, "browser")
	body := stripSGR(m.View().Content)
	if !strings.Contains(body, "open it in the browser") || strings.Contains(body, "comment on the line") {
		t.Errorf("the filter did not narrow the keymap to browser:\n%s", body)
	}
	press(m, special(tea.KeyEnter))
	press(m, special(tea.KeyEscape))
	if body := stripSGR(m.View().Content); !strings.Contains(body, "comment on the line") {
		t.Errorf("esc did not lift the filter")
	}
	press(m, special(tea.KeyEscape))
	if strings.Contains(stripSGR(frameLines(m)[0]), "keys") {
		t.Errorf("a second esc did not close help")
	}
}

// M4's trap, kept shut: the leader still closes help from inside the filter,
// so ctrl+o q quits rather than typing q into the query (#103).
func TestHelp_TheLeaderQuitsFromInsideTheFilter_issue438(t *testing.T) {
	m := gateHelp(t)
	press(m, key('/'))
	typeInto(m, "x")

	press(m, ctrl('o'))
	_, cmd := m.Update(key('q'))

	if !isQuit(cmd) {
		t.Errorf("ctrl+o q from inside the help filter did not quit")
	}
}
