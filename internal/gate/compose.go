package gate

import (
	"fmt"
	"strings"
)

// MaxComposeLines bounds one step's output inside the composed message. The
// per-step cap is already applied; this one is tighter because the result is
// pasted into a live session, and a message that fills claude's context in
// order to report a test failure has cost more than it explained.
const MaxComposeLines = 60

// composeHeader opens the message. It says "did not pass" rather than "failed"
// because a Missing step is not a failure of the code, and one wording has to
// cover both.
const composeHeader = "The gate did not pass.\n\n"

// Compose renders a run as the message omatty sends back into the session that
// caused it, or "" when there is nothing to report.
//
//	if msg := gate.Compose(results); msg != "" {
//	        term.SendInput(paste.BracketedPaste(msg))
//	}
//
// A green run composes to "" on purpose: spending a turn to tell a session
// everything is fine is worse than saying nothing.
func Compose(results []StepResult) string {
	var body strings.Builder
	for _, result := range results {
		if result.Verdict == Pass || result.Verdict == Pending {
			continue
		}
		body.WriteString(block(result))
	}
	if body.Len() == 0 {
		return ""
	}
	return composeHeader + body.String()
}

// block is one step's section: what it was, how it ended, and its output.
func block(result StepResult) string {
	kept, dropped := lastLines(result.Output, MaxComposeLines)
	if dropped > 0 {
		kept = elision(dropped, 0) + kept
	}
	return fmt.Sprintf("%s — %s\n$ %s\n%s\n", result.Step.Name, outcome(result), result.Step.Run, kept)
}

// outcome states how a step ended in the message's own terms. A Missing step
// reports the machine, not the code, so it never reads as an exit status.
func outcome(result StepResult) string {
	if result.Verdict == Missing {
		return "not run"
	}
	return fmt.Sprintf("exit %d", result.ExitCode)
}
