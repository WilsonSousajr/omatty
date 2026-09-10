package ui_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/ui"
)

// A session dtach was already holding comes back with a blank pane: dtach
// cleared it and claude, whose size did not change, repainted nothing. At
// boot the model asks such a terminal for a repaint, and only such a one:
// a fresh claude paints on its own (#191).
func TestModel_AHeldSessionIsRepaintedOnBoot_issue191(t *testing.T) {
	terms, fakes := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Reattached = map[string]bool{"s1": true}
	m := ui.NewModel(d)

	// Init alone: the Fake counts the call itself, and settling Init would
	// chase the once-a-second tick it also schedules.
	m.Init()

	if fakes["s1"].Repaints != 1 {
		t.Errorf("held session repainted %d times, want 1", fakes["s1"].Repaints)
	}
	for _, fresh := range []string{"s2", "s3"} {
		if fakes[fresh].Repaints != 0 {
			t.Errorf("fresh session %s repainted %d times, want 0", fresh, fakes[fresh].Repaints)
		}
	}
}

// ctrl+o r starts a new claude, which paints from scratch: no nudge.
func TestModel_ARestartDoesNotRepaint_issue191(t *testing.T) {
	s := &startRecorder{}
	m, _ := modelWithStarter(t, s)

	pressAndSettle(m, ctrl('o'))
	pressAndSettle(m, key('r'))

	if s.Term == nil {
		t.Fatal("restart started nothing")
	}
	if s.Term.Repaints != 0 {
		t.Errorf("the restarted terminal was repainted %d times, want 0", s.Term.Repaints)
	}
}
