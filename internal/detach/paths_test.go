package detach_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/detach"
)

func TestSocketPath_IsTheSessionsSocketUnderTheSessionDir(t *testing.T) {
	got, err := detach.SocketPath("/home/u", "abc-123")

	if err != nil {
		t.Fatalf("SocketPath() error = %v, want nil", err)
	}
	if got != "/home/u/.omatty/s/abc-123.sock" {
		t.Errorf("SocketPath() = %q, want %q", got, "/home/u/.omatty/s/abc-123.sock")
	}
}

// bind(2) fails past sun_path with an error naming neither the session nor the
// limit, so omatty refuses first and says both. A home directory deep enough to
// trip this is unusual but not impossible - an encrypted volume mounted under a
// long path is the realistic case (#43).
func TestSocketPath_RefusesAPathOverTheLimitNamingIt_issue43(t *testing.T) {
	deep := "/" + strings.Repeat("d", 200)

	_, err := detach.SocketPath(deep, "abc-123")

	if err == nil {
		t.Fatal("SocketPath() with a 200-character home returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "104") {
		t.Errorf("error %q does not name the 104-byte limit", err)
	}
	if !strings.Contains(err.Error(), "abc-123") {
		t.Errorf("error %q does not name the offending session", err)
	}
}

// The pidfile is opened, never bound, so it is not subject to sun_path and
// needs no error return of its own.
func TestPidPath_IsTheSessionsPidfile(t *testing.T) {
	got := detach.PidPath("/home/u", "abc-123")

	if got != "/home/u/.omatty/s/abc-123.pid" {
		t.Errorf("PidPath() = %q, want %q", got, "/home/u/.omatty/s/abc-123.pid")
	}
}
