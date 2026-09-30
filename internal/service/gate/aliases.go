package gate

import dgate "github.com/WilsonSousajr/omatty/internal/domain/gate"

// The gate's vocabulary moved to internal/domain/gate (ADR 0001, migration step
// 3.3). These aliases keep every importer compiling while the importers move
// over in small steps; migration step 8.1 deletes them. New code imports
// internal/domain/gate directly:
//
//	import "github.com/WilsonSousajr/omatty/internal/domain/gate"
//	steps := []gate.Step{{Name: "test", Run: "go test ./..."}}

// Step is dgate.Step.
type Step = dgate.Step

// Verdict is dgate.Verdict.
type Verdict = dgate.Verdict

// StepResult is dgate.StepResult.
type StepResult = dgate.StepResult

// Report is dgate.Report.
type Report = dgate.Report

// The verdicts, as dgate declares them.
const (
	Pending   = dgate.Pending
	Running   = dgate.Running
	Pass      = dgate.Pass
	Fail      = dgate.Fail
	Missing   = dgate.Missing
	Cancelled = dgate.Cancelled
)

// The output caps, as dgate declares them.
const (
	MaxOutputLines = dgate.MaxOutputLines
	MaxOutputBytes = dgate.MaxOutputBytes
	KeptBytes      = dgate.KeptBytes
)

// KindCoverage is dgate.KindCoverage.
const KindCoverage = dgate.KindCoverage

// MaxComposeLines is dgate.MaxComposeLines.
const MaxComposeLines = dgate.MaxComposeLines

// Compose is dgate.Compose.
//
//	prompt := gate.Compose(report.Results)
func Compose(results []StepResult) string { return dgate.Compose(results) }
