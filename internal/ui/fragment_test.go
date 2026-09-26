package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// sentText is everything the review submitted into the session's terminal.
func sentText(t *testing.T, fakes map[string]*termwrap.Fake) string {
	t.Helper()
	if len(fakes["s1"].Sent) == 0 {
		t.Fatal("nothing was sent to the session")
	}
	return strings.Join(fakes["s1"].Sent, "")
}

// typeIn presses each rune of s, then enter.
func typeIn(m *ui.Model, s string) {
	for _, r := range s {
		press(m, key(r))
	}
	press(m, special(tea.KeyEnter))
}

// onDiffLine opens the diff and puts the cursor on the sample diff's added
// `c := 4` line. The row count is asserted rather than assumed: the first
// draft of these tests counted one row short and typed a fragment that was
// genuinely not on the line it landed on, which the refusal then reported
// correctly.
func onDiffLine(t *testing.T) (*ui.Model, map[string]*termwrap.Fake) {
	t.Helper()
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))
	for range 5 {
		press(m, key('j'))
	}
	press(m, key('c'))
	lineWith(t, m.View().Content, "note:") // fatals unless the cursor is on a line
	press(m, special(tea.KeyEscape))
	return m, fakes
}

// The first half of #339. `Anchor{File, Hunk, Hash, Nth}` always supported it
// and the comment store is a slice, so this asserts end to end what was never
// asserted: two notes on one line are both kept, both shown, and both sent.
func TestModel_TwoCommentsOnOneLineAreBothSent_issue339(t *testing.T) {
	m, fakes := onDiffLine(t)

	press(m, key('c'))
	typeIn(m, "first")
	press(m, key('c'))
	typeIn(m, "second")

	if got := m.PendingComments(); got != 2 {
		t.Fatalf("PendingComments() = %d, want 2", got)
	}
	view := m.View().Content
	if !strings.Contains(view, "first") || !strings.Contains(view, "second") {
		t.Errorf("both notes should be on the line:\n%s", view)
	}

	press(m, key('S'))

	sent := sentText(t, fakes)
	for _, want := range []string{"(2)", "1. ", "2. ", "first", "second"} {
		if !strings.Contains(sent, want) {
			t.Errorf("%q missing from the submitted message:\n%s", want, sent)
		}
	}
}

// d must take the one under the cursor, not the first on the line. Deleting by
// line would silently drop the wrong note.
func TestModel_dDeletesOnlyTheCommentUnderTheCursor_issue339(t *testing.T) {
	m, _ := onDiffLine(t)
	press(m, key('c'))
	typeIn(m, "keep me")
	press(m, key('c'))
	typeIn(m, "drop me")

	press(m, key('j')) // onto the first comment row
	press(m, key('j')) // onto the second
	press(m, key('d'))

	if got := m.PendingComments(); got != 1 {
		t.Fatalf("PendingComments() = %d, want 1", got)
	}
	view := m.View().Content
	if !strings.Contains(view, "keep me") {
		t.Errorf("the wrong comment was deleted:\n%s", view)
	}
	if strings.Contains(view, "drop me") {
		t.Errorf("the comment under the cursor survived:\n%s", view)
	}
}

// The second half: C says which part of the line the note is about, and it
// reaches claude as the quoted fragment.
func TestModel_CCommentsOnPartOfTheLine_issue339(t *testing.T) {
	m, fakes := onDiffLine(t)

	press(m, key('C'))
	typeIn(m, "c :=") // the fragment
	typeIn(m, "why 3?")

	if got := m.PendingComments(); got != 1 {
		t.Fatalf("PendingComments() = %d, want 1", got)
	}

	press(m, key('S'))

	sent := sentText(t, fakes)
	if !strings.Contains(sent, `about: "c :="`) {
		t.Errorf("the fragment did not reach the composed message:\n%s", sent)
	}
	if !strings.Contains(sent, "why 3?") {
		t.Errorf("the note did not reach the composed message:\n%s", sent)
	}
}

// A fragment that is not in the line is a typo, and it is caught while the
// operator is still looking at the line rather than after the message is sent.
func TestModel_CRefusesAFragmentThatIsNotInTheLine_issue339(t *testing.T) {
	m, _ := onDiffLine(t)

	press(m, key('C'))
	typeIn(m, "not in this line at all")

	if got := m.PendingComments(); got != 0 {
		t.Errorf("PendingComments() = %d, want the note refused", got)
	}
	if view := m.View().Content; !strings.Contains(view, "not on this line") {
		t.Errorf("nothing said why the fragment was refused:\n%s", view)
	}
}

// esc out of the fragment prompt leaves nothing behind, the way esc out of the
// note editor does.
func TestModel_escInTheFragmentPromptQueuesNothing_issue339(t *testing.T) {
	m, _ := onDiffLine(t)

	press(m, key('C'))
	press(m, key('b'))
	press(m, special(tea.KeyEscape))

	if got := m.PendingComments(); got != 0 {
		t.Errorf("PendingComments() = %d, want 0", got)
	}
	press(m, key('j')) // the pane still takes keys rather than being stuck
}
