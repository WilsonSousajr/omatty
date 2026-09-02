package hooks_test

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/hooks"
)

// shortSocketDir returns a temp dir short enough for a unix socket path;
// macOS caps sun_path near 104 bytes and t.TempDir() can exceed it.
func shortSocketDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "om")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func listen(t *testing.T) (string, <-chan string) {
	t.Helper()
	path := filepath.Join(shortSocketDir(t), "s")
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	got := make(chan string, 1)
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer func() { _ = c.Close() }()
		line, _ := bufio.NewReader(c).ReadString('\n')
		got <- line
	}()
	return path, got
}

func TestReport_ForwardsThePayloadToTheSocket_issue18(t *testing.T) {
	path, got := listen(t)
	stdin := strings.NewReader(`{"session_id":"abc","hook_event_name":"Notification","notification_type":"idle_prompt"}`)

	if err := hooks.Report(stdin, path, time.Second); err != nil {
		t.Fatalf("Report() error = %v, want nil", err)
	}

	select {
	case line := <-got:
		var p hooks.Payload
		if err := json.Unmarshal([]byte(line), &p); err != nil {
			t.Fatalf("listener got non-JSON %q: %v", line, err)
		}
		if p.SessionID != "abc" || p.HookEventName != "Notification" || p.NotificationType != "idle_prompt" {
			t.Errorf("forwarded payload = %+v, want the fields intact", p)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("listener never received the payload")
	}
}

// Invariant 11: a missing socket (omatty not running) must not fail the hook,
// or every claude session on the machine would stall.
func TestReport_MissingSocketIsNotAnError_issue18(t *testing.T) {
	err := hooks.Report(
		strings.NewReader(`{"session_id":"x","hook_event_name":"Stop"}`),
		filepath.Join(shortSocketDir(t), "no"), time.Second)
	if err != nil {
		t.Errorf("Report() with no socket = %v, want nil (invariant 11)", err)
	}
}

func TestReport_MalformedJSONIsNotAnError_issue18(t *testing.T) {
	path, _ := listen(t)
	if err := hooks.Report(strings.NewReader("{not json at all"), path, time.Second); err != nil {
		t.Errorf("Report() with garbage stdin = %v, want nil (invariant 11)", err)
	}
}

// A hostile or runaway producer must not make the hook buffer megabytes.
func TestReport_OversizedStdinIsBounded_issue18(t *testing.T) {
	path, got := listen(t)
	huge := `{"session_id":"` + strings.Repeat("A", 2<<20) + `","hook_event_name":"Stop"}`

	done := make(chan error, 1)
	go func() { done <- hooks.Report(strings.NewReader(huge), path, time.Second) }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Report() = %v, want nil even for oversized input", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Report did not return; it likely buffered the whole input")
	}
	// Whatever reached the socket must be capped, not the full 2 MiB.
	select {
	case line := <-got:
		if len(line) > 128<<10 {
			t.Errorf("forwarded %d bytes, want it capped near 64 KiB", len(line))
		}
	case <-time.After(time.Second):
	}
}
