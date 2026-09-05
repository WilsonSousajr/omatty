package registry

import (
	"fmt"
	"path/filepath"
	"strings"
)

// RepoRooter is the slice of vcs.Git the registry needs. Declared narrow so a
// fake in a caller's tests carries one method rather than nine - cmd/omatty's
// adapters could not be tested at all while they demanded the concrete *vcs.CLI
// (#91).
type RepoRooter interface {
	RepoRoot(dir string) (string, error)
}

// AddProject registers the git repository containing dir, naming it after
// the repository's own directory. It is the whole of `omatty add`.
//
//	p, err := registry.AddProject(store, vcs.NewCLI(), cwd)
func AddProject(s *Store, git RepoRooter, dir string) (Project, error) {
	root, err := git.RepoRoot(dir)
	if err != nil {
		// Deliberately does not assert the cause: the path may be missing, a
		// file, or a directory that simply is not a repository (issue #29).
		return Project{}, fmt.Errorf("registry: cannot register %q: %w", dir, err)
	}
	st, err := s.Load()
	if err != nil {
		return Project{}, err
	}
	p := Project{Name: filepath.Base(root), Root: root}
	// The name is the key sessions look up, so a collision would silently
	// attach new sessions to the wrong repository.
	if existing, err := findProject(&st, p.Name); err == nil {
		return Project{}, fmt.Errorf(
			"registry: project %q is already registered at %q", p.Name, existing.Root)
	}
	st.Projects = append(st.Projects, p)
	return p, s.Save(st)
}

// Registration is what became of one root RegisterAll was given. Project is
// the row that was actually written, so a caller never has to reconstruct it.
type Registration struct {
	Root    string
	Project Project
	Err     error
}

// RegisterAll registers each root, carrying on past a failure so one collision
// does not abandon the rest of a bulk pick. AddProject refuses a duplicate
// *name* even when the roots differ, which one repository at a time is a rare
// annoyance and in bulk is not (#91).
//
// It exists so `omatty discover` and the TUI picker share one algorithm. They
// had a copy each - the same loop in cmd/omatty and in ui - which invariant 10
// forbids and which `dupl` cannot see across packages, so a change to the
// collision policy would have had to be made twice or the two would disagree.
//
//	for _, r := range registry.RegisterAll(store, git, roots) { ... }
func RegisterAll(s *Store, git RepoRooter, roots []string) []Registration {
	out := make([]Registration, 0, len(roots))
	for _, root := range roots {
		p, err := AddProject(s, git, root)
		out = append(out, Registration{Root: root, Project: p, Err: err})
	}
	return out
}

// RenameSession retitles a session in place. The title is display-only, so
// this touches nothing that relaunching a session depends on (invariant 9):
// it is a state.json edit and a sidebar rebuild (#41).
//
//	err := registry.RenameSession(store, sess.ID, "parser-fix")
func RenameSession(s *Store, id, title string) error {
	// A blank title would leave a sidebar row with nothing to aim at. The check
	// is on the trimmed title, not on title == "": a name of nothing but spaces
	// renders exactly as empty a row, and passed the old guard (#41).
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf(
			"registry: session %q: title %q is blank, want a name with a non-space character", id, title)
	}
	st, err := s.Load()
	if err != nil {
		return err
	}
	i, err := indexOfSession(&st, id)
	if err != nil {
		return err
	}
	st.Sessions[i].Title = title
	return s.Save(st)
}

// RemoveSession drops a session from the registry and returns it, so the
// caller can decide from its own fields whether a worktree may be deleted
// (#40). The transcript on disk is untouched: this forgets a session, it does
// not destroy its history.
//
//	sess, err := registry.RemoveSession(store, id)
//	if err == nil && sess.Worktree { /* git worktree remove sess.Dir */ }
func RemoveSession(s *Store, id string) (Session, error) {
	st, err := s.Load()
	if err != nil {
		return Session{}, err
	}
	i, err := indexOfSession(&st, id)
	if err != nil {
		return Session{}, err
	}
	sess := st.Sessions[i]
	st.Sessions = append(st.Sessions[:i], st.Sessions[i+1:]...)
	return sess, s.Save(st)
}

// indexOfSession locates a session by id. It returns an index rather than a
// Session because callers mutate the one inside the state they are about to
// save; a copy would be written back over.
//
// The miss returns -1, not 0: a caller that dropped the error would otherwise
// rename or archive whichever session happens to sit first in state.json, and
// silent data corruption is worse than the immediate panic an out-of-range
// index gives (#41).
func indexOfSession(st *State, id string) (int, error) {
	for i := range st.Sessions {
		if st.Sessions[i].ID == id {
			return i, nil
		}
	}
	return -1, fmt.Errorf(
		"registry: no session with id %q (known sessions: %v)", id, sessionIDs(st))
}

func sessionIDs(st *State) []string {
	ids := make([]string, 0, len(st.Sessions))
	for _, sess := range st.Sessions {
		ids = append(ids, sess.ID)
	}
	return ids
}

// AddSession creates and persists a session. It is the whole of `omatty new`.
// State is saved only after the session is fully created, so a failed
// worktree leaves nothing behind.
func AddSession(s *Store, c *Creator, project, title, branch string) (Session, error) {
	st, err := s.Load()
	if err != nil {
		return Session{}, err
	}
	sess, err := c.Create(&st, project, title, branch)
	if err != nil {
		return Session{}, err
	}
	return sess, s.Save(st)
}
