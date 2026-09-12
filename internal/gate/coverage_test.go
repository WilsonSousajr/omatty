package gate_test

import (
	"context"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/gate"
)

// Real output from the tools a gate is likely to name. Recorded rather than
// invented: the shapes differ more than they look, and a parser written
// against imagined output is a parser tested against itself.
func TestRun_coverageStep_readsThePercentage(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want float64
	}{
		{"go tool cover total", "total:\t\t\t(statements)\t92.6%", 92.6},
		{"omatty's own gate script", "coverage 92.4% meets the 90% gate", 92.4},
		{"go test -cover per package", "ok  \tgithub.com/x/y\t1.2s\tcoverage: 88.4% of statements", 88.4},
		{"istanbul table", "All files |   83.12 |    71.4 |   90.0 |   83.12 |", 83.12},
		{"pytest-cov", "TOTAL                      1204    103    91%", 91},
		{"tarpaulin", "|| Tested/Total Lines:\n|| \n79.55% coverage, 245/308 lines covered", 79.55},
		{"a whole number", "coverage: 100% of statements", 100},
		{"the last percentage wins over an earlier one", "threshold 90%\ntotal:\t(statements)\t93.1%", 93.1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			steps := []gate.Step{{Name: "cov", Kind: gate.KindCoverage, Run: "printf '%s' " + shellQuote(c.out)}}

			got, err := gate.Run(context.Background(), t.TempDir(), steps)
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if got[0].Percent != c.want {
				t.Errorf("Percent = %v, want %v\noutput was:\n%s", got[0].Percent, c.want, c.out)
			}
		})
	}
}

// An ordinary step is not searched for a percentage. A test that prints "85%
// done" is not a coverage report, and reading it as one would put a number on
// the card that means nothing.
func TestRun_ordinaryStep_isNotSearchedForAPercentage(t *testing.T) {
	steps := []gate.Step{{Name: "test", Run: "echo 'progress: 85% done'"}}

	got, _ := gate.Run(context.Background(), t.TempDir(), steps)

	if got[0].Percent != 0 {
		t.Errorf("Percent = %v, want 0: only a coverage step is parsed", got[0].Percent)
	}
}

// Invariant 12. The percentage is for display; the exit code decides. A step
// that cannot be parsed still reports the verdict its command asserted, rather
// than failing a run the tool itself said had passed.
func TestRun_coverageStepWithNoParseablePercentage_stillPasses(t *testing.T) {
	steps := []gate.Step{{Name: "cov", Kind: gate.KindCoverage, Run: "echo 'no numbers here'"}}

	got, _ := gate.Run(context.Background(), t.TempDir(), steps)

	if got[0].Verdict != gate.Pass {
		t.Errorf("verdict = %v, want Pass: a parse miss must not fail the run", got[0].Verdict)
	}
	if got[0].Percent != 0 {
		t.Errorf("Percent = %v, want 0", got[0].Percent)
	}
}

// And the reverse: a parseable percentage does not rescue a failing command.
func TestRun_coverageStepThatFails_isStillAFailure(t *testing.T) {
	steps := []gate.Step{{Name: "cov", Kind: gate.KindCoverage, Run: "echo 'coverage 71.0% is below the 90% gate'; exit 1"}}

	got, _ := gate.Run(context.Background(), t.TempDir(), steps)

	if got[0].Verdict != gate.Fail {
		t.Errorf("verdict = %v, want Fail: the exit code decides", got[0].Verdict)
	}
	if got[0].Percent != 90 && got[0].Percent != 71 {
		t.Logf("Percent = %v (either reading is defensible here)", got[0].Percent)
	}
}
