// Package e2e verifies the real cross-process status path: the actual `omatty
// hook` binary writing to a real status.Listen socket. This is the wiring the
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

	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/hookserver"
	"github.com/WilsonSousajr/omatty/internal/service/status"
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
	build := exec.Command("go", "build", "-o", omattyBin, "../../../../cmd/omatty")
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
	case p := <-events:
		// The hook server hands over the payload; the watcher's adapter says
		// what it means (step 5.2d, #653). Both halves are real here.
		kind, _ := status.KindOf(p)
		ev := dstatus.Event{SessionID: p.SessionID, Kind: kind}
		if ev.SessionID != "abc" || ev.Kind != dstatus.PermissionRequested {
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

// An agent omatty does not know is a hook that sends nothing and exits 0
// silently, whatever its payload (invariant 11, #522).
//
// Nothing sent is shown by order rather than by waiting: a claude hook runs
// after the unknown one has exited, so had the first sent its payload, it
// would be the first to arrive.
func TestOmattyHook_UnknownAgentExitsZeroSilently_issue522(t *testing.T) {
	dir, events, closeListener := listenUnderHome(t)
	defer closeListener()

	out, err := runHookArgs(t, []string{"HOME=" + dir}, `{"session_id":"x","hook_event_name":"Stop"}`, "--agent", "nope")
	if err != nil || len(out) != 0 {
		t.Errorf("omatty hook --agent nope exited %v with output %q, want exit 0 and no output (invariant 11)", err, out)
	}
	if _, err := runHookArgs(t, []string{"HOME=" + dir}, `{"session_id":"after","hook_event_name":"Stop"}`); err != nil {
		t.Fatal(err)
	}
	select {
	case p := <-events:
		if p.SessionID != "after" {
			t.Errorf("first payload is %q, want the claude hook's: the unknown agent's was sent", p.SessionID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the claude hook never reached the listener")
	}
}

// --agent claude is the flagless hook, delivered the same way; a malformed
// payload in claude's shape is still a silent success (#522).
func TestOmattyHook_AgentFlagReadsThatAgentsShape_issue522(t *testing.T) {
	dir, events, closeListener := listenUnderHome(t)
	defer closeListener()

	if out, err := runHookArgs(t, []string{"HOME=" + dir}, `{not json`, "--agent", "claude"); err != nil || len(out) != 0 {
		t.Errorf("malformed payload: exited %v with %q, want exit 0 and no output", err, out)
	}
	if out, err := runHookArgs(t, []string{"HOME=" + dir}, `{"session_id":"abc","hook_event_name":"Stop"}`, "--agent", "claude"); err != nil || len(out) != 0 {
		t.Fatalf("exited %v with %q, want exit 0 and no output", err, out)
	}
	select {
	case p := <-events:
		if p.SessionID != "abc" || p.HookEventName != "Stop" {
			t.Errorf("received %+v, want abc Stop", p)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("omatty hook --agent claude never reached the listener")
	}
}

// The 4 MiB bound holds with the flag as without it (invariant 11, #55).
func TestOmattyHook_AgentFlagKeepsTheStdinBound_issue522(t *testing.T) {
	big := `{"session_id":"x","hook_event_name":"Stop","tool_response":"` + strings.Repeat("a", 5<<20) + `"}`
	out, err := runHookArgs(t, nil, big, "--agent", "claude")
	if err != nil || len(out) != 0 {
		t.Errorf("oversized stdin: exited %v with %q, want exit 0 and no output", err, out)
	}
}

// runHookArgs runs the built binary's hook subcommand with args after
// "hook", the test's environment minus HOME, plus env.
func runHookArgs(t *testing.T, env []string, stdin string, args ...string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(omattyBinary(t), append([]string{"hook"}, args...)...)
	cmd.Env = append(withoutHome(os.Environ()), env...)
	cmd.Stdin = strings.NewReader(stdin)
	return cmd.CombinedOutput()
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
func listenUnderHome(t *testing.T) (string, <-chan dstatus.HookPayload, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "om") // macOS caps a unix path near 104 bytes
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".omatty"), 0o700); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(dir, ".omatty", "sock")
	events := make(chan dstatus.HookPayload, 4)
	l, err := hookserver.Listen(sock, events)
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
