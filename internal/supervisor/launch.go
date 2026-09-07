// Package supervisor owns the lifecycle of the claude process behind each
// session.
package supervisor

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/WilsonSousajr/omatty/internal/agent"
	"github.com/WilsonSousajr/omatty/internal/detach"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// Launcher builds and starts the claude process for a session.
//
//	l := supervisor.NewLauncher(agent.Claude(), cfg.ClaudeBin, paths.HooksFile(home), home, detach.New(home))
//	term, err := l.Start(termwrap.Start, sess, 80, 24)
//
// One profile per Launcher because M7 has one agent. When a second arrives,
// Start resolves sess.Agent and Command takes the profile as a parameter;
// the change is local to this file, which is what the seam buys (#46).
type Launcher struct {
	profile   agent.Profile
	bin       string
	hooksFile string
	home      string
	// holder keeps the process alive across omatty's own exit. It is an
	// interface, not the dtach type, because invariant 4 keeps the binary
	// inside internal/detach and because a machine without dtach gets the
	// Plain holder instead (#43).
	holder detach.Holder
}

// NewLauncher returns a Launcher running profile's agent as bin with
// hooksFile as its settings. home is where the agent keeps transcripts; it
// decides between a fresh start and a resume. holder decides whether the
// process survives quitting omatty.
func NewLauncher(profile agent.Profile, bin, hooksFile, home string, holder detach.Holder) *Launcher {
	return &Launcher{profile: profile, bin: bin, hooksFile: hooksFile, home: home, holder: holder}
}

// Command returns the process omatty starts for a session, built from the
// profile's template. The flag choice - a fresh start or a resume - is still
// made here, because it is a fact about this session's transcript rather
// than about the agent; which flags express it is the profile's business
// (#36, #46). For claude that is --session-id versus --resume, and --settings
// naming omatty's own file, never the user's (invariant 3).
//
// The claude command built here is then handed to the holder, which under dtach
// returns a client attaching to a master that outlives omatty. The two decisions
// compose rather than interact: the holder never inspects the claude line, and
// the flag choice above is unaware a holder exists (#43).
func (l *Launcher) Command(sessionID, dir string) (*exec.Cmd, error) {
	resume := HasTranscript(l.profile, l.home, dir, sessionID)
	args := l.profile.Command(l.bin, sessionID, dir, resume, l.hooksFile)
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	return l.holder.Wrap(sessionID, cmd)
}

// HasTranscript reports whether the profile's agent has written a transcript
// for the session, which is the condition under which it must be resumed
// rather than started (#36, #46).
func HasTranscript(profile agent.Profile, home, dir, sessionID string) bool {
	info, err := os.Stat(profile.TranscriptPath(home, dir, sessionID))
	return err == nil && !info.IsDir()
}

// Start launches the session's process inside a w by h embedded terminal.
func (l *Launcher) Start(
	f termwrap.Factory, sess registry.Session, w, h int,
) (termwrap.Terminal, error) {
	cmd, err := l.Command(sess.ID, sess.Dir)
	if err != nil {
		return nil, err
	}
	term, err := f(w, h, cmd)
	if err != nil {
		return nil, fmt.Errorf("supervisor: starting session %s in %q: %w", sess.ID, sess.Dir, err)
	}
	return term, nil
}
