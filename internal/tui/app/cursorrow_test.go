package app_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/tui/app"
)

var foregroundSGR = regexp.MustCompile(`38;[0-9;]+`)

// rowWith is the frame's first line that holds text, as drawn.
func rowWith(t *testing.T, m *app.Model, text string) string {
	t.Helper()
	for _, l := range frameLines(m) {
		if strings.Contains(stripSGR(l), text) {
			return l
		}
	}
	t.Fatalf("no row holds %q:\n%s", text, m.View().Content)
	return ""
}

// Regression, issue #763: the cursor row was drawn as plain reverse video,
// before the path that applies syntax and changed-word colours, so the row
// being read - and about to be commented on - was the one row with none. It
// keeps every foreground it has without the cursor.
func TestReview_cursorRowKeepsSyntax_issue763(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	down(m, 4) // +	b := 3
	withCursor := foregroundSGR.FindAllString(rowWith(t, m, "b := 3"), -1)

	down(m, 1)
	without := foregroundSGR.FindAllString(rowWith(t, m, "b := 3"), -1)

	if len(without) == 0 {
		t.Fatal("the row draws no foreground colour even without the cursor; the fixture proves nothing")
	}
	if !slices.Equal(withCursor, without) {
		t.Errorf("the cursor row's foregrounds %q differ from the row's own %q", withCursor, without)
	}
}
