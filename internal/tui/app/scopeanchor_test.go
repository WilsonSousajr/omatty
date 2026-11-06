package app_test

import "testing"

// sampleDiff's whole-session rows: 0 model.go, 1 hunk, 2 a := 1, 3 -b := 2,
// 4 +b := 3, 5 +c := 4, 6 return, 7 }, 8 new.txt, 9 hunk, 10 +fresh,
// 11 +file. turnDiffText's: 0 model.go, 1 hunk, 2 a := 1, 3 b := 3,
// 4 +c := 4, 5 return, 6 }.

// Regression, issue #765: t reset the cursor to the top on every flip, so
// reading a line and asking "did this turn do that?" meant scrolling back to
// it each time. A line in both scopes keeps the cursor, both ways.
func TestTurn_toggleKeepsCursorAnchor_issue765(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})
	down(m, 5) // +c := 4

	pressAndSettle(m, key('t'))
	if got := m.ReviewCursor(); got != 4 {
		t.Fatalf("after t the cursor is on row %d, want 4: the same c := 4 in the turn", got)
	}

	pressAndSettle(m, key('t'))
	if got := m.ReviewCursor(); got != 5 {
		t.Errorf("after t back the cursor is on row %d, want 5: the same c := 4 in the session", got)
	}
}

// Issue #765: a context line the turn shows too is the same line, even though
// the session diff saw it added.
func TestTurn_toggleKeepsALineTheTurnShowsAsContext_issue765(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})
	down(m, 4) // +b := 3, context in the turn

	pressAndSettle(m, key('t'))

	if got := m.ReviewCursor(); got != 3 {
		t.Errorf("after t the cursor is on row %d, want 3: b := 3 as the turn shows it", got)
	}
}

// Issue #765: a file the turn did not touch has no rows in it, so the cursor
// goes to the nearest file there is rather than the top of nothing.
func TestTurn_toggleOnAnUntouchedFileGoesToTheNearestFile_issue765(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})
	down(m, 10) // +fresh in new.txt

	pressAndSettle(m, key('t'))

	if got := m.ReviewCursor(); got != 0 {
		t.Errorf("after t the cursor is on row %d, want 0: model.go, the only file the turn has", got)
	}
}
