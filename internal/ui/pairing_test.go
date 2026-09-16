package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

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
