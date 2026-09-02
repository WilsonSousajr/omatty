// Package supervisor owns the lifecycle of the claude process behind each
// session.
package supervisor

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// Launcher builds and starts the claude process for a session.
//
//	l := supervisor.NewLauncher("claude", paths.HooksFile(home), home)
//	term, err := l.Start(termwrap.Start, sess, 80, 24)
type Launcher struct {
	bin       string
	hooksFile string
	home      string
}

// NewLauncher returns a Launcher invoking bin with hooksFile as its settings.
// home is where claude keeps transcripts; it decides between a fresh start
// and a resume.
func NewLauncher(bin, hooksFile, home string) *Launcher {
	return &Launcher{bin: bin, hooksFile: hooksFile, home: home}
}

// Command returns the process omatty starts for a session.
//
// A session that has never spoken starts with --session-id, which lets omatty
// choose the uuid and so know the transcript path (invariant 2). Once a
// transcript exists claude refuses that flag - "Session ID <uuid> is already
// in use" - because the transcript itself is the claim; there is no lock file.
// So a session with a transcript is started with --resume instead (issue #36).
// Either way --settings names omatty's own file, never the user's (invariant 3).
func (l *Launcher) Command(sessionID, dir string) *exec.Cmd {
	flag := "--session-id"
	if HasTranscript(l.home, dir, sessionID) {
		flag = "--resume"
	}
	cmd := exec.Command(l.bin, flag, sessionID, "--settings", l.hooksFile)
	cmd.Dir = dir
	return cmd
}

// HasTranscript reports whether claude has written a transcript for the
// session, which is the condition under which it must be resumed rather than
// started (issue #36).
func HasTranscript(home, dir, sessionID string) bool {
	info, err := os.Stat(paths.Transcript(home, dir, sessionID))
	return err == nil && !info.IsDir()
}

// Start launches the session's process inside a w by h embedded terminal.
func (l *Launcher) Start(
	f termwrap.Factory, sess registry.Session, w, h int,
) (termwrap.Terminal, error) {
	term, err := f(w, h, l.Command(sess.ID, sess.Dir))
	if err != nil {
		return nil, fmt.Errorf("supervisor: starting session %s in %q: %w", sess.ID, sess.Dir, err)
	}
	return term, nil
}
