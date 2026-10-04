package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// filterModel's rows are the issues #399 #369 #315, then the pull request #405.

// tab on an issue folds the issues to their heading with its count, and the
// cursor goes to the pull requests that were buried under them (#663).
func TestTracker_tabFoldsTheIssuesUnderTheirCount_issue663(t *testing.T) {
	m := filterModel(t)

	pressDeliver(m, special(tea.KeyTab))

	body := stripSGR(trackerBody(t, m))
	if !strings.Contains(body, "issues (3) ▸") || strings.Contains(body, "#399") {
		t.Errorf("tab did not fold the issues to a counted heading:\n%s", body)
	}
	if got := m.TrackerAtCursor(); got != "#405" {
		t.Errorf("cursor on %q after folding, want #405, the first pull request", got)
	}
}

// Folding the last section moves the cursor up to the one before; a folded
// heading is a stop, since tab on it is what opens it again.
func TestTracker_tabFoldsAndOpensEachSection_issue663(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, key(']'))

	pressDeliver(m, special(tea.KeyTab))
	if got := m.TrackerAtCursor(); got != "#399" {
		t.Errorf("cursor on %q after folding the pull requests, want #399", got)
	}
	pressDeliver(m, special(tea.KeyTab))
	if got := m.TrackerAtCursor(); got != "pull requests (1) ▸" {
		t.Errorf("cursor on %q with both folded, want the next heading, the pull requests'", got)
	}
	pressDeliver(m, special(tea.KeyTab))
	if got := m.TrackerAtCursor(); got != "#405" {
		t.Errorf("cursor on %q after opening the pull requests, want #405", got)
	}
}

// A poll that brings a new list does not unfold anything (#663).
func TestTracker_aFoldSurvivesAPoll_issue663(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, special(tea.KeyTab))

	m.Update(app.IssuesLoadedMsg{Project: "omatty", Issues: []forge.Issue{{Number: 663, Title: "fold a section", Updated: fixedNow}}})

	if body := stripSGR(trackerBody(t, m)); !strings.Contains(body, "issues (1) ▸") {
		t.Errorf("a poll unfolded the issues:\n%s", body)
	}
}

// Moving to another project and back keeps the fold, as the tracker keeps its
// place; nothing is stored, so a restart opens everything (#663).
func TestTracker_aFoldSurvivesAProjectSwitch_issue663(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, special(tea.KeyTab))

	leader(m, key(']')) // to api-svc; the tracker follows
	leader(m, key('[')) // and back

	if body := stripSGR(trackerBody(t, m)); !strings.Contains(body, "issues (3) ▸") {
		t.Errorf("the fold did not survive a look at another project:\n%s", body)
	}
}

// A filter searches a folded section too: its heading shows the filtered
// count, and it stays folded (#663). "w" keeps #369 and #405.
func TestTracker_aFoldedSectionCountsUnderAFilter_issue663(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, special(tea.KeyTab))

	pressDeliver(m, key('/'))
	typeInto(m, "w")

	if body := stripSGR(trackerBody(t, m)); !strings.Contains(body, "issues (1) ▸") || strings.Contains(body, "#369") {
		t.Errorf("the folded heading does not carry the filtered count:\n%s", body)
	}
}

// ] and [ skip a folded section's items and stop at its heading (#663).
func TestTracker_bracketsStopAtAFoldedHeading_issue663(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, key(']'))
	pressDeliver(m, special(tea.KeyTab)) // folds the pull requests; cursor on #399

	pressDeliver(m, key(']'))

	if got := m.TrackerAtCursor(); !strings.HasPrefix(got, "pull requests (1) ▸") {
		t.Errorf("] landed on %q, want the folded pull requests heading", got)
	}
}

// A folded heading the cursor rests on is drawn as the cursor row: it is the
// one heading tab acts on (#663).
func TestTracker_aFoldedHeadingUnderTheCursorIsHighlighted_issue663(t *testing.T) {
	m := filterModel(t)
	pressDeliver(m, key(']'))
	pressDeliver(m, special(tea.KeyTab)) // pull requests folded, cursor on #399
	before := lineWith(t, m.View().Content, "pull requests (1)")

	pressDeliver(m, key(']'))

	if after := lineWith(t, m.View().Content, "pull requests (1)"); after == before {
		t.Errorf("the folded heading under the cursor is drawn as it was without it: %q", after)
	}
}
