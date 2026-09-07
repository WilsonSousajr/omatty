package registry

import (
	"fmt"

	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// CreatorOpts is where a Creator puts worktrees and what it forks them from.
// A struct rather than two adjacent string parameters, which are trivially
// swapped at a call site and would fail only at `git worktree add` time (#44).
type CreatorOpts struct {
	WorktreeRoot string
	// BaseBranch forks every worktree from this branch. Empty keeps the
	// existing derivation: whatever branch the project's main checkout is on.
	BaseBranch string
}

// Creator turns a request for a session into a registered Session, creating
// a git worktree when the caller asked for one.
//
//	c := registry.NewCreator(vcs.NewCLI(), registry.CreatorOpts{WorktreeRoot: root}, uuid.NewString)
//	sess, err := c.Create(&state, "omatty", "parser fix", "parser-fix")
type Creator struct {
	git   vcs.Git
	opts  CreatorOpts
	newID func() string
}

// NewCreator returns a Creator. newID is injected so tests get stable ids.
func NewCreator(git vcs.Git, opts CreatorOpts, newID func() string) *Creator {
	return &Creator{git: git, opts: opts, newID: newID}
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

// addWorktree creates sess's worktree, forked from the configured base or
// the branch the main checkout is on, and records that branch as Base (#21).
func (c *Creator) addWorktree(sess *Session, p Project) error {
	base, err := c.base(p.Root)
	if err != nil {
		return fmt.Errorf("registry: reading the base branch of %q: %w", p.Root, err)
	}
	dir := paths.WorktreeDir(c.opts.WorktreeRoot, p.Name, sess.Branch)
	if err := c.git.AddWorktree(p.Root, dir, sess.Branch, base); err != nil {
		return fmt.Errorf("registry: creating worktree %q on branch %q from %q: %w",
			dir, sess.Branch, base, err)
	}
	sess.Dir, sess.Base, sess.Worktree = dir, recordedBase(base), true
	return nil
}

// base is the branch a new worktree forks from: the configured one, or the
// branch the project's main checkout is on (#21, #44). A configured base is
// not asked of git, which is one fork fewer per session.
func (c *Creator) base(root string) (string, error) {
	if c.opts.BaseBranch != "" {
		return c.opts.BaseBranch, nil
	}
	return c.git.CurrentBranch(root)
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
