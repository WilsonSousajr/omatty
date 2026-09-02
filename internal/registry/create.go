package registry

import (
	"fmt"

	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// Creator turns a request for a session into a registered Session, creating
// a git worktree when the caller asked for one.
//
//	c := registry.NewCreator(vcs.NewCLI(), home, func() string { return uuid.NewString() })
//	sess, err := c.Create(&state, "omatty", "parser fix", "parser-fix")
type Creator struct {
	git   vcs.Git
	home  string
	newID func() string
}

// NewCreator returns a Creator. newID is injected so tests get stable ids.
func NewCreator(git vcs.Git, home string, newID func() string) *Creator {
	return &Creator{git: git, home: home, newID: newID}
}

// Create registers a session on st and returns it. An empty branch runs the
// session in the project's main checkout; otherwise omatty creates a worktree
// at paths.WorktreeDir. On any failure st is left untouched.
func (c *Creator) Create(st *State, project, title, branch string) (Session, error) {
	p, err := findProject(st, project)
	if err != nil {
		return Session{}, err
	}
	dir, err := c.resolveDir(p, branch)
	if err != nil {
		return Session{}, err
	}
	sess := Session{
		ID: c.newID(), Project: project, Title: title,
		Dir: dir, Branch: branch, Worktree: branch != "",
	}
	st.Sessions = append(st.Sessions, sess)
	return sess, nil
}

// resolveDir returns the working directory for a new session, creating the
// worktree as a side effect when branch is non-empty.
func (c *Creator) resolveDir(p Project, branch string) (string, error) {
	if branch == "" {
		return p.Root, nil
	}
	dir := paths.WorktreeDir(c.home, p.Name, branch)
	if err := c.git.AddWorktree(p.Root, dir, branch); err != nil {
		return "", fmt.Errorf("registry: creating worktree %q on branch %q: %w", dir, branch, err)
	}
	return dir, nil
}

func findProject(st *State, name string) (Project, error) {
	for _, p := range st.Projects {
		if p.Name == name {
			return p, nil
		}
	}
	return Project{}, fmt.Errorf(
		"registry: no project named %q (known projects: %v)", name, projectNames(st))
}

func projectNames(st *State) []string {
	names := make([]string, 0, len(st.Projects))
	for _, p := range st.Projects {
		names = append(names, p.Name)
	}
	return names
}
