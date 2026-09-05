package detach_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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
	proc := exec.Command("sleep", "60")
	if err := proc.Start(); err != nil {
		t.Fatalf("starting the stand-in process: %v", err)
	}
	t.Cleanup(func() { _ = proc.Process.Kill() })
	writePid(t, home, sessionID, proc.Process.Pid)
	exited := make(chan error, 1)
	go func() { exited <- proc.Wait() }()
	return exited
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

	if err := detach.NewDtach(home, "dtach").Stop("abc-123"); err != nil {
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
	if err := detach.NewDtach(home, "dtach").Stop("abc-123"); err != nil {
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

	if err := detach.NewDtach(home, "dtach").Stop("abc-123"); err != nil {
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
	if err := detach.NewDtach(t.TempDir(), "dtach").Stop("never-started"); err != nil {
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

	err := detach.NewDtach(home, "dtach").Stop("abc-123")

	if err == nil {
		t.Fatal("Stop() with a corrupt pidfile returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "not-a-pid") {
		t.Errorf("error %q does not carry the offending contents", err)
	}
}
