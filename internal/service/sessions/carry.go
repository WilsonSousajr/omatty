// Carrying a project's gitignored files into a new worktree (#309).
//
// A worktree is a clean checkout, so everything git does not track is absent
// from it: .env, local certificates, generated config. M9 made the gate the
// product, and a gate step that fails because .env is missing is the gate
// being wrong about the code - a red card the operator learns to ignore,
// which is worse than no card. So the copy runs before the session starts,
// not as a hook the operator remembers to write.
//
// The list lives in state.json as Project.Carry, beside Project.Gate, rather
// than in a repository file the way ccmanager's .worktreeinclude and fleet's
// .fleet.json do. Three reasons: it is how every other per-project setting
// here already works, so there is one place to look; the empty value is
// derivable, so no migration and Version stays 1 (invariant 9); and a cloned
// repository must not get to decide which files are copied off this
// operator's disk. The cost is real and worth naming - the list is not shared
// with a team, and each person sets their own.

package sessions

// SetCarry records the gitignored paths copied into each new worktree of a
// project, replacing whatever it had.
//
//	err := sessions.SetCarry(store, "omatty", []string{".env", "certs"})
func SetCarry(s StateStore, project string, paths []string) error {
	return editCarry(s, project, paths)
}

// ClearCarry forgets a project's carry list, returning it to "nothing to
// carry" - which is the nil Project.Carry already means.
func ClearCarry(s StateStore, project string) error {
	return editCarry(s, project, nil)
}

// editCarry is the load-find-write both commands share, as editGate is for
// the gate.
func editCarry(s StateStore, project string, paths []string) error {
	st, err := s.Load()
	if err != nil {
		return err
	}
	if _, err := findProject(&st, project); err != nil {
		return err
	}
	for i := range st.Projects {
		if st.Projects[i].Name == project {
			st.Projects[i].Carry = paths
		}
	}
	return s.Save(st)
}
