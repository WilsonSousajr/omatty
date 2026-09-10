package detach_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/detach"
	"github.com/WilsonSousajr/omatty/internal/paths"
)

// heldProcess starts a process that will not exit on its own and records its
// pid where Stop looks for it, standing in for the claude a dtach master holds.
//
// The returned channel carries the process's exit, and a goroutine is already
// waiting on it before Stop runs. That reaping is not test scaffolding, it is
// the production shape: claude's parent is the dtach master, which reaps it, so
// its pid stops answering as soon as it dies. Without a reaper here the killed
// process lingers as a zombie whose pid still answers signal 0, Stop waits out
// the whole grace period, and the test passes on the SIGKILL escalation while
// appearing to prove SIGTERM works (#43).
func heldProcess(t *testing.T, home, sessionID string) <-chan error {
	t.Helper()
	return recordedProcess(t, home, sessionID, exec.Command("sleep", "60"))
}

// recordedProcess starts proc, records its pid where Stop looks for it and
// marks the session held, returning the channel heldProcess documents.
func recordedProcess(t *testing.T, home, sessionID string, proc *exec.Cmd) <-chan error {
	t.Helper()
	if err := proc.Start(); err != nil {
		t.Fatalf("starting the stand-in process: %v", err)
	}
	t.Cleanup(func() { _ = proc.Process.Kill() })
	writePid(t, home, sessionID, proc.Process.Pid)
	holdSocket(t, home, sessionID)
	exited := make(chan error, 1)
	go func() { exited <- proc.Wait() }()
	return exited
}

// holdSocket creates the socket file that marks a session as still held.
//
// Stop stats it rather than connecting: dtach unlinks its socket when the
// master exits, so the file's presence is the only liveness signal omatty has.
// The fake needs one because a pidfile alone is not evidence that the pid is
// still this session's, and Stop now correctly declines to signal without it.
func holdSocket(t *testing.T, home, sessionID string) {
	t.Helper()
	if err := os.WriteFile(sockPath(home, sessionID), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

// sockPath builds the socket path the way the holder does. Not
// detach.SocketPath: that enforces the real 103-byte limit, which t.TempDir()
// on macOS is well past, and these tests are not about the limit.
func sockPath(home, sessionID string) string {
	return filepath.Join(paths.SessionDir(home), sessionID+".sock")
}

// stopper is the holder under test, with its socket-path limit out of the way
// for the reason testDtach in detach_test.go gives.
func stopper(home string) *detach.Dtach {
	return detach.NewDtachCapped(home, "dtach", 4096)
}

// awaitExit blocks until the stand-in process has gone, so the assertions run
// against a settled state without a sleep (AGENTS.md, F.I.R.S.T).
func awaitExit(t *testing.T, exited <-chan error) error {
	t.Helper()
	select {
	case err := <-exited:
		return err
	case <-time.After(10 * time.Second):
		t.Fatal("the stand-in process never exited after Stop()")
		return nil
	}
}

// awaitFile blocks until path exists, so a test does not race the startup of
// the process it is about.
func awaitFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%q never appeared; the stand-in process never reached its trap", path)
}

func writePid(t *testing.T, home, sessionID string, pid int) {
	t.Helper()
	dir := paths.SessionDir(home)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := detach.PidPath(home, sessionID)
	if err := os.WriteFile(path, []byte(strconv.Itoa(pid)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// Archiving is the one place omatty deliberately ends a claude. Without this
// the process would outlive its registry row, holding a socket, reachable from
// neither the sidebar nor state.json (#40, #43).
func TestDtachStop_EndsTheProcessNamedByThePidfile_issue43(t *testing.T) {
	home := t.TempDir()
	exited := heldProcess(t, home, "abc-123")

	if err := stopper(home).Stop("abc-123"); err != nil {
		t.Fatalf("Stop() error = %v, want nil", err)
	}

	if err := awaitExit(t, exited); err == nil {
		t.Error("the process exited cleanly; want it ended by a signal from Stop()")
	}
}

// SIGTERM, not SIGKILL: Claude Code flushes its transcript on the way out, so
// the polite signal has to be the one that lands. A Stop that only ever worked
// by escalation would take the full grace period and lose that flush, and the
// timing is the only thing that tells the two apart (#43).
func TestDtachStop_EndsItPolitelyRatherThanWaitingOutTheGrace_issue43(t *testing.T) {
	home := t.TempDir()
	exited := heldProcess(t, home, "abc-123")

	start := time.Now()
	if err := stopper(home).Stop("abc-123"); err != nil {
		t.Fatal(err)
	}
	took := time.Since(start)
	_ = awaitExit(t, exited)

	if took >= time.Second {
		t.Errorf("Stop() took %v, want it to return as soon as SIGTERM lands rather than waiting out the grace period", took)
	}
}

func TestDtachStop_RemovesThePidfileItActedOn_issue43(t *testing.T) {
	home := t.TempDir()
	exited := heldProcess(t, home, "abc-123")

	if err := stopper(home).Stop("abc-123"); err != nil {
		t.Fatal(err)
	}
	_ = awaitExit(t, exited)

	if _, err := os.Stat(detach.PidPath(home, "abc-123")); !os.IsNotExist(err) {
		t.Errorf("pidfile still on disk after Stop(); Stat err = %v, want not-exist", err)
	}
}

// A session started before dtach existed, or one whose master is long gone, has
// no pidfile. There is nothing to stop, so that is success: an error here would
// make every such archive report a failure the operator cannot act on.
func TestDtachStop_MissingPidfileIsNotAnError_issue43(t *testing.T) {
	if err := stopper(t.TempDir()).Stop("never-started"); err != nil {
		t.Errorf("Stop() with no pidfile = %v, want nil", err)
	}
}

// A pidfile holding something that is not a number means omatty's own state is
// corrupt. Say which file and what was in it rather than signalling pid 0,
// which on Unix means "every process in my group".
func TestDtachStop_RefusesAPidfileThatIsNotANumber_issue43(t *testing.T) {
	home := t.TempDir()
	dir := paths.SessionDir(home)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "abc-123.pid")
	if err := os.WriteFile(path, []byte("not-a-pid\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := stopper(home).Stop("abc-123")

	if err == nil {
		t.Fatal("Stop() with a corrupt pidfile returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "not-a-pid") {
		t.Errorf("error %q does not carry the offending contents", err)
	}
}

// Regression, issue #43: nothing removes a pidfile when a claude exits on its
// own - the operator typed /exit, or it crashed - so the file outlives the
// process it names. Pids are recycled (macOS wraps at ~99998), so archiving
// such a row hours later sent SIGTERM and then SIGKILL to whatever process had
// since inherited the number, reported success, and deleted the evidence.
//
// The socket is the discriminator: a dtach master unlinks its own on the way
// out, so a pidfile with no socket beside it names a process that is not this
// session's.
func TestDtachStop_LeavesAPidAloneWhenTheSessionIsNoLongerHeld_issue43(t *testing.T) {
	home := t.TempDir()
	stranger := exec.Command("sleep", "60")
	if err := stranger.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = stranger.Process.Kill() })
	writePid(t, home, "abc-123", stranger.Process.Pid) // a stale pidfile: no socket

	if err := stopper(home).Stop("abc-123"); err != nil {
		t.Fatalf("Stop() error = %v, want nil: an unheld session is nothing to stop", err)
	}

	if err := stranger.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("Stop() signalled a process this session does not own: %v", err)
	}
	if _, err := os.Stat(detach.PidPath(home, "abc-123")); !os.IsNotExist(err) {
		t.Errorf("the stale pidfile survived Stop(); Stat err = %v, want not-exist", err)
	}
}

// The SIGKILL escalation is the riskiest path in the package and had no test at
// all: stop_test covered only the case where SIGTERM lands. Nothing exercised
// waitGone's deadline, its final post-loop probe, or killProcess - the branch
// that sends SIGKILL to whatever the pidfile names (#43).
func TestDtachStop_KillsAProcessThatIgnoresSIGTERM_issue43(t *testing.T) {
	home := t.TempDir()
	ready := filepath.Join(home, "trap-installed")
	// The marker is written once the trap is installed, and Stop does not run
	// until it appears. sh needs a moment to parse and install it, and a
	// SIGTERM that arrives first lands on a process still carrying the default
	// disposition: it dies politely, the escalation never runs, and the test
	// passes while proving the opposite of its name.
	deaf := exec.Command("sh", "-c", `trap "" TERM; : > "$1"; while :; do sleep 0.1; done`, "sh", ready)
	exited := recordedProcess(t, home, "abc-123", deaf)
	awaitFile(t, ready)

	start := time.Now()
	if err := stopper(home).Stop("abc-123"); err != nil {
		t.Fatalf("Stop() error = %v, want nil: SIGKILL is the escalation, not a failure", err)
	}
	took := time.Since(start)

	if err := awaitExit(t, exited); err == nil {
		t.Error("the process exited cleanly; want it killed after it ignored SIGTERM")
	}
	if took < time.Second {
		t.Errorf("Stop() returned in %v, want it to wait out the grace period before escalating", took)
	}
}
