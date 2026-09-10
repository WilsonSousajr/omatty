package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Bracketed paste is on, so the host wraps a paste and bubbletea hands the
// model a PasteMsg rather than keystrokes. Nothing routed it: it fell through
// to the broadcast, which bubbleterm ignores, and no PTY ever saw it (#190).
// A paste goes to the focused terminal alone, re-bracketed (invariant 8).
func TestModel_pasteReachesTheFocusedPaneOnly_issue190(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.PasteMsg{Content: "hello\nworld"})

	if got := strings.Join(fakes["s1"].Sent, "|"); got != "\x1b[200~hello\nworld\x1b[201~" {
		t.Errorf("focused pane received %q, want the content inside paste brackets", got)
	}
	for _, other := range []string{"s2", "s3"} {
		if len(fakes[other].Sent) != 0 {
			t.Errorf("%s received %q; a paste is not broadcast", other, fakes[other].Sent)
		}
		for _, msg := range fakes[other].Msgs {
			if _, ok := msg.(tea.PasteMsg); ok {
				t.Errorf("%s had the PasteMsg broadcast to it", other)
			}
		}
	}
}

// The operator pasted, not pressed enter: no carriage return follows.
func TestModel_pasteCarriesNoCarriageReturn_issue190(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.PasteMsg{Content: "one\ntwo\n"})

	if len(fakes["s1"].Sent) != 1 {
		t.Fatalf("sent %q, want one write", fakes["s1"].Sent)
	}
	if body := fakes["s1"].Sent[0]; !strings.HasSuffix(body, "\x1b[201~") || strings.Contains(body, "\r") {
		t.Errorf("sent %q, want it to end at the paste bracket with no CR anywhere", body)
	}
}

// The brackets themselves are omatty's to write, once; the host's arrive as
// their own messages and must not become text.
func TestModel_pasteBracketsAreNotForwarded_issue190(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.PasteStartMsg{})
	m.Update(tea.PasteEndMsg{})

	if len(fakes["s1"].Sent) != 0 {
		t.Errorf("sent %q for the bare brackets, want nothing", fakes["s1"].Sent)
	}
}

// The note editor is the other surface that owns the keyboard; a paste
// there lands in the note, not in the PTY.
func TestModel_pasteIntoTheNoteEditor_issue190(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 3)
	press(m, key('c'))

	m.Update(tea.PasteMsg{Content: "was this needed?"})
	press(m, special(tea.KeyEnter))

	if len(fakes["s1"].Sent) != 0 {
		t.Errorf("the PTY received %q while the note editor had the keys", fakes["s1"].Sent)
	}
	if m.PendingComments() != 1 {
		t.Fatalf("pending = %d after pasting a note and pressing enter, want 1", m.PendingComments())
	}
	lineWith(t, m.View().Content, "was this needed?")
}

// The review column and a modal own no text field; a paste there is dropped
// rather than typed into claude behind them.
func TestModel_pasteWithTheColumnOrAModalFocusedIsDropped_issue190(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))
	m.Update(tea.PasteMsg{Content: "dropped"})
	press(m, special(tea.KeyEscape))
	leader(m, key('?'))
	m.Update(tea.PasteMsg{Content: "dropped too"})

	if len(fakes["s1"].Sent) != 0 {
		t.Errorf("the PTY received %q, want nothing", fakes["s1"].Sent)
	}
}

// The help modal and the README say how text leaves a pane, since mouse
// reporting took the host's plain drag away (#107) and nothing gives it back.
func TestModel_helpSaysHowToCopyOutOfAPane_issue190(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 50})
	leader(m, key('?'))
	lineWith(t, m.View().Content, "shift/opt+drag")
	lineWith(t, m.View().Content, "paste")
}
