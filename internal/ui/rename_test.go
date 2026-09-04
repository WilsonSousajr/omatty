package ui_test

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// recordRename is a named fake capturing what the rename box asked for.
type recordRename struct {
	SessionID string
	Title     string
	Calls     int
	Err       error
}

func (r *recordRename) fn(sessionID, title string) error {
	r.Calls++
	r.SessionID, r.Title = sessionID, title
	return r.Err
}

func modelWithRename(t *testing.T, r *recordRename) (*ui.Model, map[string]*termwrap.Fake) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Rename = r.fn
	return ui.NewModel(d), fakes
}

// shift spells a capital the way a terminal reporting the modifier does.
func shift(base rune, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: base, Mod: tea.ModShift, Text: text}
}

// Both spellings must open the rename box. Keystroke() gives "shift+r" on a
// terminal that reports the modifier and the bare "R" on one that cannot; the
// upper-case "shift+R" never occurs (issue #87). Lower-case r is restart, so a
// missed spelling would silently restart nothing instead of renaming.
func TestModel_leaderROpensTheRenameBoxFromBothSpellings_issue87(t *testing.T) {
	// The two live spellings only: shift+r from a terminal that reports the
	// modifier, and the bare R from a legacy one that cannot.
	for _, k := range []tea.KeyPressMsg{shift('r', "R"), {Code: 'R', Text: "R"}} {
		m, _ := modelWithRename(t, &recordRename{})

		press(m, ctrl('o'))
		press(m, k)

		// Pre-filled with the selected session's title, so correcting a typo
		// is a small edit rather than a retype.
		if got := m.View().Content; !strings.Contains(got, "rename") || !strings.Contains(got, "main") {
			t.Errorf("keystroke %q did not open the rename box on the selected session:\n%s",
				k.Keystroke(), got)
		}
	}
}

func TestModel_renameCommitsTheNewTitle_issue41(t *testing.T) {
	r := &recordRename{}
	m, _ := modelWithRename(t, r)

	press(m, ctrl('o'))
	press(m, shift('r', "R"))
	for range len("main") {
		press(m, special(tea.KeyBackspace))
	}
	for _, c := range "parser-fix" {
		press(m, key(c))
	}
	press(m, special(tea.KeyEnter))

	if r.Calls != 1 || r.SessionID != "s1" || r.Title != "parser-fix" {
		t.Fatalf("rename called %d times with (%q, %q), want once with (s1, parser-fix)",
			r.Calls, r.SessionID, r.Title)
	}
	if got := m.View().Content; !strings.Contains(got, "parser-fix") {
		t.Errorf("sidebar does not show the new title:\n%s", got)
	}
}

// The renamed row must still be the selected row: a rename is not a move.
func TestModel_renameKeepsTheCursorOnTheRenamedSession_issue41(t *testing.T) {
	m, _ := modelWithRename(t, &recordRename{})

	press(m, ctrl('o'))
	press(m, key('j')) // onto s2, so a fallback to the first row would show
	press(m, ctrl('o'))
	press(m, shift('r', "R"))
	press(m, special(tea.KeyEnter))

	if got := m.Focused(); got != "s2" {
		t.Errorf("focused session = %q after renaming s2, want s2", got)
	}
}

func TestModel_renameFailureSurfacesAndKeepsTheOldTitle_issue41(t *testing.T) {
	r := &recordRename{Err: errors.New("state.json is read-only")}
	m, _ := modelWithRename(t, r)

	press(m, ctrl('o'))
	press(m, shift('r', "R"))
	press(m, key('x'))
	press(m, special(tea.KeyEnter))

	got := m.View().Content
	if !strings.Contains(got, "read-only") {
		t.Errorf("View() does not surface the failure:\n%s", got)
	}
	if !strings.Contains(got, "main") {
		t.Errorf("sidebar lost the old title after a failed rename:\n%s", got)
	}
}

// esc must leave the session exactly as it was, without calling through.
func TestModel_renameEscCancelsWithoutPersisting_issue41(t *testing.T) {
	r := &recordRename{}
	m, _ := modelWithRename(t, r)

	press(m, ctrl('o'))
	press(m, shift('r', "R"))
	press(m, key('z'))
	press(m, special(tea.KeyEscape))

	if r.Calls != 0 {
		t.Errorf("rename was called %d times after esc, want 0", r.Calls)
	}
	if got := m.View().Content; !strings.Contains(got, "main") {
		t.Errorf("sidebar does not still show the original title:\n%s", got)
	}
}

// An empty buffer must leave the box open rather than blanking the title,
// which would leave a sidebar row with nothing to aim at.
func TestModel_renameRejectsAnEmptyTitle_issue41(t *testing.T) {
	r := &recordRename{}
	m, _ := modelWithRename(t, r)

	press(m, ctrl('o'))
	press(m, shift('r', "R"))
	for range len("main") {
		press(m, special(tea.KeyBackspace))
	}
	press(m, special(tea.KeyEnter))

	if r.Calls != 0 {
		t.Errorf("rename was called %d times with an empty title, want 0", r.Calls)
	}
	if got := m.View().Content; !strings.Contains(got, "rename") {
		t.Errorf("the rename box closed on an empty title:\n%s", got)
	}
}

// Regression, issue #41: onPromptKey appended the *keystroke* behind a
// len([]rune(key)) == 1 guard, so a capital - which Keystroke() spells
// "shift+f" - was silently dropped and could not be typed into a title. The
// editors take msg.Text instead, as the note editor always has.
func TestModel_editorsAcceptCapitalLetters_issue41(t *testing.T) {
	for _, tc := range []struct {
		name string
		open tea.KeyPressMsg
	}{
		{"prompt", key('n')},
		{"rename", shift('r', "R")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := modelWithRename(t, &recordRename{})
			press(m, ctrl('o'))
			press(m, tc.open)
			for range len("main") { // clear the pre-filled rename buffer
				press(m, special(tea.KeyBackspace))
			}

			press(m, shift('f', "F"))
			press(m, key('i'))
			press(m, shift('x', "X"))

			if got := m.View().Content; !strings.Contains(got, "FiX") {
				t.Errorf("capitals did not reach the %s buffer:\n%s", tc.name, got)
			}
		})
	}
}

// Issue #28, for the rename box: an open surface must never trap the operator.
func TestModel_ctrlCQuitsWhileTheRenameBoxIsOpen_issue28(t *testing.T) {
	m, _ := modelWithRename(t, &recordRename{})
	press(m, ctrl('o'))
	press(m, shift('r', "R"))

	_, cmd := m.Update(ctrl('c'))

	if !isQuit(cmd) {
		t.Error("ctrl+c while the rename box is open did not quit")
	}
}

// Invariant 1: while a surface owns the keyboard its keys build the buffer and
// none of them reach the PTY.
func TestModel_renameKeysBuildTheBufferNotThePTY_issue41(t *testing.T) {
	m, fakes := modelWithRename(t, &recordRename{})
	press(m, ctrl('o'))
	press(m, shift('r', "R"))

	press(m, key('z'))

	if n := len(fakes["s1"].Msgs); n != 0 {
		t.Errorf("focused terminal received %d messages while the rename box was open, want 0", n)
	}
}
