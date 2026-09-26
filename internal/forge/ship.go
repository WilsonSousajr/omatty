// Acting on a pull request (#331): push-and-open, and merge one that is already
// green. This is the only part of this package that writes.
//
// It is bounded to what a person asks for while reading. The decision, argued in
// `docs/ROADMAP.md`'s shipping section: the criterion is not whether the forge is
// touched, it is whether omatty acts while nobody is reading. One keypress, one
// session, every time - never merging when checks *go* green, never a
// force-push, never a branch deletion, and never into a protected branch.

package forge

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// prNumberInURL reads the number off the url `gh pr create` prints, which is the
// only place it reports one.
var prNumberInURL = regexp.MustCompile(`/pull/(\d+)`)

// CreatePR opens a pull request for head against base and returns its number.
//
//	n, err := forge.NewCLI().CreatePR(root, sess.Branch, sess.Base, title)
//
// `--fill` takes the body from the branch's commits, which is the operator's own
// writing; omatty composes nothing on their behalf. The title is passed because
// a session has one and it is better than the last commit subject.
func (c *CLI) CreatePR(repoRoot, head, base, title string) (int, error) {
	ctx, cancel, err := c.bounded()
	if err != nil {
		return 0, err
	}
	defer cancel()
	out, err := c.run(ctx, repoRoot, "pr", "create",
		"--head", head, "--base", base, "--title", title, "--fill")
	if err != nil {
		return 0, err
	}
	m := prNumberInURL.FindSubmatch(out)
	if m == nil {
		return 0, fmt.Errorf("forge: gh pr create printed no pull request url: %s", out)
	}
	n, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return 0, fmt.Errorf("forge: reading the pull request number from %q: %w", out, err)
	}
	return n, nil
}

// MergePR merges the pull request, with the repository's own merge method.
//
//	err := forge.NewCLI().MergePR(root, 443)
//
// No method flag, and no `--delete-branch` or `--admin`. The method a repository
// allows is its own setting; a repository that allows several makes gh say so,
// which is the right answer to give the operator rather than picking one for
// them. Deleting the branch and overriding a failing check are both refused by
// #331 outright.
func (c *CLI) MergePR(repoRoot string, number int) error {
	ctx, cancel, err := c.bounded()
	if err != nil {
		return err
	}
	defer cancel()
	_, err = c.run(ctx, repoRoot, "pr", "merge", strconv.Itoa(number))
	return err
}

// BranchProtected reports whether branch is protected on the forge.
//
//	protected, err := forge.NewCLI().BranchProtected(root, sess.Base)
//
// The branch object's own `protected` flag, which needs no admin scope - unlike
// the protection *settings*, which do. This is the bound #331's own text does
// not carry and the roadmap added: AGENTS.md moves `main` only by a promotion
// pull request, "and this applies to the repository owner too", so a ship key
// able to merge there would route around omatty's own release gate.
//
// **It fails closed.** On any error it returns true beside the error, so a caller
// that reads the bool without the error still refuses. The one case this check
// exists for is the one where being wrong cannot be undone.
func (c *CLI) BranchProtected(repoRoot, branch string) (bool, error) {
	ctx, cancel, err := c.bounded()
	if err != nil {
		return true, err
	}
	defer cancel()
	out, err := c.run(ctx, repoRoot, "api",
		"repos/{owner}/{repo}/branches/"+branch, "--jq", ".protected")
	if err != nil {
		return true, err
	}
	protected, convErr := strconv.ParseBool(strings.TrimSpace(string(out)))
	if convErr != nil {
		return true, fmt.Errorf("forge: reading whether %q is protected, gh said %q", branch, out)
	}
	return protected, nil
}
