package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// modelShowingDiff opens the review column on a diff parsed from raw, at a
// width where the title is not cut - the flag is what is under test, not how a
// narrow column truncates it.
func modelShowingDiff(t *testing.T, raw string) *ui.Model {
	t.Helper()
	d, err := review.ParseDiff(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parsing the fixture: %v", err)
	}
	terms, _ := fakeTerms(t)
	deps := baseDeps(twoProjectState(), terms)
	deps.Diff = (&diffRecorder{Diff: d}).fn
	m := ui.NewModel(deps)
	m.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	leader(m, key('d'))
	return m
}

// diffOf builds a unified diff that adds the given lines to each path, which
// is all Pair needs to answer for every language but Rust.
func diffOf(files map[string][]string) string {
	var b strings.Builder
	for path, added := range files {
		b.WriteString("diff --git a/" + path + " b/" + path + "\n")
		b.WriteString("--- a/" + path + "\n+++ b/" + path + "\n")
		b.WriteString("@@ -1,0 +1," + itoa(len(added)) + " @@\n")
		for _, l := range added {
			b.WriteString("+" + l + "\n")
		}
	}
	return b.String()
}

func itoa(n int) string { return string(rune('0' + n)) }

// The flag is on the screen the operator already reads. Only for Unpaired:
// a remark that appeared on most diffs would be decoration.
func TestModel_anUnpairedDiffSaysSoInTheTitle_issue257(t *testing.T) {
	m := modelShowingDiff(t, diffOf(map[string][]string{"internal/gate/run.go": {"x := 1"}}))

	title := lineWith(t, m.View().Content, "diff ·")

	if !strings.Contains(title, "no tests") {
		t.Errorf("title does not carry the flag:\n%s", title)
	}
	if !strings.Contains(title, "1 files · 0 comments") {
		t.Errorf("the flag replaced the counts rather than joining them:\n%s", title)
	}
}

// Paired, None and Unknown say nothing at all. Three quarters of the outcomes
// are silence, which is what keeps the fourth worth reading.
func TestModel_onlyUnpairedRaisesTheFlag_issue257(t *testing.T) {
	cases := map[string]map[string][]string{
		"source with its test":    {"internal/gate/run.go": {"x := 1"}, "internal/gate/run_test.go": {"// t"}},
		"docs only":               {"README.md": {"hello"}},
		"a language with no rule": {"src/Cart.java": {"int x = 1;"}},
	}
	for name, files := range cases {
		t.Run(name, func(t *testing.T) {
			m := modelShowingDiff(t, diffOf(files))

			if title := lineWith(t, m.View().Content, "diff ·"); strings.Contains(title, "no tests") {
				t.Errorf("the flag was raised on a %s diff:\n%s", name, title)
			}
		})
	}
}

// Rust's in-file tests, on the screen: a .rs change that added #[cfg(test)]
// must not raise the flag, or the eye learns to skip it.
func TestModel_aRustDiffWithInFileTestsDoesNotRaiseTheFlag_issue257(t *testing.T) {
	m := modelShowingDiff(t, diffOf(map[string][]string{
		"src/cart.rs": {"fn total() -> u32 { 1 }", "#[cfg(test)]", "mod tests {"},
	}))

	if title := lineWith(t, m.View().Content, "diff ·"); strings.Contains(title, "no tests") {
		t.Errorf("a Rust diff that tested itself raised the flag:\n%s", title)
	}
}

// It is a remark, not a gate. Nothing is blocked and nothing turns red -
// M9's line holds: omatty reports, the operator decides.
func TestModel_theFlagBlocksNothing_issue257(t *testing.T) {
	m := modelShowingDiff(t, diffOf(map[string][]string{"internal/gate/run.go": {"x := 1"}}))

	body := m.View().Content

	if !strings.Contains(body, "internal/gate/run.go") {
		t.Errorf("the diff itself is not drawn:\n%s", body)
	}
	if strings.Contains(body, "blocked") || strings.Contains(body, "refus") {
		t.Errorf("the flag reads as a verdict rather than a remark:\n%s", body)
	}
}

// The flag carries the only remark M10 makes about a whole diff, and a
// straight cut from the right took it out at the *default* window size: the
// review column is 27 cells there, and `diff · 2 files · 0 comments · ⚠ no
// tests` was drawn as `⚠ no` (#283). So the title gives up whole parts, in a
// stated order, instead of being cut mid-word.
//
// The widths are measured, not guessed: a 100-cell window gives the column 27
// cells and a 120-cell window gives it 35, less one for the space the header
// row puts in front of every title.
func TestModel_theTitleGivesUpPartsInOrder_issue283(t *testing.T) {
	cases := []struct {
		name   string
		window int
		want   string
	}{
		{
			name:   "room for everything",
			window: 160,
			want:   "diff · 1 files · 0 comments · ⚠ no tests",
		},
		{
			name:   "the absence of news goes first",
			window: 120,
			want:   "diff · 1 files · ⚠ no tests",
		},
		{
			name:   "then the file count, at the default window",
			window: 100,
			want:   "diff · ⚠ no tests",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := modelShowingDiff(t, diffOf(map[string][]string{"internal/gate/run.go": {"x := 1"}}))
			m.Update(tea.WindowSizeMsg{Width: c.window, Height: 30})

			title := lineWith(t, m.View().Content, "diff ·")

			if !strings.Contains(title, c.want) {
				t.Errorf("at %d columns the title does not read %q:\n%s", c.window, c.want, title)
			}
		})
	}
}

// Unsent comments are state the operator needs; a file count is context. So
// when both cannot fit, the file count is the one that goes.
func TestModel_aNarrowColumnKeepsQueuedCommentsOverTheFileCount_issue283(t *testing.T) {
	m := modelShowingDiff(t, diffOf(map[string][]string{"internal/gate/run.go": {"x := 1"}}))
	down(m, 2)
	typeNote(m, "this one")
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	title := lineWith(t, m.View().Content, "diff ·")

	if !strings.Contains(title, "diff · 1 comments · ⚠ no tests") {
		t.Errorf("a queued comment was given up before the file count:\n%s", title)
	}
}

// Parts are given up to make room for something, never on principle: with the
// whole title fitting, nothing is dropped and it reads in its documented order.
func TestModel_aWideColumnGivesUpNothing_issue283(t *testing.T) {
	m := modelShowingDiff(t, diffOf(map[string][]string{"internal/gate/run.go": {"x := 1"}}))

	title := lineWith(t, m.View().Content, "diff ·")

	if !strings.Contains(title, "diff · 1 files · 0 comments · ⚠ no tests") {
		t.Errorf("a wide column did not draw the whole title:\n%s", title)
	}
}

// The rule is about whole parts, not about the flag: a paired diff too narrow
// for its counts gives one up rather than being cut mid-word. `diff · 2 files`
// says something; `0 comment` says nothing and looks like a bug.
func TestModel_aTitleWithNoFlagIsStillGivenUpInWholeParts_issue283(t *testing.T) {
	m := modelShowingDiff(t, diffOf(map[string][]string{
		"internal/gate/run.go":      {"x := 1"},
		"internal/gate/run_test.go": {"// t"},
	}))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	title := lineWith(t, m.View().Content, "diff ·")

	if !strings.Contains(title, "diff · 2 files") || strings.Contains(title, "comment") {
		t.Errorf("the title was cut mid-word rather than given up whole:\n%s", title)
	}
}

// Giving parts up must not buy width back: the row is still exactly the frame.
func TestModel_theElidedTitleStillFitsItsColumn_issue283(t *testing.T) {
	m := modelShowingDiff(t, diffOf(map[string][]string{"internal/gate/run.go": {"x := 1"}}))
	for _, w := range []int{160, 120, 100, 92, 84, 76} {
		m.Update(tea.WindowSizeMsg{Width: w, Height: 30})

		for i, l := range strings.Split(m.View().Content, "\n") {
			if got := lipgloss.Width(l); got != w {
				t.Fatalf("width %d: row %d is %d cells, want %d:\n%s", w, i, got, w, l)
			}
		}
	}
}
