// Starting a session: what runs, wrapped by the holder that keeps it alive
// across quit, as a session.Launch the terminal spawns (ADR 0001, "Starting a
// session"). This was internal/supervisor until migration step 5.5 (#653).

package sessions

import (
	"os"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
)

// Launcher builds and starts the claude process for a session.
//
//	l := sessions.NewLauncher(profile, cfg.ClaudeBin, paths.HooksFile(home), home, detach.New(home))
//	launch, err := l.Launch(sess)
//
// One profile per Launcher because M7 has one agent. When a second arrives,
// Start resolves sess.Agent and Command takes the profile as a parameter;
// the change is local to this file, which is what the seam buys (#46).
type Launcher struct {
	profile   agent.Profile
	bin       string
	hooksFile string
	home      string
	// holder keeps the process alive across omatty's own exit. It is a port,
	// not the dtach type, because invariant 4 keeps the binary inside
	// internal/infra/detach and because a machine without dtach gets the
	// Plain holder instead (#43).
	holder Holder
}

// NewLauncher returns a Launcher running profile's agent as bin with
// hooksFile as its settings. home is where the agent keeps transcripts; it
// decides between a fresh start and a resume. holder decides whether the
// process survives quitting omatty.
func NewLauncher(profile agent.Profile, bin, hooksFile, home string, holder Holder) *Launcher {
	return &Launcher{profile: profile, bin: bin, hooksFile: hooksFile, home: home, holder: holder}
}

// Launch returns what omatty starts for a session, built from the profile's
// template: a command line, its environment and its directory, for the
// terminal to spawn (ADR 0001, "Starting a session"; migration step 5.5, #653).
// Built from the profile's template. The flag choice - a fresh start or a resume - is still
// made here, because it is a fact about this session's transcript rather
// than about the agent; which flags express it is the profile's business
// (#36, #46). For claude that is --session-id versus --resume, and --settings
// naming omatty's own file, never the user's (invariant 3).
//
// The claude command built here is then handed to the holder, which under dtach
// returns a client attaching to a master that outlives omatty. The two decisions
// compose rather than interact: the holder never inspects the claude line, and
// the flag choice above is unaware a holder exists (#43).
//
// The conversation is what claude resumes, and the row's ID is what the holder
// names and what the hook reads back from the environment: after /clear the
// two differ, and only the first moves (#316).
func (l *Launcher) Launch(sess Session) (session.Launch, error) {
	conv := sess.ConversationID()
	resume := HasTranscript(l.profile, l.home, sess.Dir, conv)
	argv, err := l.holder.Wrap(sess.ID, l.profile.Command(l.bin, conv, sess.Dir, resume, l.hooksFile))
	if err != nil {
		return session.Launch{}, err
	}
	return session.Launch{Argv: argv, Env: ownedEnv(os.Environ(), sess.ID), Dir: sess.Dir}, nil
}

// ownedEnv is env with session.SessionEnv set to id. A value inherited from an
// outer omatty pane is dropped rather than shadowed, so an omatty running
// inside omatty cannot re-bind the pane it runs in (#316).
func ownedEnv(env []string, id string) []string {
	prefix := session.SessionEnv + "="
	out := make([]string, 0, len(env)+1)
	for _, kv := range env {
		if !strings.HasPrefix(kv, prefix) {
			out = append(out, kv)
		}
	}
	return append(out, prefix+id)
}

// HasTranscript reports whether the profile's agent has written a transcript
// for the session, which is the condition under which it must be resumed
// rather than started (#36, #46).
func HasTranscript(profile agent.Profile, home, dir, sessionID string) bool {
	info, err := os.Stat(profile.TranscriptPath(home, dir, sessionID))
	return err == nil && !info.IsDir()
}

// Reattaching reports whether the session's process is already running from
// an earlier omatty, so that Start will attach rather than launch. The boot
// path asks before starting, because such a pane comes back blank and needs
// a repaint nudge that a fresh claude does not (#191).
//
//	held, err := l.Reattaching(sess.ID)
func (l *Launcher) Reattaching(sessionID string) (bool, error) {
	return l.holder.Held(sessionID)
}
