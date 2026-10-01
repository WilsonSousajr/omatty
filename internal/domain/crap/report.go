package crap

import (
	"fmt"
	"io"
)

// worstShown is how many rows a passing run prints. The number a reader acts on
// is the one just under the line, not the verdict, so a green gate still shows
// where the tree is heading.
const worstShown = 10

// Report writes the scores and returns how many reached threshold.
//
//	over := crap.Report(os.Stdout, scores, 15)
//
// Reaching the threshold fails, rather than exceeding it: CC 3 at zero coverage
// is exactly 12.0 and CC 5 at zero coverage exactly 30.0, so a threshold lands
// on a real score often enough that leaving the boundary implicit would hide an
// off-by-one inside a float comparison.
func Report(w io.Writer, scores []Score, threshold float64) int {
	over := 0
	for _, s := range scores {
		if s.Value() >= threshold {
			over++
		}
	}
	writeRows(w, scores, threshold, over)
	writeVerdict(w, scores, threshold, over)
	return over
}

// writeRows prints the offenders, or the worst few when there are none.
func writeRows(w io.Writer, scores []Score, threshold float64, over int) {
	shown := over
	if shown == 0 {
		shown = min(worstShown, len(scores))
	}
	if shown == 0 {
		return
	}
	_, _ = fmt.Fprintf(w, "%-8s %4s %8s  %s\n", "CRAP", "CC", "COVER", "FUNCTION")
	for _, s := range scores[:shown] {
		mark := " "
		if s.Value() >= threshold {
			mark = "!"
		}
		_, _ = fmt.Fprintf(w, "%s%-7.1f %4d %7.1f%%  %s:%d %s\n",
			mark, s.Value(), s.Complexity, s.Coverage()*100, s.File, s.Line, s.Name)
	}
}

// writeVerdict states the outcome in the shape check-coverage.sh states its
// own, so the two gate steps read alike.
func writeVerdict(w io.Writer, scores []Score, threshold float64, over int) {
	worst := 0.0
	if len(scores) > 0 {
		worst = scores[0].Value()
	}
	if over > 0 {
		_, _ = fmt.Fprintf(w, "%d of %d functions reach the CRAP %.0f gate; worst is %.1f\n",
			over, len(scores), threshold, worst)
		return
	}
	_, _ = fmt.Fprintf(w, "worst CRAP %.1f over %d functions meets the %.0f gate\n",
		worst, len(scores), threshold)
}
