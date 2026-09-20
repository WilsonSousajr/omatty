package gate

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Run executes a project's gate in dir and returns one result per step, in
// order. It stops at the first step that does not pass: a gate reports whether
// the work is sound, and once one step says no the rest are not evidence.
// Steps that never ran come back Pending rather than Pass.
//
//	results, err := gate.Run(ctx, sess.Dir, proj.Gate)
//
// An err means dir itself is unusable. A failing step is a result, not an
// error - callers render it.
func Run(ctx context.Context, dir string, steps []Step) ([]StepResult, error) {
	if err := usableDir(dir); err != nil {
		return nil, err
	}
	results := make([]StepResult, len(steps))
	for i, step := range steps {
		results[i] = StepResult{Step: step, Verdict: Pending}
	}
	for i := range steps {
		results[i] = runStep(ctx, dir, steps[i])
		if results[i].Verdict != Pass {
			break
		}
	}
	return results, nil
}

// cancelGrace is how long Wait will keep reading a cancelled step's output
// before giving up on it. isolate already kills the process group, so this is
// the backstop for anything that escaped it - without it, one process holding
// the pipe open blocks the whole run (#224).
const cancelGrace = 2 * time.Second

// usableDir rejects a working directory before any step runs, so the failure
// names the path rather than arriving as N identical shell errors.
func usableDir(dir string) error {
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("gate: working directory %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("gate: working directory %q is not a directory", dir)
	}
	return nil
}

// runStep runs one step, or declines to run it when its tool is absent.
func runStep(ctx context.Context, dir string, step Step) StepResult {
	if tool, absent := absentTool(step.Run, dir); absent {
		return StepResult{
			Step:    step,
			Verdict: Missing,
			Output:  fmt.Sprintf("gate: %q is not on PATH, so this step was not run\n", tool),
		}
	}
	started := time.Now()
	out, err := shellOut(ctx, dir, step.Run)

	verdict, code := classify(ctx, err)
	result := StepResult{Step: step, Verdict: verdict, ExitCode: code, Output: out, Elapsed: time.Since(started)}
	if step.Kind == KindCoverage {
		result.Percent = percentIn(result.Output)
	}
	return result
}

// shellOut runs one gate line under sh and returns its output already bounded
// both ways: to a KeptBytes window while it runs, and to the caps tail
// applies once it has finished.
//
// Stdout and Stderr are the same writer, which os/exec serves from one pipe
// and one goroutine - the arrangement CombinedOutput makes, without
// CombinedOutput's habit of holding a runaway step's every byte first.
func shellOut(ctx context.Context, dir, run string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", run)
	cmd.Dir = dir
	isolate(cmd)
	cmd.WaitDelay = cancelGrace
	var out boundedOutput
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	return tail(out.String(), out.dropped), err
}

// classify turns the error from a finished command into a verdict. Invariant
// 12: only the exit status is consulted, never the output.
func classify(ctx context.Context, err error) (Verdict, int) {
	if ctx.Err() != nil {
		return Cancelled, -1
	}
	if err == nil {
		return Pass, 0
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return Fail, exit.ExitCode()
	}
	return Fail, -1
}

// absentTool reports the step's leading command when the shell cannot resolve
// it at all.
//
// This is the pre-flight invariant 12 describes, and it asks `sh` rather than
// guessing. exec.LookPath answers a different question - "is there a binary on
// PATH" - and a shell builtin has none: `exit`, `return`, `local`, `declare`
// and `break` all failed it, so a step of `exit 1` was declined as a missing
// tool and stopped the gate (#248). `command -v` resolves builtins, keywords,
// functions and PATH entries alike, in the same sh the step itself runs under,
// which is the only authority that can be right about this.
//
// The cost is one tiny process per step, which is nothing beside running a
// test suite, and it replaces a list of shell builtins that could never be
// completed - they differ by shell.
func absentTool(run, dir string) (string, bool) {
	name := leadingWord(run)
	if name == "" {
		return "", false
	}
	// `command -v --` so a name beginning with a dash is a name, not a flag.
	probe := exec.Command("sh", "-c", `command -v -- "$1" > /dev/null 2>&1`, "sh", name)
	// In the directory the step will run in, not the one omatty was launched
	// from. A gate line is written relative to the repository it belongs to -
	// `./scripts/check-coverage.sh` is what Detect itself proposes - so a probe
	// run anywhere else answers a question nobody asked, and answers it "no"
	// (#289). Missing also stops the gate, so the invented absence costs every
	// step after it too.
	probe.Dir = dir
	if probe.Run() != nil {
		return name, true
	}
	return "", false
}

// leadingWord is the step's command name, or "" when the line does not begin
// with one: a FOO=1 assignment, a subshell, a variable, a redirect.
//
// Deliberately conservative. Anything it cannot confidently read as a plain
// command name is simply run, because the two mistakes are not equal: a wrong
// "" costs a step that executes normally, while a wrong name hides a real
// failure behind a bogus Missing and stops the gate.
func leadingWord(run string) string {
	fields := strings.Fields(run)
	if len(fields) == 0 {
		return ""
	}
	word := fields[0]
	if strings.ContainsAny(word, `="'$`+"`"+`(){}|&;<>`) {
		return ""
	}
	return word
}
