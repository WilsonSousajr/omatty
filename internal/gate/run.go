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
	if tool, absent := absentTool(step.Run); absent {
		return StepResult{
			Step:    step,
			Verdict: Missing,
			Output:  fmt.Sprintf("gate: %q is not on PATH, so this step was not run\n", tool),
		}
	}
	started := time.Now()
	cmd := exec.CommandContext(ctx, "sh", "-c", step.Run)
	cmd.Dir = dir
	isolate(cmd)
	cmd.WaitDelay = cancelGrace
	out, err := cmd.CombinedOutput()

	verdict, code := classify(ctx, err)
	result := StepResult{Step: step, Verdict: verdict, ExitCode: code, Output: tail(string(out)), Elapsed: time.Since(started)}
	if step.Kind == KindCoverage {
		result.Percent = percentIn(result.Output)
	}
	return result
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

// absentTool reports the step's leading command when PATH cannot resolve it.
//
// This is the pre-flight invariant 12 describes. It is deliberately
// conservative: anything it cannot confidently read as a plain command name is
// simply run, because wrongly reporting Missing would hide a real failure.
func absentTool(run string) (string, bool) {
	name := leadingWord(run)
	if name == "" {
		return "", false
	}
	if _, err := exec.LookPath(name); err != nil {
		return name, true
	}
	return "", false
}

// shellWords are names sh resolves itself. A PATH lookup for them proves
// nothing - some systems ship /usr/bin/cd and most do not, and either way sh
// runs its own - so a step starting with one is left alone.
var shellWords = map[string]bool{
	".": true, ":": true, "[": true, "case": true, "cd": true, "eval": true,
	"exec": true, "export": true, "for": true, "if": true, "read": true,
	"set": true, "shift": true, "source": true, "test": true, "trap": true,
	"umask": true, "unset": true, "wait": true, "while": true,
}

// leadingWord is the step's command name, or "" when the line does not begin
// with one: a FOO=1 assignment, a subshell, a redirect, a shell builtin.
func leadingWord(run string) string {
	fields := strings.Fields(run)
	if len(fields) == 0 {
		return ""
	}
	word := fields[0]
	if strings.ContainsAny(word, `="'$`+"`"+`(){}|&;<>`) || shellWords[word] {
		return ""
	}
	return word
}
