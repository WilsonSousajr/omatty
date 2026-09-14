// Package crap scores every function by the C.R.A.P. metric - Change Risk
// Anti-Patterns - which is cyclomatic complexity weighted by how much of the
// function its tests actually reach.
//
//	scores, err := crap.Scores(modulePath, pkgs, blocks)
//	over := crap.Report(os.Stdout, scores, 15)
//
//	CRAP(f) = CC(f)² × (1 − coverage(f))³ + CC(f)
//
// It exists because a repo-wide coverage total hides the functions worth
// finding. omatty's gate was green at 90% while internal/watcher.PromptText, an
// exported function, had no test touching it at all (#262). A total cannot see
// that; a per-function score can.
//
// The shape of the formula is the argument for it. High coverage collapses the
// score to the bare complexity, so a branchy function that is thoroughly tested
// is not penalised. Low coverage multiplies, so the same function untested
// scores an order of magnitude worse. Complexity alone would flag the first;
// coverage alone would miss that the second is where the risk is.
package crap

// Score is one function's reading.
type Score struct {
	// Package is the import path the function was declared in.
	Package string
	// File is repo-relative, the path a person greps for.
	File string
	// Line is where the func keyword sits.
	Line int
	// Name is the function's own name. Methods carry no receiver, because
	// nothing here needs to resolve one and the file and line already
	// identify the declaration uniquely.
	Name string
	// Complexity is cyclomatic complexity, counted over the same nodes
	// fzipp/gocyclo counts so that this and the lint gate agree.
	Complexity int
	// Statements is how many statements the coverage profile attributes to
	// this function, and Covered how many of them ran.
	Statements, Covered int
}

// Coverage is the fraction of the function's statements that ran.
//
// A function with no statements is covered. `go tool cover -func` reports it as
// 0.0% because percent(0, 0) divides by a substituted 1, and that reading is
// wrong in the direction that matters: it would put the six no-op defaults in
// internal/ui at the top of a report about untested code.
func (s Score) Coverage() float64 {
	if s.Statements == 0 {
		return 1
	}
	return float64(s.Covered) / float64(s.Statements)
}

// Value is the C.R.A.P. score.
func (s Score) Value() float64 {
	cc := float64(s.Complexity)
	uncovered := 1 - s.Coverage()
	return cc*cc*uncovered*uncovered*uncovered + cc
}
