package app_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
)

// Regression, issue #745: at claude's permission dialog the paste's closing
// \r selects the highlighted option, "Yes", so S approved the pending tool
// call while the comments were lost and marked sent. While the session waits
// on a permission, S sends nothing, keeps every comment, and says why.
func TestModel_SAtAPermissionPromptSendsNothing_issue745(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 3)
	typeNote(m, "was this needed?")
	sendStatus(m, "s1", dstatus.PermissionRequested, time.Now())

	press(m, tea.KeyPressMsg{Code: 's', Mod: tea.ModShift, Text: "S"})

	if n := len(fakes["s1"].Sent); n != 0 {
		t.Errorf("sent %d messages at a permission prompt, want none: %q", n, fakes["s1"].Sent)
	}
	if m.PendingComments() != 1 {
		t.Errorf("pending = %d, want the comment kept", m.PendingComments())
	}
	if !strings.Contains(m.View().Content, "permission") {
		t.Errorf("the operator is not told why nothing was sent:\n%s", m.View().Content)
	}
}

// Regression, issue #745: once the prompt is answered the session moves on,
// and S sends as it always did.
func TestModel_SSendsOnceThePermissionIsAnswered_issue745(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 3)
	typeNote(m, "was this needed?")
	sendStatus(m, "s1", dstatus.PermissionRequested, time.Now().Add(-time.Minute))
	sendStatus(m, "s1", dstatus.TurnEnded, time.Now())

	press(m, tea.KeyPressMsg{Code: 's', Mod: tea.ModShift, Text: "S"})

	if n := len(fakes["s1"].Sent); n != 1 {
		t.Errorf("sent %d messages after the prompt was answered, want 1", n)
	}
}

// Regression, issue #745: the gate's S writes the same paste and the same
// \r, so it approved the pending call too.
func TestModel_gateSAtAPermissionPromptSendsNothing_issue745(t *testing.T) {
	m, fakes, _ := modelWithDiff(t)
	m.SetGateReport("s1", failingReport())
	leader(m, key('g'))
	sendStatus(m, "s1", dstatus.PermissionRequested, time.Now())

	press(m, key('S'))

	if n := len(fakes["s1"].Sent); n != 0 {
		t.Errorf("sent %d gate messages at a permission prompt, want none", n)
	}
	if !strings.Contains(m.View().Content, "permission") {
		t.Errorf("the operator is not told why nothing was sent:\n%s", m.View().Content)
	}
}
