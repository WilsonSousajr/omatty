package sessions

import "context"

// StateStore loads and saves the registry's State (ADR 0001's port). Every
// command here takes one; internal/infra/store implements it over
// ~/.omatty/state.json, because reading and writing a file is infra's
// business (migration step 5.4, #653).
//
//	err := sessions.RenameSession(ctx, store.NewStore(paths.StateFile(home)), id, "parser fix")
type StateStore interface {
	Load(ctx context.Context) (State, error)
	Save(ctx context.Context, st State) error
}

// Worktrees is the slice of git a Creator needs: the branch a checkout is on,
// and adding and removing a worktree (ADR 0001's port, migration step 5.4,
// #653). internal/infra/vcs's CLI implements it.
//
//	c := sessions.NewCreator(vcs.NewCLI().Contextual(), opts, uuid.NewString)
type Worktrees interface {
	CurrentBranch(ctx context.Context, dir string) (string, error)
	AddWorktree(ctx context.Context, repoRoot, dir, branch, base string) error
	RemoveWorktree(ctx context.Context, repoRoot, dir string) error
}
