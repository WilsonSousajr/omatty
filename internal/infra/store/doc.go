// Package store is omatty's registry on disk: ~/.omatty/state.json, loaded
// and saved atomically so a crash cannot leave a truncated file that strands
// running sessions (invariant 9), and the copy of a project's gitignored
// files into a new worktree (#309). It implements internal/service/sessions'
// StateStore port (ADR 0001, migration step 5.4, #653).
//
//	s := store.NewStore(paths.StateFile(home))
//	state, err := s.Load(ctx)
package store
