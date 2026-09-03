package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Invariant 8, end to end: three comments leave as one SendInput carrying one
// bracketed paste and one carriage return. Keystrokes never use SendInput, so
// what lands in Sent is the review and nothing else (#23).
func TestModel_SSubmitsEveryCommentAsOneBracketedPaste_issue23(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 3)
	typeNote(m, "was this needed?")
	down(m, 2)
	typeNote(m, "use a match here")
	down(m, 7)
	typeNote(m, "name this file")

	press(m, tea.KeyPressMsg{Code: 's', Mod: tea.ModShift, Text: "S"})

	sent := fakes["s1"].Sent
	if len(sent) != 1 {
		t.Fatalf("SendInput called %d times, want exactly 1: %q", len(sent), sent)
	}
	body := sent[0]
	if !strings.HasPrefix(body, "\x1b[200~Review comments (3):") {
		t.Errorf("message does not open the paste with the header:\n%q", body)
	}
	if !strings.HasSuffix(body, "\x1b[201~\r") {
		t.Errorf("message does not close the paste then submit once:\n%q", body)
	}
	if n := strings.Count(body, "\r"); n != 1 {
		t.Errorf("%d carriage returns, want 1: every extra one submits a fragment", n)
	}
	for _, want := range []string{
		"1. internal/ui/model.go:11\n   > \tb := 2\n   was this needed?",
		"2. internal/ui/model.go:11\n   > \tb := 3\n   use a match here",
		"3. new.txt:1\n   > fresh\n   name this file",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("message lacks %q:\n%s", want, body)
		}
	}
	if m.PendingComments() != 0 {
		t.Errorf("pending = %d after submit, want 0", m.PendingComments())
	}
	if m.ReviewFocused() {
		t.Error("focus should return to the terminal so the operator watches claude act")
	}
}

func TestModel_SWithNoCommentsExplainsInTheFooter_issue23(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))

	press(m, key('S'))

	if len(fakes["s1"].Sent) != 0 {
		t.Errorf("sent %q with nothing queued", fakes["s1"].Sent)
	}
	if !strings.Contains(m.View().Content, "no comments to submit") {
		t.Errorf("footer does not explain:\n%s", m.View().Content)
	}
}

// The keystroke S must not reach claude as a typed character while the review
// column owns the keys.
func TestModel_SDoesNotTypeIntoClaude_issue23(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))
	before := len(fakes["s1"].Msgs)

	press(m, key('S'))

	if len(fakes["s1"].Msgs) != before {
		t.Error("S reached the terminal as a keystroke")
	}
}
