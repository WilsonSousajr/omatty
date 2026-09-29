package vcs_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// RemoteURL is origin's URL exactly as git holds it: forge parses it, so the
// reader must not reshape it on the way (#450).
func TestRemoteURL_IsOriginsURLAsGitHoldsIt_issue450(t *testing.T) {
	dir := newRepo(t)
	gitOut(t, dir, "remote", "add", "origin", "git@gitlab.com:group/sub/project.git")

	got, err := vcs.NewCLI().RemoteURL(dir)

	if err != nil || got != "git@gitlab.com:group/sub/project.git" {
		t.Errorf("RemoteURL = %q, %v; want origin's URL", got, err)
	}
}

// A checkout with no origin is an error that names the remote it looked for.
func TestRemoteURL_WithoutOriginSaysSo_issue450(t *testing.T) {
	dir := newRepo(t)

	_, err := vcs.NewCLI().RemoteURL(dir)

	if err == nil || !strings.Contains(err.Error(), "origin") {
		t.Errorf("RemoteURL error = %v, want one naming origin", err)
	}
}
