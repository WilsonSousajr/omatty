package forge

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrNoGH is ListPRs' answer when gh is not installed: nothing to ask, and
// not a failure to report again on every poll.
var ErrNoGH = errors.New("forge: gh is not on PATH")

// ErrNotGitHub is ListPRs' answer for a checkout gh cannot map to a GitHub
// repository - no remote, another host, or not a repository at all.
var ErrNotGitHub = errors.New("forge: not a GitHub repository")

// notGitHub is gh's stderr for each way a checkout has no GitHub repository,
// read off gh 2.87.3.
var notGitHub = []string{
	"no git remotes found",
	"none of the git remotes configured for this repository point to a known GitHub host",
	"not a git repository",
}

// prFields is exactly what Fold reads.
const prFields = "number,headRefName,headRefOid,isCrossRepository,state,mergeStateStatus,statusCheckRollup"

// CLI runs the gh binary.
//
//	prs, err := forge.NewCLI().ListPRs("/p/omatty")
type CLI struct{ bin string }

// NewCLI returns a CLI that invokes "gh" from PATH.
func NewCLI() *CLI { return &CLI{bin: "gh"} }

// ListPRs is the repository's most recent hundred pull requests, of every
// state, in one call: a card with ten sessions in one repository costs one
// request, never one per pull request, which is what tripped GitHub's
// secondary rate limit before. gh resolves the repository from repoRoot's
// remote and uses the operator's own authentication; omatty holds nothing.
func (c *CLI) ListPRs(repoRoot string) ([]PR, error) {
	if _, err := exec.LookPath(c.bin); err != nil {
		return nil, ErrNoGH
	}
	cmd := exec.Command(c.bin, "pr", "list", "--state", "all", "--limit", "100", "--json", prFields)
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, classify(repoRoot, strings.TrimSpace(stderr.String()), err)
	}
	return Fold(out)
}

// classify names a checkout that is not on GitHub, and carries gh's own
// words for anything else.
func classify(repoRoot, stderr string, err error) error {
	for _, s := range notGitHub {
		if strings.Contains(stderr, s) {
			return fmt.Errorf("forge: %s: %w", stderr, ErrNotGitHub)
		}
	}
	return fmt.Errorf("forge: gh pr list in %q: %s: %w", repoRoot, stderr, err)
}
