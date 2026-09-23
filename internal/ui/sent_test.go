package ui_test

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// shiftS is S as the review column receives it.
var shiftS = tea.KeyPressMsg{Code: 's', Mod: tea.ModShift, Text: "S"}

// refocusGate gives the gate view the keys back after S handed them to the
// terminal. A click cannot: the gate view has no rows a click can land on, so
// switching views away and back is the operator's way in.
func refocusGate(m *ui.Model) {
	leader(m, key('d'))
	leader(m, key('g'))
}

// refocusColumn gives the review column the keys back after a submit handed
// them to the terminal, the way a click on one of its rows does (#168).
func refocusColumn(m *ui.Model) {
	m.Update(clickAt(reviewHairlineX+5, reviewRowY(0)))
}

// A sent comment is no longer pending: a second S finds nothing to send, and
// the comment stays on its line marked sent, so the next turn can be read
// against what was asked. Before #335 the queue was cleared on submit and the
// note vanished with it.
func TestModel_submitMarksCommentsSentAndKeepsThemShown_issue335(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 3)
	typeNote(m, "why?")

	press(m, shiftS)
	refocusColumn(m)
	press(m, shiftS)

	if n := len(fakes["s1"].Sent); n != 1 {
		t.Fatalf("SendInput called %d times across two S presses, want 1", n)
	}
	body := m.View().Content
	if !strings.Contains(body, "no comments to submit") {
		t.Errorf("the second S does not say there is nothing pending:\n%s", body)
	}
	if !regexp.MustCompile(`>> \(sent \d\d:\d\d\) why\?`).MatchString(body) {
		t.Errorf("the sent comment is not shown on its line, marked sent:\n%s", body)
	}
}

// The next submit carries only what was written since the last one.
func TestModel_aSecondSubmitSendsOnlyNewComments_issue335(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 3)
	typeNote(m, "was this needed?")
	press(m, shiftS)

	refocusColumn(m) // the click puts the cursor on row 0
	down(m, 3)
	typeNote(m, "second thought on b")
	press(m, shiftS)

	sent := fakes["s1"].Sent
	if len(sent) != 2 {
		t.Fatalf("SendInput called %d times, want 2", len(sent))
	}
	if !strings.Contains(sent[1], "Review comments (1):") || !strings.Contains(sent[1], "second thought on b") {
		t.Errorf("the second submit is not the one new comment:\n%s", sent[1])
	}
	if strings.Contains(sent[1], "was this needed?") {
		t.Errorf("the second submit resent a comment already sent:\n%s", sent[1])
	}
}

// The title's count is what S would send.
func TestModel_diffTitleCountsOnlyPendingComments_issue335(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 3)
	typeNote(m, "was this needed?")
	press(m, shiftS)

	refocusColumn(m)
	down(m, 3)
	typeNote(m, "second thought on b")
	// Wide enough that the title keeps its count; at 100 columns the column
	// gives the count up before the no-tests flag (#283).
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})

	body := m.View().Content
	if !strings.Contains(body, "1 comments") || strings.Contains(body, "2 comments") {
		t.Errorf("the title does not count the one pending comment:\n%s", body)
	}
	if m.PendingComments() != 1 {
		t.Errorf("PendingComments() = %d, want 1", m.PendingComments())
	}
}

// When the line a sent comment was on is gone, the comment goes too: it asked
// about code that no longer exists. A pending comment in the same place stays,
// floated up as moved, because nobody has read it yet (#22).
func TestModel_aSentCommentWhoseLineIsGoneIsDropped_issue335(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 3)
	typeNote(m, "sent b")
	press(m, shiftS)
	refocusColumn(m)
	down(m, 3)
	typeNote(m, "pend b")

	edited, err := review.ParseDiff(strings.NewReader(strings.Replace(sampleDiff, "-\tb := 2", "-\tb := 7", 1)))
	if err != nil {
		t.Fatal(err)
	}
	m.Update(ui.DiffLoadedMsg{SessionID: "s1", Diff: edited})

	body := m.View().Content
	if strings.Contains(body, "sent b") {
		t.Errorf("a sent comment whose line is gone is still drawn:\n%s", body)
	}
	if !strings.Contains(body, "(moved) pend b") {
		t.Errorf("the pending comment whose line is gone is not shown as moved:\n%s", body)
	}
}

// S on a gate report whose failures already went warns rather than sending
// them again; S once more resends, and a fresh report sends on the first S.
func TestModel_gateSWarnsBeforeResendingTheSameFailures_issue335(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	m.SetGateReport("s1", failingReport())
	leader(m, key('g'))

	press(m, key('S'))
	refocusGate(m)
	press(m, key('S'))

	if n := len(fakes["s1"].Sent); n != 1 {
		t.Fatalf("the second S sent again: %d messages, want 1", n)
	}
	if body := m.View().Content; !strings.Contains(body, "already sent") {
		t.Errorf("the second S does not say the failures were already sent:\n%s", body)
	}

	press(m, key('S'))
	if n := len(fakes["s1"].Sent); n != 2 {
		t.Fatalf("S after the warning did not resend: %d messages, want 2", n)
	}

	m.Update(ui.GateMsg(failingReport()))
	refocusGate(m)
	press(m, key('S'))
	if n := len(fakes["s1"].Sent); n != 3 {
		t.Errorf("a fresh report's failures did not send on the first S: %d messages, want 3", n)
	}
}
