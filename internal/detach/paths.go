package detach

import (
	"fmt"
	"path/filepath"

	"github.com/WilsonSousajr/omatty/internal/paths"
)

// maxSocketPath is the kernel's cap on a unix socket path. sun_path is 104
// bytes on macOS and 108 on Linux, and in both cases that size *includes* the
// terminating NUL - so the longest usable path is one less than the smaller of
// the two. dtach applies the same arithmetic to the name it is handed
// (`strlen(name) > sizeof(sun_path) - 1`), and the smaller platform is the
// budget omatty has to fit inside, because a socket that binds on one platform
// and not the other is worse than one that is refused on both.
//
// It was 104 with a `>` test, which admitted the single boundary case the
// guard exists to catch: a 104-byte path passed omatty and was then refused by
// dtach, with dtach's own message (#43).
const maxSocketPath = 103

// SocketPath returns the dtach socket for a session.
//
//	sock, err := detach.SocketPath(home, sess.ID) // ~/.omatty/s/<uuid>.sock
//
// It returns an error rather than an over-long path because bind(2) fails past
// the cap with an error naming neither the session nor the limit, leaving an
// operator with a session that will not start and nothing to act on (#43).
func SocketPath(home, sessionID string) (string, error) {
	return socketPathWithin(home, sessionID, maxSocketPath)
}

// socketPathWithin is SocketPath with the limit named, so NewDtachCapped can
// vary it. Without that seam a test of the limit has to find a directory short
// enough to sit under it, which t.TempDir is not on macOS.
func socketPathWithin(home, sessionID string, limit int) (string, error) {
	path := filepath.Join(paths.SessionDir(home), sessionID+".sock")
	if len(path) > limit {
		return "", fmt.Errorf(
			"detach: session %s: socket path %q is %d bytes, over the %d-byte limit a unix socket allows",
			sessionID, path, len(path), limit)
	}
	return path, nil
}

// PidPath returns the file recording the pid of the claude behind a session.
//
//	pid := detach.PidPath(home, sess.ID) // ~/.omatty/s/<uuid>.pid
//
// No length check: this file is opened, never bound, so sun_path does not
// apply to it.
func PidPath(home, sessionID string) string {
	return filepath.Join(paths.SessionDir(home), sessionID+".pid")
}
