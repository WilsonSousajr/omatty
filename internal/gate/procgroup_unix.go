//go:build unix

package gate

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
)

// isolate puts a step in its own process group and kills that whole group on
// cancel, rather than only the sh that started it.
//
// Found by CI on #224: macOS passed and ubuntu did not, because killing the
// direct child is enough only when sh exec'd the step's command into itself.
// Where it forked instead, the grandchild survived, kept the output pipe open,
// and CombinedOutput waited on it - a cancelled run took the full 30 seconds.
//
// The orphan is the real cost, not the delay. A cancelled `go test ./... -race`
// that keeps running is exactly the machine-thrashing the runner's parallelism
// bound exists to prevent (#229).
func isolate(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return killGroup(cmd) }
}

// killGroup SIGKILLs the step's process group. Setpgid makes the child its own
// group leader, so its pid is the group id and the negative of it names them
// all.
func killGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return os.ErrProcessDone
	}
	err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return os.ErrProcessDone
	}
	return err
}
