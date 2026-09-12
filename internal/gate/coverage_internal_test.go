package gate

import (
	"strings"
	"testing"
)

// The one fixture that is not recorded from someone else's tool: omatty's own
// gate line, whose output shape is the one this project will actually run.
func TestPercentIn_omattysOwnGateOutput(t *testing.T) {
	out := `ok  	github.com/WilsonSousajr/omatty/internal/vcs	2.646s	coverage: 94.9% of statements
ok  	github.com/WilsonSousajr/omatty/internal/watcher	(cached)	coverage: 91.6% of statements
ok  	github.com/WilsonSousajr/omatty/internal/watcher/e2e	(cached)	coverage: [no statements]
coverage 92.6% meets the 90% gate`

	if got := percentIn(out); got != 92.6 {
		t.Errorf("percentIn(omatty's gate output) = %v, want 92.6", got)
	}
}

// A step that is below its threshold still reports the coverage it reached,
// not the threshold it missed - the card shows where you are.
func TestPercentIn_belowThreshold_readsTheCoverageNotTheGate(t *testing.T) {
	if got := percentIn("coverage 71.0% is below the 90% gate"); got != 71 {
		t.Errorf("percentIn() = %v, want 71", got)
	}
}

// "[no statements]" lines carry a coverage word and no number. They must not
// shadow the real summary below them.
func TestPercentIn_noStatementsLine_doesNotShadowTheTotal(t *testing.T) {
	out := "ok\tx/e2e\t(cached)\tcoverage: [no statements]\ncoverage 88.1% meets the 90% gate"
	if got := percentIn(out); got != 88.1 {
		t.Errorf("percentIn() = %v, want 88.1", got)
	}
}

// A table whose first column is a count, not a percentage. The 100 bound is
// what stops "1204" being read as coverage; without it the card would show a
// number with no meaning.
func TestPercentIn_skipsNumbersThatCannotBePercentages(t *testing.T) {
	if got := percentIn("All files |   1204 |   83.12 |"); got != 83.12 {
		t.Errorf("percentIn() = %v, want 83.12", got)
	}
}

// A digit run long enough to overflow float64 parses to an error rather than a
// number. It must be skipped like any other implausible match, not abort the
// line.
func TestPercentIn_skipsAnUnparseableRunOfDigits(t *testing.T) {
	huge := strings.Repeat("9", 400)
	if got := percentIn("All files | " + huge + " | 77.5 |"); got != 77.5 {
		t.Errorf("percentIn() = %v, want 77.5", got)
	}
}
