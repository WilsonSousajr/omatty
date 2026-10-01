package session

// SessionEnv names the environment variable the launcher sets on every
// session's process to that session's registry id. The hook inherits it from
// claude, which is how a SessionStart for a new conversation - a /clear -
// names the pane it belongs to: the payload's cwd cannot, since two panes may
// share a directory (#316). Here since migration step 5.5 (#653): the launcher
// that sets it is the session service, and the hook that reads it is infra.
const SessionEnv = "OMATTY_SESSION"

// Launch is what starting a session runs: the agent's command line, filled in
// and wrapped by the holder that keeps it alive across quit, with the
// environment and directory it runs in (ADR 0001, "Starting a session";
// migration step 5.5, #653). The session service decides what runs; the
// terminal component spawns it; neither imports the other's libraries.
//
//	l := session.Launch{Argv: []string{"claude", "--resume", id}, Env: env, Dir: sess.Dir}
type Launch struct {
	Argv []string
	Env  []string
	Dir  string
}
