package crap_test

import (
	"math"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/coverage"
	"github.com/WilsonSousajr/omatty/internal/crap"
	"github.com/WilsonSousajr/omatty/internal/golist"
)

// The published formula, at the points that decide the gate's threshold.
func TestValue_matchesThePublishedFormula(t *testing.T) {
	cases := []struct {
		name               string
		cc, covered, stmts int
		want               float64
	}{
		{"trivial and fully covered", 1, 1, 1, 1},
		{"untested with three branches", 3, 0, 4, 12},
		{"at the gocyclo cap, at the coverage floor", 10, 9, 10, 10.1},
		{"at the gocyclo cap, untested", 10, 0, 10, 110},
		{"the canonical threshold is five branches untested", 5, 0, 5, 30},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := crap.Score{Complexity: c.cc, Covered: c.covered, Statements: c.stmts}

			if got := s.Value(); math.Abs(got-c.want) > 0.001 {
				t.Errorf("Value() = %.3f, want %.3f", got, c.want)
			}
		})
	}
}

// go tool cover reports percent(0, 0) as 0.0%, which is why this package reads
// the profile's blocks instead of its text: a function with no statements has
// nothing to test, and listing it as untested would train the eye to ignore
// the report.
func TestCoverage_aFunctionWithNoStatementsIsNotUntested(t *testing.T) {
	s := crap.Score{Complexity: 1, Statements: 0, Covered: 0}

	if got := s.Coverage(); got != 1 {
		t.Errorf("Coverage() = %v, want 1 for a function with no statements", got)
	}
	if got := s.Value(); got != 1 {
		t.Errorf("Value() = %v, want the bare complexity, not a penalty", got)
	}
}

// The node set must match fzipp/gocyclo's, because .golangci.yml already
// enforces gocyclo at 10: two tools disagreeing about the same function is the
// fastest way to make a metric ignorable.
func TestScores_complexityMatchesGocyclosNodeSet(t *testing.T) {
	got := scoreSample(t, nil)

	for name, want := range map[string]int{
		"simple": 1, "branchy": 5, "withClosure": 3, "empty": 1,
	} {
		if s, ok := got[name]; !ok {
			t.Errorf("%s was never scored", name)
		} else if s.Complexity != want {
			t.Errorf("%s complexity = %d, want %d", name, s.Complexity, want)
		}
	}
}

// A package absent from the profile is untested, not unknown. Reporting
// nothing for it would hide exactly the code the metric exists to find.
func TestScores_aFileWithNoProfileDataIsUntested(t *testing.T) {
	got := scoreSample(t, nil)

	if s := got["branchy"]; s.Coverage() != 0 {
		t.Errorf("branchy coverage = %v, want 0 when the profile says nothing", s.Coverage())
	}
	if s, want := got["branchy"], 30.0; s.Value() != want {
		t.Errorf("branchy CRAP = %v, want %v", s.Value(), want)
	}
}

// Attribution is by extent, not by line, so a block belongs to the function
// whose span contains it.
func TestScores_attributesBlocksToTheFunctionThatContainsThem(t *testing.T) {
	blocks := []coverage.Block{
		{Path: "sample/sample.go", StartLine: 7, StartCol: 20, EndLine: 7, EndCol: 33, NumStmt: 1, Covered: true},
		{Path: "sample/sample.go", StartLine: 11, StartCol: 18, EndLine: 12, EndCol: 11, NumStmt: 1, Covered: false},
	}

	got := scoreSample(t, blocks)

	if s := got["simple"]; s.Statements != 1 || s.Covered != 1 {
		t.Errorf("simple = %d/%d statements covered, want 1/1", s.Covered, s.Statements)
	}
	if s := got["branchy"]; s.Covered != 0 || s.Statements == 0 {
		t.Errorf("branchy = %d/%d, want some statements and none covered", s.Covered, s.Statements)
	}
}

// Worst first: a gate's output is read from the top, and a stable order keeps
// two runs of an unchanged tree comparable.
func TestScores_comeBackWorstFirst(t *testing.T) {
	got, err := crap.Scores("m", samplePackage(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(got); i++ {
		if got[i-1].Value() < got[i].Value() {
			t.Fatalf("scores are not sorted: %v before %v", got[i-1], got[i])
		}
	}
}

// Report must fail the run on a score that reaches the threshold, and say
// which function did it - a gate that only prints a number sends a session
// looking for the problem.
func TestReport_namesWhatCrossedTheThreshold(t *testing.T) {
	scores, err := crap.Scores("m", samplePackage(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder

	over := crap.Report(&out, scores, 15)

	if over != 1 {
		t.Errorf("Report() = %d over the threshold, want 1 (branchy at 30)", over)
	}
	if !strings.Contains(out.String(), "branchy") {
		t.Errorf("report does not name the offending function:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "15") {
		t.Errorf("report does not name the threshold:\n%s", out.String())
	}
}

// A clean run still prints the worst offenders: the number people act on is
// the one just under the line, not the verdict.
func TestReport_showsTheWorstEvenWhenEverythingPasses(t *testing.T) {
	scores, err := crap.Scores("m", samplePackage(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder

	if over := crap.Report(&out, scores, 999); over != 0 {
		t.Fatalf("Report() = %d over a 999 threshold, want 0", over)
	}
	if !strings.Contains(out.String(), "branchy") {
		t.Errorf("a passing report hides the worst function:\n%s", out.String())
	}
}

// A package whose source will not parse is an error, not a silent zero: a
// scorer that quietly skips what it cannot read reports a clean tree.
func TestScores_unreadableSourceIsAnError(t *testing.T) {
	pkgs := []golist.Package{{
		ImportPath: "m/missing", Dir: "testdata/missing", GoFiles: []string{"nope.go"},
	}}

	_, err := crap.Scores("m", pkgs, nil)

	if err == nil {
		t.Fatal("Scores() error = nil, want the unreadable package surfaced")
	}
	if !strings.Contains(err.Error(), "crap:") {
		t.Errorf("error = %q, want it to name the package doing the work", err)
	}
}

// samplePackage points at the fixture the way golist would describe it.
func samplePackage() []golist.Package {
	return []golist.Package{{
		ImportPath: "m/sample", Dir: "testdata/sample", GoFiles: []string{"sample.go"},
	}}
}

// scoreSample scores the fixture and keys the result by function name.
func scoreSample(t *testing.T, blocks []coverage.Block) map[string]crap.Score {
	t.Helper()
	scores, err := crap.Scores("m", samplePackage(), blocks)
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]crap.Score{}
	for _, s := range scores {
		byName[s.Name] = s
	}
	return byName
}
