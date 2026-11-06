package app_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

// Regression, issue #762: between ctrl+o d and the first DiffLoadedMsg the
// column drew "no changes" for a session with real changes - a wrong answer,
// shown first, about the one thing the column exists to show. Until the load
// answers it says it is loading.
func TestReview_noChangesNotShownBeforeLoad_issue762(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	press(m, ctrl('o'))
	m.Update(key('d')) // the load is scheduled and never delivered

	got := stripSGR(m.View().Content)

	if strings.Contains(got, "no changes") {
		t.Errorf("the column says \"no changes\" before the diff has loaded:\n%s", got)
	}
	if !strings.Contains(got, "loading the diff") {
		t.Errorf("the column does not say it is loading:\n%s", got)
	}
}

// Issue #762: "no changes" is the answer to a load that came back empty, and
// only that.
func TestReview_noChangesShownAfterAnEmptyLoad_issue762(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	press(m, ctrl('o'))
	_, cmd := m.Update(key('d'))
	_ = cmd // dropped: the answer below is the one that lands
	m.Update(app.DiffLoadedMsg{SessionID: m.Selected(), Seq: m.DiffSeq()})

	if got := stripSGR(m.View().Content); !strings.Contains(got, "no changes") {
		t.Errorf("an empty load does not say \"no changes\":\n%s", got)
	}
}
