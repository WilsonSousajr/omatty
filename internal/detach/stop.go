package detach

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// stopGrace is how long a claude gets to exit on SIGTERM before it is killed.
// Claude Code flushes its transcript on the way out, so the polite signal is
// worth waiting for; two seconds is long enough for that and short enough that
// an archive still feels immediate.
const stopGrace = 2 * time.Second

// stopPoll is how often the wait re-checks. Signal 0 is a liveness probe that
// costs a syscall, so this can be frequent without mattering.
const stopPoll = 20 * time.Millisecond

// Stop ends the claude a dtach master is holding for a session.
//
//	err := holder.Stop(sess.ID)
//
// It is called when a session is archived - the one place omatty deliberately
// ends a claude - and never when omatty merely quits, which is the whole point
// of holding it (#40, #43). SIGTERM first so the transcript is flushed, then
// SIGKILL if the process outlives stopGrace. The dtach master exits with its
// child and unlinks its own socket.
func (d *Dtach) Stop(sessionID string) error {
	pid, err := readPid(PidPath(d.home, sessionID), sessionID)
	if err != nil || pid == 0 {
		return err
	}
	if err := endProcess(pid, sessionID); err != nil {
		return err
	}
	// Best effort: the process is gone, which is what the caller asked for. A
	// leftover pidfile only means the next Stop finds a pid that no longer
	// exists, which it already tolerates.
	_ = os.Remove(PidPath(d.home, sessionID))
	return nil
}

// readPid reads the pid a session's wrapper recorded. A missing file returns
// zero and no error: a session started before dtach, or one whose master is
// long gone, has nothing to stop, and reporting that as a failure would make
// every such archive look broken.
func readPid(path, sessionID string) (int, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("detach: session %s: reading pidfile %q: %w", sessionID, path, err)
	}
	text := strings.TrimSpace(string(raw))
	pid, err := strconv.Atoi(text)
	if err != nil || pid <= 0 {
		// Never fall through to signalling: pid 0 means "every process in my
		// process group" and a negative pid means a whole group, so a corrupt
		// file must stop here rather than become a very bad kill.
		return 0, fmt.Errorf(
			"detach: session %s: pidfile %q holds %q, want a positive process id", sessionID, path, text)
	}
	return pid, nil
}

// endProcess signals pid politely, then forcibly.
func endProcess(pid int, sessionID string) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("detach: session %s: locating process %d: %w", sessionID, pid, err)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		// Already gone is the common case: the operator quit claude themselves
		// before archiving the session.
		return nil
	}
	if waitGone(proc, stopGrace) {
		return nil
	}
	return killProcess(proc, pid, sessionID)
}

// waitGone polls until the process is unreachable or the grace period is up,
// reporting whether it went. Signal 0 delivers nothing and only asks whether
// the process is still there.
func waitGone(proc *os.Process, grace time.Duration) bool {
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if proc.Signal(syscall.Signal(0)) != nil {
			return true
		}
		time.Sleep(stopPoll)
	}
	return proc.Signal(syscall.Signal(0)) != nil
}

// killProcess is the escalation for a claude that ignored SIGTERM.
func killProcess(proc *os.Process, pid int, sessionID string) error {
	if err := proc.Kill(); err != nil {
		return fmt.Errorf(
			"detach: session %s: process %d survived SIGTERM and could not be killed: %w",
			sessionID, pid, err)
	}
	return nil
}
