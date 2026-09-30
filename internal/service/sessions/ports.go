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
