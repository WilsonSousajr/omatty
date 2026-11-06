package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// startFromTracker is S on the row under the cursor, then enter on the
// prefilled worktree prompt it opens (#777).
func startFromTracker(m *app.Model) {
	pressDeliver(m, shiftS)
	pressDeliver(m, special(tea.KeyEnter))
}

// Regression, issue #777: in the diff and the gate n is "next", and in the
// tracker it created a worktree, a branch and a running agent on one
// unconfirmed key. n and N now do nothing in the tracker, list or open item.
func TestKeys_nNeverCreatesASession_issue777(t *testing.T) {
	for _, item := range []bool{false, true} {
		m, create, _, _ := workModel(t)
		if item {
			pressDeliver(m, special(tea.KeyEnter))
		}

		pressDeliver(m, key('n'))
		pressDeliver(m, tea.KeyPressMsg{Code: 'n', Mod: tea.ModShift, Text: "N"})

		if create.Calls != 0 || m.ModalOpen() {
			t.Errorf("n/N in the tracker (item open: %v) created %d sessions, modal open %v; want neither",
				item, create.Calls, m.ModalOpen())
		}
	}
}

// Issue #777: S opens the worktree prompt with the branch filled in and starts
// nothing until enter, so the most expensive action in the column takes two
// keys and shows what it will make first.
func TestTracker_SOpensAPrefilledPromptAndCreatesNothing_issue777(t *testing.T) {
	m, create, _, _ := workModel(t)

	pressDeliver(m, shiftS)

	if create.Calls != 0 {
		t.Fatalf("S created %d sessions before enter, want none", create.Calls)
	}
	if p := m.Prompt(); !p.Active || !p.Worktree || p.Buffer != "399-filter-the-tracker-and-the-argument" {
		t.Errorf("prompt = %+v, want a worktree prompt holding the issue's branch", p)
	}
	if got := m.View().Content; !strings.Contains(got, "#399") {
		t.Errorf("the prompt does not say which issue it starts on:\n%s", got)
	}
}

// Issue #777: esc on the prefilled prompt starts nothing and leaves the
// tracker with the keys, where it was.
func TestTracker_escOnThePromptStartsNothing_issue777(t *testing.T) {
	m, create, _, _ := workModel(t)
	pressDeliver(m, shiftS)

	pressDeliver(m, special(tea.KeyEscape))

	if create.Calls != 0 || m.ModalOpen() || !m.ReviewFocused() {
		t.Errorf("after esc: created %d, modal open %v, tracker focused %v; want 0, false, true",
			create.Calls, m.ModalOpen(), m.ReviewFocused())
	}
}

// Issue #777: the branch in the prompt is a suggestion; an edited one is the
// branch the session gets, and the title stays the issue's.
func TestTracker_anEditedBranchIsTheOneCreated_issue777(t *testing.T) {
	m, create, _, _ := workModel(t)
	pressDeliver(m, shiftS)
	for range len("argument") + 1 {
		pressDeliver(m, special(tea.KeyBackspace))
	}

	pressDeliver(m, special(tea.KeyEnter))

	if create.Branch != "399-filter-the-tracker-and-the" || !strings.Contains(create.Title, "#399") {
		t.Errorf("created branch %q title %q, want the edited branch and the issue's title", create.Branch, create.Title)
	}
}
