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
	if !strings.Contains(err.Error(), "103") {
		t.Errorf("error %q does not name the 103-byte limit", err)
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

// suffix is what SocketPath appends to a home, so a test can build a path of an
// exact length rather than asserting against a magic number.
const suffix = "/.omatty/s/abc-123.sock"

// Regression, issue #43: the guard was `len(path) > 104` against a 104-byte
// limit, which admitted exactly the one path it exists to catch. sun_path's 104
// bytes include the terminating NUL, so a 104-character path does not fit;
// dtach applies the same arithmetic and refused it with its own message, naming
// neither the session nor the limit - the outcome SocketPath promises to
// prevent.
func TestSocketPath_RefusesThePathThatIsExactlyTheStructSize_issue43(t *testing.T) {
	home := "/" + strings.Repeat("h", 104-len(suffix)-1)

	got, err := detach.SocketPath(home, "abc-123")

	if err == nil {
		t.Fatalf("SocketPath() = %q (%d bytes), want an error: sun_path's 104 bytes include the NUL",
			got, len(got))
	}
}

// The other side of the same boundary. A guard that is exact in one direction
// only either admits paths that cannot bind or refuses homes that work, and
// both are worse than the check being absent, because both are silent.
func TestSocketPath_AcceptsTheLongestPathThatFits_issue43(t *testing.T) {
	home := "/" + strings.Repeat("h", 103-len(suffix)-1)

	got, err := detach.SocketPath(home, "abc-123")

	if err != nil {
		t.Fatalf("SocketPath() error = %v, want nil for a path of exactly 103 bytes", err)
	}
	if len(got) != 103 {
		t.Fatalf("the test built a %d-byte path, want 103; the suffix constant is wrong", len(got))
	}
}
