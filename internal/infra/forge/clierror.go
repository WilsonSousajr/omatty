package forge

import "fmt"

// CLIError is a forge CLI that failed: the call, where it ran, the CLI's own
// words and the process error beneath. Error() is the whole account, for the
// log; Detail is what a narrow surface shows, because the account leads with
// a command line and a path, ends with "exit status 1", and the CLI's words -
// the part anyone acts on - were buried between them in the tracker's notice
// (#804). vcs.CommandError does the same for git (#351).
//
//	var cli *forge.CLIError
//	if errors.As(err, &cli) { show(cli.Detail()) }
type CLIError struct {
	Call  string // "gh pr list", "glab api"
	Where string // `in "/repo"` or "on gitlab.com"
	Said  string // the CLI's stderr, cleaned and on one line (said)
	Err   error
}

// Error is the whole account, for the log.
func (e *CLIError) Error() string {
	return fmt.Sprintf("forge: %s %s: %s: %v", e.Call, e.Where, e.Said, e.Err)
}

// Unwrap is the process error, so errors.As still finds an *exec.ExitError.
func (e *CLIError) Unwrap() error { return e.Err }

// Detail is the CLI's words, or the process error when it said nothing.
func (e *CLIError) Detail() string {
	if e.Said == "" {
		return e.Err.Error()
	}
	return e.Said
}
