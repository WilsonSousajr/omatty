package sessions

// StateStore loads and saves the registry's State (ADR 0001's port). Every
// command here takes one; internal/infra/store implements it over
// ~/.omatty/state.json, because reading and writing a file is infra's
// business (migration step 5.4, #653).
//
//	err := sessions.RenameSession(store.NewStore(paths.StateFile(home)), id, "parser fix")
type StateStore interface {
	Load() (State, error)
	Save(State) error
}

// Worktrees is the slice of git a Creator needs: the branch a checkout is on,
// and adding and removing a worktree (ADR 0001's port, migration step 5.4,
// #653). internal/infra/vcs's CLI implements it.
//
//	c := sessions.NewCreator(vcs.NewCLI(), opts, uuid.NewString)
type Worktrees interface {
	CurrentBranch(dir string) (string, error)
	AddWorktree(repoRoot, dir, branch, base string) error
	RemoveWorktree(repoRoot, dir string) error
}
