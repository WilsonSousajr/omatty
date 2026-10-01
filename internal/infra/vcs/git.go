// Package vcs is omatty's only route to git.
//
// Invariant 4: no other package shells out to git or imports a git library.
// Worktrees go through the git CLI rather than go-git, whose linked-worktree
// support is v6-experimental and implements only add and remove.
package vcs

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"github.com/WilsonSousajr/omatty/internal/domain/review"
	"io/fs"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Git is the surface omatty uses. Fake it in tests; do not fake exec.Cmd.
type Git interface {
	RepoRoot(dir string) (string, error)
	MainCheckout(dir string) (string, error)
	CurrentBranch(dir string) (string, error)
	AddWorktree(repoRoot, dir, branch, base string) error
	RemoveWorktree(repoRoot, dir string) error
	// RenameBranch renames a branch in place, leaving any worktree that has
	// it checked out exactly where it is (#151).
	RenameBranch(repoRoot, old, name string) error
	// CommitsOnBranch counts what branch has that base does not, which is how
	// a branch nobody has committed to yet is told from one that is in use.
	CommitsOnBranch(repoRoot, base, branch string) (int, error)
	// MergeBase returns the commit where ref and dir's HEAD diverged.
	MergeBase(dir, ref string) (string, error)
	// CommitExists reports whether ref still names a commit, which a merged
	// and deleted base branch does not (#684).
	CommitExists(dir, ref string) (bool, error)
	// Diff returns the unified diff of dir's working tree against commit, so
	// committed and uncommitted changes appear as one diff (#21).
	Diff(dir, commit string) (string, error)
	// Shortstat summarises the working tree against commit: files changed,
	// lines added and removed. A clean tree is the zero value (#180).
	Shortstat(dir, commit string) (Shortstat, error)
	// Untracked lists files git does not track, honouring .gitignore.
	Untracked(dir string) ([]string, error)
	// UntrackedDiff renders one untracked file as an all-additions diff.
	UntrackedDiff(dir, path string) (string, error)
	// ListFiles lists tracked and untracked files under dir, honouring
	// .gitignore, sorted (#24).
	ListFiles(dir string) ([]string, error)
	// SnapshotTree writes dir's working tree - tracked and untracked files,
	// honouring .gitignore - as a tree object, without touching HEAD, the
	// index or the stash (#311).
	SnapshotTree(dir string) (string, error)
	// SetTurnRef, TurnRef and DeleteTurnRef keep one session's turn baseline
	// under refs/omatty/turn/<id> (#311).
	SetTurnRef(dir, id, tree string) error
	TurnRef(dir, id string) (string, bool, error)
	DeleteTurnRef(dir, id string) error
	// DiffTrees is the unified diff between two trees (#311).
	DiffTrees(dir, from, to string) (string, error)
	// Head is the commit dir has checked out (#310).
	Head(dir string) (string, error)
	// Attr reports which of paths git resolves attr to set on, in one call
	// (#338). Used for .gitattributes' linguist-generated.
	Attr(dir, attr string, paths []string) (map[string]bool, error)
	// RestoreTree writes tree over dir's working tree, removing what the tree
	// does not hold (#334). SnapshotTree read backwards.
	RestoreTree(dir, tree string) error
}

// CLI runs the real git binary.
//
//	branch, err := vcs.NewCLI().CurrentBranch("/p/omatty")
type CLI struct {
	bin string
	// limit, when set, replaces every per-command deadline. Only tests set it,
	// so a git that hangs can be cut off in milliseconds rather than 30 s.
	limit time.Duration
	// parent is the caller's context, set by Contextual's methods (ADR 0001's
	// ports, migration step 5.4, #653). Nil is the background: a call made
	// through a plain method is bounded by its deadline alone.
	parent context.Context
}

// Every git call has a deadline (#650). Several run on the TUI's event loop -
// git worktree add on ctrl+o N, rev-parse per root on discovery - so a git
// that hangs on a network filesystem, a lock or a credential prompt froze
// omatty outright. These are the ceilings: generous for any healthy git, and
// finite. ADR 0001's migration moves them to the service ports (5.4, 5.8).
const (
	gitDeadline         = 30 * time.Second
	worktreeAddDeadline = 60 * time.Second // checks a whole tree out
)

// waitDelay bounds how long a killed git's output is still waited for. A git
// that hung may have children - a credential helper, an ssh - still holding
// its stdout, and without this the wait for them would outlast the deadline.
const waitDelay = 2 * time.Second

// commandFor is git in dir under its deadline. The caller defers cancel.
func (c *CLI) commandFor(dir string, args []string) (*exec.Cmd, context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(c.parentOrBackground(), c.deadlineFor(args))
	cmd := exec.CommandContext(ctx, c.bin, args...)
	cmd.Dir = dir
	cmd.WaitDelay = waitDelay
	return cmd, ctx, cancel
}

// failure is a failed invocation as a CommandError, saying so when the
// deadline is what ended it: "signal: killed" alone would read as a crash.
func (c *CLI) failure(ctx context.Context, dir string, args []string, stderr *bytes.Buffer, err error) error {
	switch {
	case c.parent != nil && c.parent.Err() != nil:
		err = fmt.Errorf("git was stopped by its caller: %w", c.parent.Err())
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		err = fmt.Errorf("git did not finish within %v: %w", c.deadlineFor(args), ctx.Err())
	}
	return &CommandError{Args: args, Dir: dir, Stderr: strings.TrimSpace(stderr.String()), Err: err}
}

// deadlineFor is how long one invocation may take.
func (c *CLI) deadlineFor(args []string) time.Duration {
	if c.limit > 0 {
		return c.limit
	}
	if len(args) >= 2 && args[0] == "worktree" && args[1] == "add" {
		return worktreeAddDeadline
	}
	return gitDeadline
}

// NewCLI returns a CLI that invokes "git" from PATH.
func NewCLI() *CLI { return &CLI{bin: "git"} }

// capture executes git in dir and returns stdout untouched: diff output needs
// its final newline. okExit is one extra exit status treated as success,
// because `git diff --no-index` exits 1 to mean "differences found", which is
// the answer rather than a failure (#21).
func (c *CLI) capture(dir string, okExit int, args ...string) (string, error) {
	return c.captureEnv(dir, okExit, nil, args...)
}

// captureEnv is capture with extra environment entries, for SnapshotTree,
// which points git at a temporary index (#311).
func (c *CLI) captureEnv(dir string, okExit int, env []string, args ...string) (string, error) {
	if err := checkDir(dir); err != nil {
		return "", err
	}
	cmd, ctx, cancel := c.commandFor(dir, args)
	defer cancel()
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && !exitedWith(err, okExit) {
		return "", c.failure(ctx, dir, args, &stderr, err)
	}
	return string(out), nil
}

// exitedWith reports whether err is git exiting with exactly code. A zero
// code never matches: success is not an error in the first place.
func exitedWith(err error, code int) bool {
	var exit *exec.ExitError
	return code != 0 && errors.As(err, &exit) && exit.ExitCode() == code
}

// run executes git in dir and returns trimmed stdout. Failures carry git's
// own stderr, which is the only useful diagnostic a caller can act on.
func (c *CLI) run(dir string, args ...string) (string, error) {
	out, err := c.capture(dir, 0, args...)
	return strings.TrimSpace(out), err
}

// checkDir rejects a bad path before exec. exec.Cmd only fails when it tries
// to chdir, so without this the caller is told "fork/exec /opt/homebrew/bin/
// git: not a directory" - which blames the git binary for the caller's path
// (issue #29).
func checkDir(dir string) error {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("vcs: %q does not exist", dir)
	}
	if err != nil {
		return fmt.Errorf("vcs: cannot read %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("vcs: %q is not a directory", dir)
	}
	return nil
}

// RepoRoot returns the top level of the working tree containing dir.
func (c *CLI) RepoRoot(dir string) (string, error) {
	return c.run(dir, "rev-parse", "--show-toplevel")
}

// MainCheckout returns the top level of the repository dir belongs to,
// resolving a linked worktree to the repository it was forked from.
//
// RepoRoot cannot do this: `rev-parse --show-toplevel` inside a linked
// worktree returns the worktree itself, so discovery would register every
// worktree as a project of its own (#91).
//
// The first record of `git worktree list --porcelain` is always the main
// worktree, whatever directory the command runs from. The parent of
// `--git-common-dir` is not: for a submodule it is `<super>/.git/modules`, and
// for a repository cloned with `--separate-git-dir` it is wherever the git
// directory was put - so discovery proposed unregistrable paths, and when the
// git directory sat under $HOME and $HOME was itself a repository, AddProject
// walked up and registered the user's whole home directory (#91).
//
//	root, err := git.MainCheckout("/w/repo/.omatty/wt/repo/fix") // "/w/repo"
func (c *CLI) MainCheckout(dir string) (string, error) {
	out, err := c.run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	root, bare := firstWorktree(out)
	switch {
	case root == "":
		return "", fmt.Errorf("vcs: %q: git worktree list named no worktree in output:\n%s", dir, out)
	case bare:
		return "", fmt.Errorf(
			"vcs: %q belongs to the bare repository %q, which has no checkout to register", dir, root)
	}
	return root, nil
}

// firstWorktree reads the main worktree out of `git worktree list --porcelain`,
// and whether it is bare. Records are separated by a blank line and the first
// is the main one, so the scan stops at the second "worktree" line.
func firstWorktree(out string) (root string, bare bool) {
	for _, line := range strings.Split(out, "\n") {
		path, isWorktree := strings.CutPrefix(line, "worktree ")
		if isWorktree {
			if root != "" {
				return root, bare // the second record begins; the first is done
			}
			root = strings.TrimSpace(path)
		}
		if strings.TrimSpace(line) == "bare" {
			bare = true
		}
	}
	return root, bare
}

// CurrentBranch returns the branch checked out in dir.
func (c *CLI) CurrentBranch(dir string) (string, error) {
	return c.run(dir, "rev-parse", "--abbrev-ref", "HEAD")
}

// AddWorktree creates a linked worktree at dir on a new branch forked from
// base, named explicitly so the recorded base and the fork point agree (#21).
func (c *CLI) AddWorktree(repoRoot, dir, branch, base string) error {
	_, err := c.run(repoRoot, "worktree", "add", "-b", branch, dir, base)
	return err
}

// RenameBranch renames a branch. git updates the HEAD of a linked worktree
// that has it checked out, so the worktree follows the name without moving:
// its directory is where claude is running and where the transcript path is
// derived from, and moving it would blind the tailer (#60, #151).
//
// -m rather than -M, so a name already taken is refused instead of clobbered.
func (c *CLI) RenameBranch(repoRoot, old, name string) error {
	_, err := c.run(repoRoot, "branch", "-m", old, name)
	return err
}

// CommitsOnBranch counts the commits branch has and base does not. Zero means
// nobody has committed to it yet, which is the only state #151 renames a
// placeholder in: after the first commit the name is in a history someone may
// already have pushed.
func (c *CLI) CommitsOnBranch(repoRoot, base, branch string) (int, error) {
	out, err := c.run(repoRoot, "rev-list", "--count", base+".."+branch)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("vcs: counting commits on %q since %q: %q is not a count: %w",
			branch, base, out, err)
	}
	return n, nil
}

// RemoveWorktree deletes a linked worktree, discarding uncommitted changes.
func (c *CLI) RemoveWorktree(repoRoot, dir string) error {
	_, err := c.run(repoRoot, "worktree", "remove", "--force", dir)
	return err
}

// diffArgs keeps diff output machine-readable: no colour, no external diff
// tool, no path quoting, renames detected so a moved file is one entry.
func diffArgs(extra ...string) []string {
	return append([]string{"-c", "core.quotepath=false", "diff",
		"--no-color", "--no-ext-diff", "-M"}, extra...)
}

// MergeBase returns the commit where ref and HEAD diverged.
//
//	base, err := vcs.NewCLI().MergeBase("/wt/parser-fix", "develop")
func (c *CLI) MergeBase(dir, ref string) (string, error) {
	return c.run(dir, "merge-base", ref, "HEAD")
}

// CommitExists reports whether ref names a commit in dir's repository. A ref
// that does not is false, not an error: rev-parse --verify --quiet says so
// with exit 1 and no output. Review asks after merge-base has failed on a
// session's recorded base, since a merged branch is normally deleted (#684).
//
//	ok, err := vcs.NewCLI().CommitExists("/wt/parser-fix", "develop")
func (c *CLI) CommitExists(dir, ref string) (bool, error) {
	out, err := c.capture(dir, 1, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
	return err == nil && strings.TrimSpace(out) != "", err
}

// Diff returns the working tree's unified diff against commit, which is
// everything a session changed whether it committed it or not (#21).
//
//	raw, err := vcs.NewCLI().Diff("/wt/parser-fix", base)
func (c *CLI) Diff(dir, commit string) (string, error) {
	return c.capture(dir, 0, diffArgs(commit, "--")...)
}

// Shortstat is git's one-line summary of a diff.
// Shortstat is review.Shortstat, the numbers a sidebar card shows (#180). It
// moved to internal/domain/review in migration step 5.8 (#653).
type Shortstat = review.Shortstat

// Shortstat is the numbers a session's sidebar card shows (#180), measured
// the way Diff measures: the working tree against commit, renames detected.
// git prints nothing for a clean tree, so the zero value is not an error.
//
//	st, err := vcs.NewCLI().Shortstat("/wt/parser-fix", base)
func (c *CLI) Shortstat(dir, commit string) (Shortstat, error) {
	out, err := c.run(dir, diffArgs("--shortstat", commit, "--")...)
	if err != nil {
		return Shortstat{}, err
	}
	return parseShortstat(out)
}

// parseShortstat reads " 3 files changed, 12 insertions(+), 4 deletions(-)".
// Every clause is optional - a pure deletion has no insertions clause and a
// binary change has neither - and each names itself, so the order is not
// trusted either.
func parseShortstat(line string) (Shortstat, error) {
	var st Shortstat
	for _, clause := range strings.Split(strings.TrimSpace(line), ",") {
		fields := strings.Fields(clause)
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.Atoi(fields[0])
		if err != nil {
			return Shortstat{}, fmt.Errorf("vcs: shortstat clause %q: want a count first: %w", clause, err)
		}
		st = withClause(st, fields[1], n)
	}
	return st, nil
}

// withClause sets the counter a shortstat clause names: file(s), insertion(s)
// or deletion(s). A function rather than a method since Shortstat became an
// alias of a domain type (#653).
func withClause(st Shortstat, noun string, n int) Shortstat {
	switch {
	case strings.HasPrefix(noun, "file"):
		st.Files = n
	case strings.HasPrefix(noun, "insertion"):
		st.Added = n
	case strings.HasPrefix(noun, "deletion"):
		st.Removed = n
	}
	return st
}

// Untracked lists untracked, non-ignored files relative to dir.
//
//	files, err := vcs.NewCLI().Untracked("/wt/parser-fix")
func (c *CLI) Untracked(dir string) ([]string, error) {
	out, err := c.run(dir, "ls-files", "--others", "--exclude-standard")
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Split(out, "\n"), nil
}

// UntrackedDiff diffs path against /dev/null so a new file reads as pure
// additions; git exits 1 for "differences", which capture tolerates.
//
//	raw, err := vcs.NewCLI().UntrackedDiff("/wt/parser-fix", "new.txt")
func (c *CLI) UntrackedDiff(dir, path string) (string, error) {
	return c.capture(dir, 1, diffArgs("--no-index", "--", os.DevNull, path)...)
}

// ListFiles returns tracked plus untracked, non-ignored paths, sorted. git
// emits the two sets one after the other, so the sort is what makes the tree
// read as a directory listing rather than as two interleaved lists (#24).
//
//	files, err := vcs.NewCLI().ListFiles(sess.Dir)
func (c *CLI) ListFiles(dir string) ([]string, error) {
	out, err := c.run(dir, "ls-files", "--cached", "--others", "--exclude-standard")
	if err != nil || out == "" {
		return nil, err
	}
	files := strings.Split(out, "\n")
	sort.Strings(files)
	return files, nil
}
