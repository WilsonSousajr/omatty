package forge

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// notGitHub is gh's stderr for each way a checkout has no GitHub repository,
// read off gh 2.87.3. Each is ErrNoForge: gh was ErrNotGitHub's only source
// until #449 named the answer for every forge.
var notGitHub = []string{
	"no git remotes found",
	"none of the git remotes configured for this repository point to a known GitHub host",
	"not a git repository",
}

// openFields is everything Fold reads: an open pull request's card shows its
// CI and whether it can merge, and the tracker's row shows its title, whether
// it is a draft, and its age (#393). All three are cheap fields on a call
// already being made; statusCheckRollup stays the only expensive one.
// reviewDecision joined for #432's review glyph, the same cheap-field argument.
const openFields = "number,title,headRefName,headRefOid,isCrossRepository,state,isDraft,updatedAt,mergeStateStatus,statusCheckRollup,reviewDecision"

// finishedFields leaves the checks out: a merged or closed card shows no CI
// mark, and the rollup is the expensive part of the answer (#358). It leaves
// the tracker's three out too, because the tracker lists only what is open.
//
// mergedAt is here for #332's lead time, and it is the one field this set needs
// that the cards never did. Cheap, on a call already being made - the same trade
// openFields' own comment describes, where statusCheckRollup stays the only
// expensive field.
const finishedFields = "number,headRefName,headRefOid,isCrossRepository,state,mergedAt"

// finishedWindow is how many recently finished pull requests are read. A
// finished one only matters when it is the work at a worktree's HEAD, which
// is recent by nature; an open one matters however old it is, so the open
// set is asked for whole rather than through this window (#358).
const finishedWindow = "30"

// ghCLI is the GitHub backend: the operator's own gh, run in the project's
// root, which resolves the repository from the remote and authenticates as the
// operator. omatty holds nothing. The Router finds gh on PATH before it builds
// one, and gives each call its context.
type ghCLI struct {
	bin     string
	timeout time.Duration
}

// listTimeout bounds one poll of gh. A stalled gh - a laptop waking, a
// network that changed under it - otherwise holds the project's poll until
// TCP gives up, minutes later, with the card still showing the last verdict
// (#356).
const listTimeout = 30 * time.Second

// ListPRs is the repository's open pull requests, then its most recently
// finished ones: two calls for the whole project, never one per pull request,
// which is what tripped GitHub's secondary rate limit before. One `--state
// all` call ordered by creation dropped an open pull request older than the
// newest hundred off its card (#358). gh resolves the repository from
// repoRoot's remote and uses the operator's own authentication; omatty holds
// nothing. Both calls share ctx: the promise is an answer inside thirty
// seconds, not thirty seconds for each half of it (#356).
func (c ghCLI) listPRs(ctx context.Context, repoRoot string) ([]PR, error) {
	open, err := c.list(ctx, repoRoot, "open", "100", openFields)
	if err != nil {
		return nil, err
	}
	finished, err := c.list(ctx, repoRoot, "closed", finishedWindow, finishedFields)
	if err != nil {
		return nil, err
	}
	return append(open, finished...), nil
}

// ListIssues is the repository's open issues: one call for the whole project,
// #358's rule applied to the other list. Closed issues are not read at all -
// the tracker answers "what is open", and a closed one is history the forge
// already keeps.
func (c ghCLI) listIssues(ctx context.Context, repoRoot string) ([]Issue, error) {
	out, err := c.run(ctx, repoRoot, "issue", "list", "--state", "open", "--limit", issueWindow, "--json", issueFields)
	if err != nil {
		return nil, err
	}
	return FoldIssues(out)
}

// list is one `gh pr list` in repoRoot. gh's "closed" includes merged.
func (c ghCLI) list(ctx context.Context, repoRoot, state, limit, fields string) ([]PR, error) {
	out, err := c.run(ctx, repoRoot, "pr", "list", "--state", state, "--limit", limit, "--json", fields)
	if err != nil {
		return nil, err
	}
	return Fold(out)
}

// run is one gh invocation in repoRoot, and the only place this package starts
// a process. Both lists share it so neither can drift from the other's
// bounding, classification or diagnostics.
func (c ghCLI) run(ctx context.Context, repoRoot string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.bin, args...)
	cmd.Dir = repoRoot
	// A grandchild holding stdout must not outlive the kill (#356), the same
	// reason supervisor's naming call sets it.
	cmd.WaitDelay = time.Second
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("forge: gh %s in %q gave no answer in %v: %w", called(args), repoRoot, c.timeout, ctx.Err())
	}
	if err != nil {
		return nil, classify(repoRoot, called(args), strings.TrimSpace(stderr.String()), err)
	}
	return out, nil
}

// called names the call for a message - "pr list", "issue list" - so a failure
// sends its reader to the right subcommand now that two lists share one runner
// (#393).
func called(args []string) string {
	if len(args) < 2 {
		return strings.Join(args, " ")
	}
	return args[0] + " " + args[1]
}

// classify names a checkout that is not on GitHub, and carries gh's own
// words for anything else.
func classify(repoRoot, call, stderr string, err error) error {
	for _, s := range notGitHub {
		if strings.Contains(stderr, s) {
			return fmt.Errorf("forge: %s: %w", stderr, ErrNoForge)
		}
	}
	return fmt.Errorf("forge: gh %s in %q: %s: %w", call, repoRoot, stderr, err)
}
