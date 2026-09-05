package registry

import (
	"fmt"
	"path/filepath"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// AddProject registers the git repository containing dir, naming it after
// the repository's own directory. It is the whole of `omatty add`.
//
//	p, err := registry.AddProject(store, vcs.NewCLI(), cwd)
func AddProject(s *Store, git vcs.Git, dir string) (Project, error) {
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

// RenameSession retitles a session in place. The title is display-only, so
// this touches nothing that relaunching a session depends on (invariant 9):
// it is a state.json edit and a sidebar rebuild (#41).
//
//	err := registry.RenameSession(store, sess.ID, "parser-fix")
func RenameSession(s *Store, id, title string) error {
	// An empty title would leave a blank sidebar row with nothing to aim at.
	if title == "" {
		return fmt.Errorf("registry: session %q: title is empty, want a non-blank name", id)
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

// indexOfSession locates a session by id. It returns an index rather than a
// Session because callers mutate the one inside the state they are about to
// save; a copy would be written back over.
func indexOfSession(st *State, id string) (int, error) {
	for i := range st.Sessions {
		if st.Sessions[i].ID == id {
			return i, nil
		}
	}
	return 0, fmt.Errorf(
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
