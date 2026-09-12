//go:build !unix

package gate

import "os/exec"

// isolate does nothing where process groups are not available, leaving
// WaitDelay in run.go as the only backstop against a step that will not let go
// of its output pipe. omatty's CI covers linux and darwin; this file exists so
// the package still builds elsewhere.
func isolate(cmd *exec.Cmd) {}
