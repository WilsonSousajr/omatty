package session

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
