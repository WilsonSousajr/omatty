package app_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
)

// ] from any issue lands on the first pull request, and [ from any pull
// request on the first issue: with many issues, the pull requests were a walk
// past every one of them away (#662). The items are #399 #369 #315 then #405.
func TestTracker_bracketsJumpBetweenTheSections_issue662(t *testing.T) {
	for _, from := range []int{0, 1, 2} {
		m := filterModel(t)
		for range from {
			pressDeliver(m, key('j'))
		}
		pressDeliver(m, key(']'))
		if got := m.TrackerCursor(); got != 3 {
			t.Errorf("] from issue %d landed on item %d, want 3, the first pull request", from, got)
		}
		pressDeliver(m, key('['))
		if got := m.TrackerCursor(); got != 0 {
			t.Errorf("[ from the pull request landed on item %d, want 0, the first issue", got)
		}
	}
}

// At the last section ] stays put, and [ at the first: no wrap, as the
// diff's ] and [ between files.
func TestTracker_bracketsStopAtTheEnds_issue662(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, key('['))
	if got := m.TrackerCursor(); got != 0 {
		t.Errorf("[ in the first section moved the cursor to %d, want 0", got)
	}
	pressDeliver(m, key('j'))
	pressDeliver(m, key('['))
	if got := m.TrackerCursor(); got != 0 {
		t.Errorf("[ from the second issue moved the cursor to %d, want the first issue", got)
	}
	pressDeliver(m, key(']'))
	pressDeliver(m, key(']'))
	if got := m.TrackerCursor(); got != 3 {
		t.Errorf("] in the last section moved the cursor to %d, want it held at 3", got)
	}
}

// Under a filter a section the filter emptied has no heading (#399) and is no
// stop: ] has nowhere to go and the cursor stays.
func TestTracker_bracketsOverAFilteredList_issue662(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, key('/'))
	typeInto(m, "brew")
	pressDeliver(m, special(tea.KeyEnter))

	pressDeliver(m, key(']'))

	if got := m.TrackerCursor(); got != 0 {
		t.Errorf("] with only #369 left moved the cursor to %d, want 0", got)
	}
}

// With one list there is one section and both keys do nothing.
func TestTracker_bracketsWithOnlyOneList_issue662(t *testing.T) {
	m, fi, _ := trackerModel(t)
	fi.Errs["/p/omatty"] = forge.ErrNoTracker
	openTracker(m)

	pressDeliver(m, key(']'))
	pressDeliver(m, key('['))

	if got := m.TrackerCursor(); got != 0 {
		t.Errorf("] and [ over one list moved the cursor to %d, want 0", got)
	}
}
