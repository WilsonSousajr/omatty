package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// With both lists on screen each has a heading, and the cursor starts on the
// first issue rather than on the heading above it (#432).
func TestTracker_BothListsHaveAHeading_issue432(t *testing.T) {
	m, _, _ := trackerModel(t)
	openTracker(m)

	body := stripSGR(m.View().Content)
	if !strings.Contains(body, "── issues") || !strings.Contains(body, "── pull requests") {
		t.Errorf("the lists are not both labelled:\n%s", body)
	}
	if row := cursorRow(m); !strings.Contains(row, "#399") {
		t.Errorf("the cursor starts on %q, want the first issue", row)
	}
}

// A pull request's row carries three glyph cells - draft, CI, review - from the
// one state vocabulary (#425), and keeps them in a column too narrow for an
// issue's label: they are three cells, not ten.
func TestTracker_APullRequestRowCarriesStateCIAndReview_issue432(t *testing.T) {
	m, _, fp := trackerModel(t)
	fp.Lists["/p/omatty"][0].Review = forge.ReviewApproved // #402, CI passing
	fp.Lists["/p/omatty"] = append(fp.Lists["/p/omatty"], forge.PR{
		Number: 405, Title: "a draft", State: forge.Open, Draft: true, CI: forge.CIRunning,
		Review: forge.ReviewChanges, Updated: fixedNow,
	})
	for _, width := range []int{120, 80} {
		m.Update(tea.WindowSizeMsg{Width: width, Height: 32})
		openTracker(m)

		if row := rowContaining(t, m, "#402"); !strings.Contains(row, " ✓✓ ") {
			t.Errorf("at %d: #402 (open, CI passing, approved) reads %q", width, row)
		}
		if row := rowContaining(t, m, "#405"); !strings.Contains(row, "◌◍✗") {
			t.Errorf("at %d: #405 (draft, CI running, changes requested) reads %q", width, row)
		}
		openTracker(m) // close again for the next width
	}
}
