// Package supervisor owns the lifecycle of the claude process behind each
// session.
package supervisor

import (
	"fmt"
	"os/exec"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// Launcher builds and starts the claude process for a session.
//
//	l := supervisor.NewLauncher("claude", paths.HooksFile(home))
//	term, err := l.Start(termwrap.Start, sess, 80, 24)
type Launcher struct {
	bin       string
	hooksFile string
}

// NewLauncher returns a Launcher invoking bin with hooksFile as its settings.
func NewLauncher(bin, hooksFile string) *Launcher {
	return &Launcher{bin: bin, hooksFile: hooksFile}
}

// Command returns the process omatty starts for a session.
//
// --session-id lets omatty compute the transcript path (invariant 2) and
// resume after a crash; --settings keeps hooks in omatty's own file so the
// user's ~/.claude/settings.json is untouched (invariant 3).
func (l *Launcher) Command(sessionID, dir string) *exec.Cmd {
	cmd := exec.Command(l.bin, "--session-id", sessionID, "--settings", l.hooksFile)
	cmd.Dir = dir
	return cmd
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
