package forge_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/forge"
)

// missingTool reports whether err says tool is not installed, however deeply
// it is wrapped.
func missingTool(err error, tool string) bool {
	var missing *forge.MissingToolError
	return errors.As(err, &missing) && missing.Tool == tool
}

// The message names the tool and, once a forge has a REST fallback, the
// variable that would stand in for it: "install X or set Y" is the whole of the
// fix, and an operator reading the log should not have to guess either half.
func TestMissingToolError_NamesTheToolAndTheFix_issue449(t *testing.T) {
	for _, tt := range []struct {
		err  forge.MissingToolError
		want string
	}{
		{forge.MissingToolError{Tool: "gh"}, "forge: gh not found on PATH"},
		{forge.MissingToolError{Tool: "glab", TokenEnv: "GITLAB_TOKEN"}, "forge: glab not found on PATH and GITLAB_TOKEN is unset"},
		{forge.MissingToolError{TokenEnv: "BITBUCKET_TOKEN"}, "forge: BITBUCKET_TOKEN is unset"},
		{forge.MissingToolError{Tool: "tea", NoLoginFor: "codeberg.org"}, "forge: tea has no login for codeberg.org"},
		{forge.MissingToolError{Tool: "tea", TokenEnv: "GITEA_TOKEN", NoLoginFor: "codeberg.org"}, "forge: tea has no login for codeberg.org and GITEA_TOKEN is unset"},
	} {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("%+v.Error() = %q, want %q", tt.err, got, tt.want)
		}
	}
}

// A wrapped MissingToolError is still found, which is how the UI sorts it from
// an outage.
func TestMissingToolError_IsFoundThroughAWrap_issue449(t *testing.T) {
	err := fmt.Errorf("reading: %w", &forge.MissingToolError{Tool: "gh"})

	if !missingTool(err, "gh") {
		t.Errorf("errors.As(%v) did not find the missing gh", err)
	}
	if missingTool(errors.New("forge: gh pr list: HTTP 502"), "gh") {
		t.Error("an ordinary failure read as a missing tool")
	}
}

// GitHub's label is the copy omatty has always drawn, and the neutral one
// differs from it only in having no forge to name: a project whose forge is
// not known yet reads exactly as a GitHub one did.
func TestLabel_GitHubIsTodaysCopyAndNeutralNamesNoForge_issue449(t *testing.T) {
	want := forge.Label{Forge: "GitHub", Change: "pull request", Short: "PR", Sigil: "#"}
	if forge.GitHub != want {
		t.Errorf("GitHub = %+v, want %+v", forge.GitHub, want)
	}
	neutral := forge.GitHub
	neutral.Forge = ""
	if forge.Neutral != neutral {
		t.Errorf("Neutral = %+v, want %+v", forge.Neutral, neutral)
	}
}

// Ref is how a change is written: "#12" on GitHub, "!12" on GitLab.
func TestLabel_RefPutsTheSigilBeforeTheNumber_issue449(t *testing.T) {
	mr := forge.Label{Forge: "GitLab", Change: "merge request", Short: "MR", Sigil: "!"}

	if got := mr.Ref(12); got != "!12" {
		t.Errorf("Ref(12) = %q, want !12", got)
	}
	if got := forge.GitHub.Ref(349); got != "#349" {
		t.Errorf("Ref(349) = %q, want #349", got)
	}
}

// NoGH is a missing gh, for a caller that has nothing wired and must answer as
// a machine without it does - without naming the tool itself, which only this
// package may (TestNoGhOutsideForge).
func TestNoGH_IsAMissingGh_issue449(t *testing.T) {
	if err := forge.NoGH(); !missingTool(err, "gh") {
		t.Errorf("NoGH() = %v, want a missing gh", err)
	}
}
