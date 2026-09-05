package fuzzy_test

import (
	"reflect"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/fuzzy"
)

func TestMatch_Subsequence(t *testing.T) {
	for _, tc := range []struct {
		name, query, s string
		want           bool
	}{
		{"exact", "main", "main", true},
		{"prefix", "par", "parser-fix", true},
		{"scattered", "psf", "parser-fix", true},
		{"case is ignored", "PSF", "parser-fix", true},
		{"query longer than the target", "parser-fix-extra", "parser-fix", false},
		{"right letters, wrong order", "fsp", "parser-fix", false},
		{"a letter that is not there", "psz", "parser-fix", false},
		{"empty query matches anything", "", "parser-fix", true},
		{"empty target matches only an empty query", "p", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, got := fuzzy.Match(tc.query, tc.s); got != tc.want {
				t.Errorf("Match(%q, %q) ok = %v, want %v", tc.query, tc.s, got, tc.want)
			}
		})
	}
}

// The whole point of scoring: initials should find the thing they initial,
// ahead of a target that merely contains the same letters further apart.
func TestMatch_ScoresTheTighterMatchLower(t *testing.T) {
	for _, tc := range []struct {
		name, query, better, worse string
	}{
		{"word boundaries beat a run of letters", "psf", "parser-fix", "prompts-final"},
		{"a prefix beats a late match", "api", "api-svc", "the-rapid-thing"},
		// Both of these match mid-word, so only the distance separates them.
		// Comparing "abacus" against "a-big-b" would not be meaningful: two
		// word-boundary hits are a good match, not a worse one.
		{"adjacent beats scattered", "ab", "abacus", "azzzb"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			good, ok := fuzzy.Match(tc.query, tc.better)
			if !ok {
				t.Fatalf("Match(%q, %q) did not match", tc.query, tc.better)
			}
			bad, ok := fuzzy.Match(tc.query, tc.worse)
			if !ok {
				t.Fatalf("Match(%q, %q) did not match", tc.query, tc.worse)
			}
			if good >= bad {
				t.Errorf("Match(%q, %q) = %d, want lower than Match(%q, %q) = %d",
					tc.query, tc.better, good, tc.query, tc.worse, bad)
			}
		})
	}
}

func TestMatch_AnExactMatchCostsNothing(t *testing.T) {
	if got, ok := fuzzy.Match("main", "main"); !ok || got != 0 {
		t.Errorf("Match(main, main) = (%d, %v), want (0, true)", got, ok)
	}
}

func TestRank_BestFirst(t *testing.T) {
	items := []string{"prompts-final", "parser-fix", "main"}

	got := fuzzy.Rank("psf", items)

	if want := []int{1, 0}; !reflect.DeepEqual(got, want) {
		t.Errorf("Rank(psf, %v) = %v, want %v (parser-fix before prompts-final, main dropped)",
			items, got, want)
	}
}

// An empty query must leave the list exactly as it was given: the switcher
// opens with one, and reordering the sidebar under the operator would be a
// surprise.
func TestRank_EmptyQueryKeepsTheGivenOrder(t *testing.T) {
	items := []string{"zeta", "alpha", "mid"}

	got := fuzzy.Rank("", items)

	if want := []int{0, 1, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("Rank(\"\", %v) = %v, want %v", items, got, want)
	}
}

// Equal scores must not reorder, or the list would shuffle as you type.
func TestRank_IsStableForEqualScores(t *testing.T) {
	items := []string{"ab", "ab", "ab"}

	got := fuzzy.Rank("ab", items)

	if want := []int{0, 1, 2}; !reflect.DeepEqual(got, want) {
		t.Errorf("Rank(ab, %v) = %v, want %v", items, got, want)
	}
}

func TestRank_NoMatchesIsEmpty(t *testing.T) {
	if got := fuzzy.Rank("zzz", []string{"main", "parser-fix"}); len(got) != 0 {
		t.Errorf("Rank(zzz, ...) = %v, want no matches", got)
	}
}

// Regression, issue #42: Match charges nothing for what follows the matched
// run, so an exact title and a longer one that merely contains it both scored
// 0 and the winner was whichever came first. Typing a session's full name and
// pressing enter jumped somewhere else.
func TestRank_AnExactMatchBeatsALongerOneContainingIt_issue42(t *testing.T) {
	items := []string{"maintenance", "main", "domain"}

	got := fuzzy.Rank("main", items)

	if len(got) == 0 || items[got[0]] != "main" {
		t.Errorf("Rank(%q, %v) put %q first, want %q", "main", items, items[got[0]], "main")
	}
}

// The tiebreak must not disturb a real score difference: a tighter match on a
// longer string still beats a loose one on a short string.
func TestRank_TheLengthTiebreakOnlyBreaksTies_issue42(t *testing.T) {
	items := []string{"p-s-f", "parser-fix"}

	got := fuzzy.Rank("psf", items)

	if len(got) == 0 || items[got[0]] != "p-s-f" {
		t.Errorf("Rank(%q, %v) put %q first, want the tighter match %q",
			"psf", items, items[got[0]], "p-s-f")
	}
}
