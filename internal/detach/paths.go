package detach

import (
	"fmt"
	"path/filepath"

	"github.com/WilsonSousajr/omatty/internal/paths"
)

// maxSocketPath is the kernel's cap on a unix socket path. sun_path is 104
// bytes on macOS and 108 on Linux; the smaller one is the budget omatty has to
// fit inside, because a socket that binds on one platform and not the other is
// worse than one that is refused on both.
const maxSocketPath = 104

// SocketPath returns the dtach socket for a session.
//
//	sock, err := detach.SocketPath(home, sess.ID) // ~/.omatty/s/<uuid>.sock
//
// It returns an error rather than an over-long path because bind(2) fails past
// the cap with an error naming neither the session nor the limit, leaving an
// operator with a session that will not start and nothing to act on (#43).
func SocketPath(home, sessionID string) (string, error) {
	path := filepath.Join(paths.SessionDir(home), sessionID+".sock")
	if len(path) > maxSocketPath {
		return "", fmt.Errorf(
			"detach: session %s: socket path %q is %d bytes, over the %d-byte limit a unix socket allows",
			sessionID, path, len(path), maxSocketPath)
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
