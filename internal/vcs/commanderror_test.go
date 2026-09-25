package vcs_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// A failed git call is a *vcs.CommandError whose Detail is git's own stderr
// (#351). Wrapped under two layers of context, its innermost error is
// "exit status 128" - useless on screen - and the long message leads with a
// command line and a path, so a narrow column cut it before git's words.
// Error() is unchanged: every log line and every existing assertion reads it.
func TestCLI_aFailedCallCarriesGitsStderrAsItsDetail_issue351(t *testing.T) {
	repo := newRepo(t)

	err := vcs.NewCLI().AddWorktree(repo, filepath.Join(t.TempDir(), "dup"), "main", "main")

	var cmdErr *vcs.CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("error %v (%T) is not a *vcs.CommandError", err, err)
	}
	if d := cmdErr.Detail(); !strings.Contains(d, "main") || strings.Contains(d, "exit status") {
		t.Errorf("Detail() = %q, want git's stderr naming the branch", d)
	}
	if !strings.HasPrefix(err.Error(), "vcs: `git worktree add") || !strings.Contains(err.Error(), "exit status") {
		t.Errorf("Error() = %q, want today's text unchanged", err)
	}
}
