// Package gate runs a project's own verification commands and says, per step,
// whether they passed.
//
// A gate is the line a project already has in its contributing guide - gofmt,
// vet, lint, the test suite, a coverage threshold. omatty does not invent it,
// interpret it, or decide what belongs in it; it runs what the project says
// and reports the outcome next to the session that caused it.
//
// Invariant 12 is the rule this package exists to keep: a step passes if and
// only if its process exits 0. Nothing here reads a step's output to decide
// pass or fail, because output moves with tool version, locale, colour and
// verbosity while the exit code is the fact the tool is asserting. Output is
// carried for a human to read and for Compose to send back, never consulted.
//
// The one place that costs something is a tool that is not installed. Steps
// run under `sh -c`, since gate lines carry pipes, arguments and script paths,
// so exec.Cmd.Err never fires - sh exists even when the tool does not - and an
// absent tool arrives as the shell's exit 127. That is a convention rather
// than a guarantee, and a real command may exit 127 for its own reasons, so
// this package does not read it as one. It resolves a step's leading word with
// exec.LookPath before running anything and reports Missing instead. Telling a
// session its lint is failing when golangci-lint is merely absent would send
// it off to fix code that was never broken.
package gate

import (
	"fmt"
	"time"
)

// Step is one verification command in a project's gate.
//
// Run is a shell line rather than an argv because that is how a project writes
// its own gate: `go test ./... -race`, `./scripts/check-coverage.sh 90`.
//
//	gate.Step{Name: "test", Run: "go test ./... -race"}
type Step struct {
	// Name labels the step in the sidebar strip, so it is short by intent.
	Name string `json:"name"`
	// Run is the shell line, executed with sh -c in the session's directory.
	Run string `json:"run"`
	// Kind is "" for an ordinary step, or "coverage" for one whose percentage
	// is worth reading out of its output for display (#225). It never decides
	// pass or fail; invariant 12 reserves that to the exit code.
	Kind string `json:"kind,omitempty"`
}

// Verdict is how a step ended.
type Verdict int

const (
	// Pending is a step the run never reached, because an earlier one did not
	// pass. Distinct from Pass so the strip can show what was not checked.
	Pending Verdict = iota
	// Running is a step in flight, set by the runner that owns the goroutine.
	Running
	// Pass is exit 0, and nothing else.
	Pass
	// Fail is any other exit status.
	Fail
	// Missing is a step whose command is not on PATH. Not a Fail: it is a
	// statement about the machine, not about the code.
	Missing
	// Cancelled is a step whose context ended before it did.
	Cancelled
)

// String names a verdict for logs and test failures.
func (v Verdict) String() string {
	switch v {
	case Pending:
		return "pending"
	case Running:
		return "running"
	case Pass:
		return "pass"
	case Fail:
		return "fail"
	case Missing:
		return "missing"
	case Cancelled:
		return "cancelled"
	}
	return fmt.Sprintf("Verdict(%d)", int(v))
}

// StepResult is one step's outcome. Output is already bounded when a caller
// sees it: a broken build writes without limit, and every byte of it would
// otherwise reach the sidebar, the gate pane and eventually claude's context.
type StepResult struct {
	Step     Step
	Verdict  Verdict
	ExitCode int
	Output   string
	Elapsed  time.Duration
}
