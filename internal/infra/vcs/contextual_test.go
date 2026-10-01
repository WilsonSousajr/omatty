package vcs_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/infra/vcs"
)

// ADR 0001's ports take a context (migration step 5.4, #653), and the caller's
// context is the parent of the git call: cancelling it ends a git that is
// still running, well inside the call's own deadline.
func TestContextual_aCancelledContextEndsTheGitCall_issue653(t *testing.T) {
	git := vcs.NewCLIWithDeadline(hungGit(t), time.Minute).Contextual()
	ctx, cancel := context.WithTimeout(t.Context(), 300*time.Millisecond)
	defer cancel()
	start := time.Now()

	_, err := git.CurrentBranch(ctx, t.TempDir())

	if err == nil {
		t.Fatal("a git whose context ended returned no error")
	}
	if elapsed := time.Since(start); elapsed > 10*time.Second {
		t.Errorf("the call returned after %v: the caller's context did not reach the git it ran", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want it to wrap the caller's context error", err)
	}
}

// Every method the session service's ports name is there, and each is the
// CLI's own call: a real repository answers through the adapter as it does
// without it.
func TestContextual_answersAsTheCLIDoes_issue653(t *testing.T) {
	repo := newRepo(t)
	cli := vcs.NewCLI()
	want, err := cli.CurrentBranch(repo)
	if err != nil {
		t.Fatal(err)
	}
	got, err := cli.Contextual().CurrentBranch(t.Context(), repo)
	if err != nil || got != want {
		t.Errorf("Contextual().CurrentBranch = %q, %v; want %q", got, err, want)
	}
	if root, err := cli.Contextual().RepoRoot(t.Context(), repo); err != nil || root == "" {
		t.Errorf("Contextual().RepoRoot = %q, %v; want the repository", root, err)
	}
}
