package gate

// Report is the outcome of one session's gate run.
//
// Err is set only when the run could not happen at all - an unusable working
// directory, or a panic. A gate that ran and failed is not an error; it is
// Results, and the caller renders it.
type Report struct {
	ID      string
	Results []StepResult
	Err     error
}
