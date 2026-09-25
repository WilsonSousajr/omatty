package vcs

import (
	"fmt"
	"strings"
)

// CommandError is a git call that failed: the command, where it ran, git's
// own stderr, and the process error beneath. Error() reads as it always has;
// Detail is what a narrow surface should show, because the innermost error
// is only "exit status 128" and the message leads with a command line and a
// path that cut off git's words at the column edge (#351).
//
//	var cmdErr *vcs.CommandError
//	if errors.As(err, &cmdErr) { show(cmdErr.Detail()) }
type CommandError struct {
	Args   []string
	Dir    string
	Stderr string // trimmed
	Err    error
}

// Error is the full account, for the log.
func (e *CommandError) Error() string {
	return fmt.Sprintf("vcs: `git %s` in %q failed: %s: %v", strings.Join(e.Args, " "), e.Dir, e.Stderr, e.Err)
}

// Unwrap is the process error, so errors.As still finds an *exec.ExitError.
func (e *CommandError) Unwrap() error { return e.Err }

// Detail is git's stderr, or the process error when git wrote nothing.
func (e *CommandError) Detail() string {
	if e.Stderr == "" {
		return e.Err.Error()
	}
	return e.Stderr
}
