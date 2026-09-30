// Package sessions is the session service: the commands that add, adopt,
// create, rename, rebind, fold and remove projects and sessions, over a
// StateStore port that internal/infra/store implements. It was
// internal/registry until migration step 5.4 (ADR 0001, #653); the record it
// edits is internal/domain/session's.
//
//	err := sessions.RenameSession(store, id, "parser fix")
package sessions
