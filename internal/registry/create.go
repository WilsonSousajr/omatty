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
	sess := Session{ID: c.newID(), Project: project, Title: title, Dir: p.Root, Branch: branch}
	if branch != "" {
		if err := c.addWorktree(&sess, p); err != nil {
			return Session{}, err
		}
	}
	st.Sessions = append(st.Sessions, sess)
	return sess, nil
}

// addWorktree creates sess's worktree, forked from the branch the main
// checkout is on, and records that branch as Base (#21).
func (c *Creator) addWorktree(sess *Session, p Project) error {
	base, err := c.git.CurrentBranch(p.Root)
	if err != nil {
		return fmt.Errorf("registry: reading the base branch of %q: %w", p.Root, err)
	}
	dir := paths.WorktreeDir(c.home, p.Name, sess.Branch)
	if err := c.git.AddWorktree(p.Root, dir, sess.Branch, base); err != nil {
		return fmt.Errorf("registry: creating worktree %q on branch %q from %q: %w",
			dir, sess.Branch, base, err)
	}
	sess.Dir, sess.Base, sess.Worktree = dir, recordedBase(base), true
	return nil
}

// recordedBase drops git's literal "HEAD" for a detached checkout: stored, it
// would make review diff the worktree against itself.
func recordedBase(base string) string {
	if base == "HEAD" {
		return ""
	}
	return base
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
