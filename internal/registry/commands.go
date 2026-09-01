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
		return Project{}, fmt.Errorf("registry: %q is not inside a git repository: %w", dir, err)
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
