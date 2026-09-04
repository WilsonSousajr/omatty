// Package e2e verifies the real cross-process status path: the actual `omatty
// hook` binary writing to a real watcher.Listen socket. This is the wiring the
// unit tests fake on both ends, and the roadmap's rule-2 check for M2.
package e2e_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// omattyBin is the binary under test, built once for the package (issue #80:
// building it per test cost a second each).
var omattyBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "om-bin")
	if err != nil {
		panic(err)
	}
	omattyBin = filepath.Join(dir, "omatty")
	build := exec.Command("go", "build", "-o", omattyBin, "../../../cmd/omatty")
	if out, err := build.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("building omatty: %v\n%s", err, out))
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestOmattyHook_DeliversToARealListener(t *testing.T) {
	bin := omattyBinary(t)

	dir, events, closeListener := listenUnderHome(t)
	defer closeListener()

	cmd := exec.Command(bin, "hook")
	cmd.Env = append(os.Environ(), "HOME="+dir)
	cmd.Stdin = strings.NewReader(`{"session_id":"abc","hook_event_name":"PermissionRequest"}`)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("omatty hook exited %v: %s", err, out)
	}
	if len(out) != 0 {
		t.Errorf("omatty hook wrote output, want none (invariant 11): %q", out)
	}

	select {
	case ev := <-events:
		if ev.SessionID != "abc" || ev.Kind != watcher.PermissionRequested {
			t.Errorf("received %+v, want session abc PermissionRequested", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the real omatty hook never reached the real listener")
	}
}

func TestOmattyHook_ExitsZeroWithNoListener(t *testing.T) {
	bin := omattyBinary(t)
	dir, _ := os.MkdirTemp("", "om")
	defer func() { _ = os.RemoveAll(dir) }()

	cmd := exec.Command(bin, "hook")
	cmd.Env = append(os.Environ(), "HOME="+dir)
	cmd.Stdin = strings.NewReader(`{"session_id":"x","hook_event_name":"Stop"}`)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("omatty hook with no listener exited %v: %s (invariant 11)", err, out)
	}
}

// Regression, issue #54: the hook subcommand ran after the log file was
// opened, so an unwritable ~/.omatty/logs made every hook on the machine
// exit 1 with two lines on stderr (invariant 11).
func TestOmattyHook_ExitsZeroWhenTheLogDirIsUnwritable_issue54(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir, err := os.MkdirTemp("", "om")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	locked := filepath.Join(dir, ".omatty")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(locked, 0o700) }()

	out, err := runHook(t, []string{"HOME=" + dir}, `{"session_id":"x","hook_event_name":"Stop"}`)

	if err != nil || len(out) != 0 {
		t.Errorf("omatty hook exited %v with output %q, want exit 0 and no output (invariant 11)", err, out)
	}
}

// Same bug, second trigger: os.UserHomeDir fails without HOME and the error
// reached main's stderr path.
func TestOmattyHook_ExitsZeroWithoutHOME_issue54(t *testing.T) {
	out, err := runHook(t, nil, `{"session_id":"x","hook_event_name":"Stop"}`)

	if err != nil || len(out) != 0 {
		t.Errorf("omatty hook without HOME exited %v with output %q, want exit 0 and no output (invariant 11)", err, out)
	}
}

// runHook runs the built binary's hook subcommand with the test's own
// environment minus HOME, plus env.
func runHook(t *testing.T, env []string, stdin string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(omattyBinary(t), "hook")
	cmd.Env = append(withoutHome(os.Environ()), env...)
	cmd.Stdin = strings.NewReader(stdin)
	return cmd.CombinedOutput()
}

func withoutHome(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, "HOME=") {
			out = append(out, kv)
		}
	}
	return out
}

// listenUnderHome creates a short-pathed socket at $HOME/.omatty/sock and a
// listener on it, returning the HOME to point the hook binary at.
func listenUnderHome(t *testing.T) (string, <-chan watcher.Event, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "om") // macOS caps a unix path near 104 bytes
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".omatty"), 0o700); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(dir, ".omatty", "sock")
	events := make(chan watcher.Event, 4)
	l, err := watcher.Listen(sock, events, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	return dir, events, func() { _ = l.Close(); _ = os.RemoveAll(dir) }
}

// omattyBinary is the path TestMain built. It is a function rather than a bare
// read of omattyBin so a test that runs without TestMain fails loudly here
// instead of exec'ing the empty string.
func omattyBinary(t *testing.T) string {
	t.Helper()
	if omattyBin == "" {
		t.Fatal("omattyBin is empty: TestMain did not build the binary")
	}
	return omattyBin
}
