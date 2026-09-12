//go:build unix

package gate

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"
)

// Cancel can fire against a step that is already gone - it raced the step's own
// exit, or never started. Either is "nothing to kill", and reporting it as a
// kill failure would turn a clean cancel into an error the caller has to
// explain (#224).
func TestKillGroup_nothingToKill_isAlreadyDone(t *testing.T) {
	t.Run("never started", func(t *testing.T) {
		if err := killGroup(exec.Command("true")); !errors.Is(err, os.ErrProcessDone) {
			t.Errorf("killGroup(unstarted) = %v, want os.ErrProcessDone", err)
		}
	})

	t.Run("already exited and reaped", func(t *testing.T) {
		cmd := exec.Command("true")
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Run(); err != nil {
			t.Fatalf("setup: %v", err)
		}
		if err := killGroup(cmd); !errors.Is(err, os.ErrProcessDone) {
			t.Errorf("killGroup(reaped) = %v, want os.ErrProcessDone", err)
		}
	})
}
