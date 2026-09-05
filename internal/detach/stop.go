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
//
// The pid is only signalled while the session is still held. See stillHeld:
// a pidfile outlives the process it names, and a pid is not a stable handle.
func (d *Dtach) Stop(sessionID string) error {
	pidfile := PidPath(d.home, sessionID)
	pid, err := readPid(pidfile, sessionID)
	if err != nil || pid == 0 {
		return err
	}
	held, err := d.stillHeld(sessionID)
	if err != nil {
		return err
	}
	if held {
		if err := endProcess(pid, sessionID); err != nil {
			return err
		}
	}
	// Best effort, and taken on both paths: the process is gone, which is what
	// the caller asked for. A leftover pidfile only means the next Stop finds a
	// pid that no longer exists, which it already tolerates - but leaving a
	// stale one is what lets a later archive signal a stranger.
	_ = os.Remove(pidfile)
	return nil
}

// stillHeld reports whether the session's dtach master is alive, which is what
// makes the recorded pid safe to signal.
//
// dtach unlinks its socket when the master exits, so the socket's presence is
// the liveness test omatty has. It is needed because nothing removes a pidfile
// when a claude exits on its own - the operator typed /exit, or it crashed -
// and pids are recycled: macOS wraps at ~99998. Without this check, archiving
// such a row hours later sent SIGTERM and then SIGKILL to whatever process had
// since inherited the number, reported success, and deleted the evidence (#43).
func (d *Dtach) stillHeld(sessionID string) (bool, error) {
	sock, err := d.socketPath(sessionID)
	if err != nil {
		return false, err
	}
	if _, err := os.Stat(sock); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf(
			"detach: session %s: checking whether socket %q is still held: %w", sessionID, sock, err)
	}
	return true, nil
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

// gone reports whether a signal error means the process no longer exists.
//
// Only ESRCH does. EPERM means the opposite - the process is alive and owned
// by somebody else, which is what a recycled pid looks like - and reading it as
// "gone" both reported a successful stop for a process omatty never touched and
// skipped the SIGKILL escalation for one that was still running (#43).
func gone(err error) bool {
	return errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH)
}

// endProcess signals pid politely, then forcibly.
func endProcess(pid int, sessionID string) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("detach: session %s: locating process %d: %w", sessionID, pid, err)
	}
	err = proc.Signal(syscall.SIGTERM)
	if gone(err) {
		// The operator quit claude themselves between the last check and now.
		return nil
	}
	if err != nil {
		return fmt.Errorf("detach: session %s: signalling process %d: %w", sessionID, pid, err)
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
		if gone(proc.Signal(syscall.Signal(0))) {
			return true
		}
		time.Sleep(stopPoll)
	}
	return gone(proc.Signal(syscall.Signal(0)))
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
