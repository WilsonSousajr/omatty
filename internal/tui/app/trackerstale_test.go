package app_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// errUnauthorised is a refresh the forge refused, as gh words it.
var errUnauthorised = errors.New("gh: HTTP 401: Bad credentials")

// staleTracker is the tracker read once, then four minutes later a refresh
// that fails: the last list is still on screen.
func staleTracker(t *testing.T) (*app.Model, *FakeIssues, *FakePRs) {
	t.Helper()
	m, fi, fp := trackerModel(t)
	openTracker(m)
	fi.Now = fi.Now.Add(4 * time.Minute)
	fi.Errs["/p/omatty"], fp.Errs["/p/omatty"] = errUnauthorised, errUnauthorised
	pressDeliver(m, key('r'))
	return m, fi, fp
}

// A refresh that fails says why, how old the list on screen is and how to
// try again, not only a ? in the title: the ? says "stale" but not whether it
// is a blip or a CLI that needs logging in (#776).
func TestTracker_refreshFailureShowsCause_issue776(t *testing.T) {
	m, _, _ := staleTracker(t)

	frame := stripSGR(m.View().Content)
	for _, want := range []string{"HTTP 401", "4m old", "r retries", "#399"} {
		if !strings.Contains(frame, want) {
			t.Errorf("the stale tracker does not show %q:\n%s", want, frame)
		}
	}
	if title := columnTitle(m); !strings.Contains(title, "?") {
		t.Errorf("the title lost its ?: %q", title)
	}
}

// A refresh that answers again takes the notice away (#776).
func TestTracker_aGoodRefreshClearsTheCause_issue776(t *testing.T) {
	m, fi, fp := staleTracker(t)
	delete(fi.Errs, "/p/omatty")
	delete(fp.Errs, "/p/omatty")
	pressDeliver(m, key('r'))

	if frame := stripSGR(m.View().Content); strings.Contains(frame, "HTTP 401") {
		t.Errorf("the cause outlived a good refresh:\n%s", frame)
	}
}

// The notice takes rows from the list, so the cursor still stays on screen and
// a click still lands on the row drawn under it (#776).
func TestTracker_theCauseLeavesTheCursorOnScreen_issue776(t *testing.T) {
	m, _, _ := staleTracker(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 9}) // fewer rows than the list
	for range 10 {
		press(m, key('j'))
	}
	if frame := stripSGR(m.View().Content); !strings.Contains(frame, "#402") {
		t.Errorf("the last row is under the notice's rows, off screen:\n%s", frame)
	}
}

// A click lands on the row drawn under it, not the row the notice's height
// above it would have been (#776).
func TestTracker_aClickUnderTheCauseSelectsThatRow_issue776(t *testing.T) {
	m, _, _ := staleTracker(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	y := -1
	for i, line := range frameLines(m) {
		if strings.Contains(stripSGR(line), "#396") {
			y = i
		}
	}
	if y < 0 {
		t.Fatalf("#396 is not on screen:\n%s", stripSGR(m.View().Content))
	}

	m.Update(clickAt(reviewHairlineX+5, y))

	if got := m.TrackerCursor(); got != 1 {
		t.Errorf("a click on #396, the second issue, left the cursor at %d", got)
	}
}

// The cause is a forge CLI's stderr, which can carry a server's words: an
// escape sequence in it is drawn as text, never handed to the terminal, where
// it could retitle the window, move the cursor or rewrite the screen (#776's
// review).
func TestTracker_refreshCauseCannotDrawEscapes_issue776(t *testing.T) {
	m, fi, fp := trackerModel(t)
	openTracker(m)
	hostile := errors.New("HTTP 502 \x1b]0;owned\x07\x1b[2J\x9b31m\rgone")
	fi.Errs["/p/omatty"], fp.Errs["/p/omatty"] = hostile, hostile
	pressDeliver(m, key('r'))

	frame := m.View().Content
	for _, bad := range []string{"\x1b]0;", "\x07", "\x1b[2J", "\x9b", "\r"} {
		if strings.Contains(frame, bad) {
			t.Errorf("the frame carries the cause's control sequence %q", bad)
		}
	}
	if !strings.Contains(stripSGR(frame), "HTTP 502") {
		t.Errorf("the cause's words are gone with its escapes:\n%s", stripSGR(frame))
	}
}
