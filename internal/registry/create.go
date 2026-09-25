package registry

import (
	"fmt"
	"log/slog"
	"strings"

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
	return c.create(st, project, title, branch, branch != "")
}

// CreateWorktree registers a session on a fresh worktree whether or not a
// branch was named. An empty branch takes the placeholder, which the session's
// first prompt renames (#151) - so ctrl+o N no longer has one string it must
// have before it can start anything.
func (c *Creator) CreateWorktree(st *State, project, title, branch string) (Session, error) {
	return c.create(st, project, title, branch, true)
}

// create is the one registration path both entry points take. worktree is a
// parameter rather than "branch != \"\"" because since #151 those are different
// questions: a worktree session may arrive with no branch named at all.
func (c *Creator) create(st *State, project, title, branch string, worktree bool) (Session, error) {
	p, err := findProject(st, project)
	if err != nil {
		return Session{}, err
	}
	id := c.newID()
	sess := Session{ID: id, Project: project, Title: titleOr(title, id), Dir: p.Root}
	if worktree {
		sess.Branch = branchOr(branch, id)
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
	return c.carry(p, dir)
}

// carry copies the project's gitignored files into the worktree just created,
// before the session is registered and so before claude can start in it -
// whatever runs next must be able to rely on them (#309).
//
// A failure takes the worktree with it. Leaving a half-populated one
// registered would hand claude a directory the operator did not choose, and
// create's contract is that a failure leaves st untouched. The removal's own
// error is logged rather than returned: the carry failure is the one worth
// reporting, and hiding it behind a cleanup error would bury the cause.
func (c *Creator) carry(p Project, dir string) error {
	if len(p.Carry) == 0 {
		return nil
	}
	if err := CarryInto(dir, p.Root, p.Carry); err != nil {
		if rmErr := c.git.RemoveWorktree(p.Root, dir); rmErr != nil {
			slog.Error("removing a worktree whose carry failed",
				"project", p.Name, "worktree", dir, "err", rmErr)
		}
		return err
	}
	return nil
}

// branchOr is the branch to put a worktree on: the slug of what the operator
// typed, or a placeholder for one created before the work it would describe
// exists.
//
// Slug, not TrimSpace, and that is a fix rather than a tidy-up: this name
// reaches `git worktree add -b` and a directory name, and until #151 a typed
// branch was the one untrusted string that got there unfiltered - looser than
// the filter #127 step 2 already applies to model output, which is exactly
// what naming.go says must never happen.
func branchOr(branch, id string) string {
	if s := Slug(branch); s != "" {
		return s
	}
	return PlaceholderBranch(id)
}

// titleOr is the name to register a session under: what the operator typed,
// or a placeholder for one created before the work it would describe exists.
// The placeholder is replaced by the session's first prompt (#127).
func titleOr(title, id string) string {
	if strings.TrimSpace(title) == "" {
		return PlaceholderTitle(id)
	}
	return title
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
