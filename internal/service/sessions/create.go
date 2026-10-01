package sessions

import (
	"context"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"time"

	"fmt"
	"log/slog"
	"strings"
)

// CreatorOpts is where a Creator puts worktrees and what it forks them from.
// A struct rather than two adjacent string parameters, which are trivially
// swapped at a call site and would fail only at `git worktree add` time (#44).
type CreatorOpts struct {
	WorktreeRoot string
	// WorktreeDir is where a project's worktree for a branch goes under
	// WorktreeRoot. Injected, because where omatty keeps files is infra's
	// knowledge (ADR 0001: paths is infra; migration step 5.4, #653): cmd
	// passes internal/infra/paths' WorktreeDir.
	WorktreeDir func(root, project, branch string) string
	// BaseBranch forks every worktree from this branch. Empty keeps the
	// existing derivation: whatever branch the project's main checkout is on.
	BaseBranch string
	// Clock stamps Session.Started (#332). Nil is the wall clock, so nothing
	// has to pass one; it is here rather than as a constructor parameter
	// because an option struct can gain a field without touching a call site.
	Clock func() time.Time
	// Carry copies a project's gitignored files into a new worktree (#309).
	// Injected, because copying files is infra's business (ADR 0001, migration
	// step 5.4, #653): cmd passes internal/infra/store's CarryInto.
	Carry func(dir, root string, paths []string) error
}

// Creator turns a request for a session into a registered Session, creating
// a git worktree when the caller asked for one.
//
//	c := sessions.NewCreator(vcs.NewCLI().Contextual(), sessions.CreatorOpts{WorktreeRoot: root, WorktreeDir: paths.WorktreeDir}, uuid.NewString)
//	sess, err := c.Create(&state, "omatty", "parser fix", "parser-fix")
type Creator struct {
	git   Worktrees
	opts  CreatorOpts
	newID func() string
}

// NewCreator returns a Creator. newID is injected so tests get stable ids.
func NewCreator(git Worktrees, opts CreatorOpts, newID func() string) *Creator {
	return &Creator{git: git, opts: opts, newID: newID}
}

// Create registers a session on st and returns it. An empty branch runs the
// session in the project's main checkout; otherwise omatty creates a worktree
// at CreatorOpts.WorktreeDir. On any failure st is left untouched.
func (c *Creator) Create(ctx context.Context, st *session.State, project, title, branch string) (session.Session, error) {
	return c.create(ctx, st, project, title, branch, branch != "")
}

// CreateWorktree registers a session on a fresh worktree whether or not a
// branch was named. An empty branch takes the placeholder, which the session's
// first prompt renames (#151) - so ctrl+o N no longer has one string it must
// have before it can start anything.
func (c *Creator) CreateWorktree(ctx context.Context, st *session.State, project, title, branch string) (session.Session, error) {
	return c.create(ctx, st, project, title, branch, true)
}

// create is the one registration path both entry points take. worktree is a
// parameter rather than "branch != \"\"" because since #151 those are different
// questions: a worktree session may arrive with no branch named at all.
func (c *Creator) create(ctx context.Context, st *session.State, project, title, branch string, worktree bool) (session.Session, error) {
	p, err := findProject(st, project)
	if err != nil {
		return session.Session{}, err
	}
	id := c.newID()
	sess := session.Session{ID: id, Project: project, Title: titleOr(title, id), Dir: p.Root,
		Started: c.now()}
	if worktree {
		sess.Branch = branchOr(branch, id)
		if err := c.addWorktree(ctx, &sess, p); err != nil {
			return session.Session{}, err
		}
	}
	st.Sessions = append(st.Sessions, sess)
	return sess, nil
}

// addWorktree creates sess's worktree, forked from the configured base or
// the branch the main checkout is on, and records that branch as Base (#21).
func (c *Creator) addWorktree(ctx context.Context, sess *session.Session, p session.Project) error {
	base, err := c.base(ctx, p.Root)
	if err != nil {
		return fmt.Errorf("registry: reading the base branch of %q: %w", p.Root, err)
	}
	if c.opts.WorktreeDir == nil {
		return fmt.Errorf("registry: project %q wants a worktree but no worktree path is wired", p.Name)
	}
	dir := c.opts.WorktreeDir(c.opts.WorktreeRoot, p.Name, sess.Branch)
	if err := c.git.AddWorktree(ctx, p.Root, dir, sess.Branch, base); err != nil {
		return fmt.Errorf("registry: creating worktree %q on branch %q from %q: %w",
			dir, sess.Branch, base, err)
	}
	sess.Dir, sess.Base, sess.Worktree = dir, recordedBase(base), true
	return c.carry(ctx, p, dir)
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
func (c *Creator) carry(ctx context.Context, p session.Project, dir string) error {
	if len(p.Carry) == 0 {
		return nil
	}
	if err := c.copyCarried(dir, p); err != nil {
		if rmErr := c.git.RemoveWorktree(ctx, p.Root, dir); rmErr != nil {
			slog.Error("removing a worktree whose carry failed",
				"project", p.Name, "worktree", dir, "err", rmErr)
		}
		return err
	}
	return nil
}

// copyCarried runs the injected copy, or refuses when none was wired: a
// project that lists files to carry and silently got none would fail its gate
// for a reason that has nothing to do with the code (#309).
func (c *Creator) copyCarried(dir string, p session.Project) error {
	if c.opts.Carry == nil {
		return fmt.Errorf("registry: project %q lists %d paths to carry but no copier is wired", p.Name, len(p.Carry))
	}
	return c.opts.Carry(dir, p.Root, p.Carry)
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
	if s := session.Slug(branch); s != "" {
		return s
	}
	return session.PlaceholderBranch(id)
}

// titleOr is the name to register a session under: what the operator typed,
// or a placeholder for one created before the work it would describe exists.
// The placeholder is replaced by the session's first prompt (#127).
func titleOr(title, id string) string {
	if strings.TrimSpace(title) == "" {
		return session.PlaceholderTitle(id)
	}
	return title
}

// base is the branch a new worktree forks from: the configured one, or the
// branch the project's main checkout is on (#21, #44). A configured base is
// not asked of git, which is one fork fewer per session.
func (c *Creator) base(ctx context.Context, root string) (string, error) {
	if c.opts.BaseBranch != "" {
		return c.opts.BaseBranch, nil
	}
	return c.git.CurrentBranch(ctx, root)
}

// recordedBase drops git's literal "HEAD" for a detached checkout: stored, it
// would make review diff the worktree against itself.
func recordedBase(base string) string {
	if base == "HEAD" {
		return ""
	}
	return base
}

func findProject(st *session.State, name string) (session.Project, error) {
	for _, p := range st.Projects {
		if p.Name == name {
			return p, nil
		}
	}
	return session.Project{}, fmt.Errorf(
		"registry: no project named %q (known projects: %v)", name, projectNames(st))
}

func projectNames(st *session.State) []string {
	names := make([]string, 0, len(st.Projects))
	for _, p := range st.Projects {
		names = append(names, p.Name)
	}
	return names
}

// now is the Creator's clock, or the wall clock when none was injected.
func (c *Creator) now() time.Time {
	if c.opts.Clock == nil {
		return time.Now()
	}
	return c.opts.Clock()
}
