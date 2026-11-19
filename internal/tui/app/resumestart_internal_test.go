package app

import "testing"

// Under lazy start the session --resume names is started like a held one
// (#815): opening omatty on a blank stopped pane would answer the hint with
// one more key to press.
func TestWithFocus_startsTheFocusedSession_issue815(t *testing.T) {
	got := WithFocus(SessionsToStart(true, threeSessions(), map[string]bool{}), "s2")

	if len(got) != 1 || !got["s2"] {
		t.Errorf("WithFocus(lazy, s2) = %v, want only s2", got)
	}
}

// Under lazy start SessionsToStart returns the held map itself, which the boot
// also hands the model as Reattached. Adding the focus to it in place would
// mark a fresh claude as reattached and nudge a pane that paints itself (#191).
func TestWithFocus_leavesTheHeldSetAlone_issue815(t *testing.T) {
	held := map[string]bool{"s1": true}

	got := WithFocus(SessionsToStart(true, threeSessions(), held), "s2")

	if held["s2"] {
		t.Errorf("held = %v after WithFocus; the focus leaked into the reattached set", held)
	}
	if !got["s1"] || !got["s2"] {
		t.Errorf("WithFocus = %v, want s1 and s2", got)
	}
}

// No --resume, no change.
func TestWithFocus_withoutAFocusIsTheSameSet_issue815(t *testing.T) {
	if got := WithFocus(map[string]bool{"s1": true}, ""); len(got) != 1 || !got["s1"] {
		t.Errorf("WithFocus(no focus) = %v, want [s1]", got)
	}
}
