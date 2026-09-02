// Package e2e verifies the real cross-process status path: the actual `omatty
// hook` binary writing to a real watcher.Listen socket. This is the wiring the
// unit tests fake on both ends, and the roadmap's rule-2 check for M2.
package e2e_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func TestOmattyHook_DeliversToARealListener(t *testing.T) {
	bin := buildOmatty(t)

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
	bin := buildOmatty(t)
	dir, _ := os.MkdirTemp("", "om")
	defer func() { _ = os.RemoveAll(dir) }()

	cmd := exec.Command(bin, "hook")
	cmd.Env = append(os.Environ(), "HOME="+dir)
	cmd.Stdin = strings.NewReader(`{"session_id":"x","hook_event_name":"Stop"}`)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Errorf("omatty hook with no listener exited %v: %s (invariant 11)", err, out)
	}
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

func buildOmatty(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "omatty")
	build := exec.Command("go", "build", "-o", bin, "../../../cmd/omatty")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building omatty: %v\n%s", err, out)
	}
	return bin
}
