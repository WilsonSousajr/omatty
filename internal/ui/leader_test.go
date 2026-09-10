package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// The leader is whatever the config names. Invariant 1 restated for a new
// leader: the old one is then an ordinary key and reaches the PTY (#44).
func TestModel_TheConfiguredLeaderRoutesInsteadOfCtrlO_issue44(t *testing.T) {
	terms, fakes := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Leader = "ctrl+a"
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	press(m, ctrl('a'))
	press(m, key('j'))
	if m.Selected() != "s2" {
		t.Fatalf("ctrl+a j selected %q, want s2", m.Selected())
	}
	press(m, ctrl('o'))
	if len(fakes["s2"].Msgs) == 0 {
		t.Error("ctrl+o did not reach the PTY once ctrl+a is the leader")
	}
}

func TestModel_NoLeaderInDepsKeepsCtrlO_issue44(t *testing.T) {
	m, _ := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('j'))
	if m.Selected() != "s2" {
		t.Errorf("ctrl+o j selected %q with no Leader configured, want s2", m.Selected())
	}
}

func TestModel_TheFooterAndHelpNameTheConfiguredLeader_issue44(t *testing.T) {
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Leader = "ctrl+a"
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	if view := m.View().Content; !strings.Contains(view, "ctrl+a q") || strings.Contains(view, "ctrl+o") {
		t.Errorf("footer does not name the configured leader:\n%s", view)
	}
	press(m, ctrl('a'))
	press(m, key('?'))
	// The body's bindings carry the leader; the body has no title of its own
	// since the header row names the modal (#188).
	if view := m.View().Content; !strings.Contains(view, "ctrl+a j / k") || strings.Contains(view, "ctrl+o") {
		t.Errorf("help does not name the configured leader:\n%s", view)
	}
}

// #103's guarantee, for a leader of any length: the exit key is on screen.
func TestModel_ALongLeaderStillLeavesTheExitKeyOnScreen_issue44(t *testing.T) {
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Leader = "ctrl+shift+alt+o"
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if !strings.Contains(m.View().Content, "ctrl+shift+alt+o q") {
		t.Error("the exit key fell off an 80-column footer")
	}
}
