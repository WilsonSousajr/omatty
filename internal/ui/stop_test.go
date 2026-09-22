package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// stopRig is a model with every dependency stop and resume touch recorded:
// the held process's end, the tailer's stop, and the next start.
type stopRig struct {
	m      *ui.Model
	ended  *recordEnd
	tails  *recordArchive
	starts *startRecorder
	fakes  map[string]*termwrap.Fake
}

func newStopRig(t *testing.T) stopRig {
	t.Helper()
	terms, fakes := fakeTerms(t)
	r := stopRig{ended: &recordEnd{}, tails: &recordArchive{}, starts: &startRecorder{}, fakes: fakes}
	d := baseDeps(twoProjectState(), terms)
	d.Stop, d.TailStop, d.Start = r.ended.stop, r.tails.stopTail, r.starts.fn
	d.Clock = func() time.Time { return fixedNow }
	r.m = ui.NewModel(d)
	r.m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	return r
}

// stop presses ctrl+o s on the selected session, s1, and runs what it
// scheduled.
func (r stopRig) stop() {
	press(r.m, ctrl('o'))
	pressAndSettle(r.m, key('s'))
}

// ctrl+o s ends the process and closes the pane's terminal, and forgets
// nothing: the row stays, and so does its tailer, the only source of the
// card's status once the process is gone (#318).
func TestModel_stopEndsTheProcessAndKeepsTheSession_issue318(t *testing.T) {
	r := newStopRig(t)

	r.stop()

	if len(r.ended.Ended) != 1 || r.ended.Ended[0] != "s1" {
		t.Errorf("ended = %v, want exactly s1", r.ended.Ended)
	}
	if !r.fakes["s1"].Closed {
		t.Error("the stopped session's terminal was left open")
	}
	if len(r.tails.Stopped) != 0 {
		t.Errorf("stop ended the tailers of %v; the row is still there and needs its status", r.tails.Stopped)
	}
	if r.m.Selected() != "s1" {
		t.Errorf("selection moved to %q, want the stopped s1 still selected", r.m.Selected())
	}
}

// Invariant 1: a stopped pane is still the session's pane and still owns
// every key. Falling through to omatty's commands made a bare q quit (#318).
func TestModel_qOnAStoppedPaneDoesNotQuit_issue318(t *testing.T) {
	r := newStopRig(t)
	r.stop()

	if _, cmd := r.m.Update(key('q')); cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("a bare q on a stopped pane quit omatty")
		}
	}
	_, cmd := r.m.Update(ctrl('c'))
	if cmd == nil {
		t.Fatal("ctrl+c on a stopped pane did nothing, want it to quit (#28)")
	}
	if _, quit := cmd().(tea.QuitMsg); !quit {
		t.Error("ctrl+c on a stopped pane did not quit (#28)")
	}
}

// A paste with a stopped pane focused has no terminal to reach. It must be
// dropped, not dereferenced (#318).
func TestModel_pasteIntoAStoppedPaneIsDropped_issue318(t *testing.T) {
	r := newStopRig(t)
	r.stop()

	r.m.Update(tea.PasteMsg{Content: "hello"})

	if len(r.fakes["s1"].Sent) != 0 {
		t.Errorf("the paste reached the closed terminal: %q", r.fakes["s1"].Sent)
	}
}

// A stopped pane says so and says how to resume. The empty state would tell
// the operator to create a session while one sits selected (#318).
func TestModel_aStoppedPaneIsNotTheEmptyState_issue318(t *testing.T) {
	r := newStopRig(t)
	r.stop()

	got := r.m.View().Content
	if strings.Contains(got, "no sessions") || !strings.Contains(got, "stopped") || !strings.Contains(got, "enter") {
		t.Errorf("a stopped pane should say it is stopped and that enter resumes it:\n%s", got)
	}
}

// The card keeps what the transcript says - the glyph and the age are the
// operator's evidence for whether to resume it (#318, invariant 2).
func TestModel_aStoppedCardKeepsItsGlyphAndAge_issue318(t *testing.T) {
	r := newStopRig(t)
	r.m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.TurnEnded, At: fixedNow.Add(-3 * time.Hour)})

	r.stop()

	if row := rowOf(t, r.m, "main"); !strings.Contains(row, "✓") || !strings.Contains(row, "3h") {
		t.Errorf("a stopped card lost its glyph or age: %q", row)
	}
}

// enter resumes a stopped session exactly once; after that the pane is live
// and enter is claude's again (#318).
func TestModel_enterStartsAStoppedSessionOnce_issue318(t *testing.T) {
	r := newStopRig(t)
	r.stop()

	pressAndSettle(r.m, special(tea.KeyEnter))
	pressAndSettle(r.m, special(tea.KeyEnter))

	if len(r.starts.Started) != 1 || r.starts.Started[0] != "s1" {
		t.Fatalf("started = %v, want s1 exactly once", r.starts.Started)
	}
	if len(r.starts.Term.Msgs) != 1 {
		t.Errorf("the second enter reached the new terminal %d times, want once", len(r.starts.Term.Msgs))
	}
}

// Any other key on a stopped pane is swallowed: forwarded into a claude
// still starting, it would vanish (#318).
func TestModel_otherKeysOnAStoppedPaneStartNothing_issue318(t *testing.T) {
	r := newStopRig(t)
	r.stop()

	pressAndSettle(r.m, key('x'))

	if len(r.starts.Started) != 0 || strings.Contains(r.m.View().Content, "archive") {
		t.Errorf("x on a stopped pane did something: started %v", r.starts.Started)
	}
}

// No confirmation, as for restart - but the footer names the undo, and the
// cost when a turn is in flight (#318).
func TestModel_stopNamesTheUndoAndTheCost_issue318(t *testing.T) {
	r := newStopRig(t)
	r.m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.ToolStarted, At: fixedNow})

	r.stop()

	got := r.m.View().Content
	if !strings.Contains(got, "enter resumes") || !strings.Contains(got, "turn") {
		t.Errorf("the footer does not name the undo and the lost turn:\n%s", got)
	}
}
