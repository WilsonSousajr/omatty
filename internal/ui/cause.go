package ui

import "errors"

// logHint follows a failure notice that shows only the cause: the whole
// chain - which command, in which directory - is in the log.
const logHint = "full error in the log"

// detailer is an error that knows the part of itself worth showing on a
// narrow surface. vcs.CommandError is one: its Detail is git's own stderr,
// where its innermost error is only "exit status 128" (#351). Asked for by
// behaviour rather than by type, so ui does not import vcs to read an error.
type detailer interface{ Detail() string }

// causeOf is what a notice shows for err: the innermost detail in its chain,
// or the whole message when nothing in it offers one.
func causeOf(err error) string {
	var d detailer
	if errors.As(err, &d) && d.Detail() != "" {
		return d.Detail()
	}
	return err.Error()
}
