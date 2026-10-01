package forge_test

import (
	"errors"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/forge"
)

// GitHub's label is the copy omatty has always drawn, and the neutral one
// differs from it only in having no forge to name: a project whose forge is
// not known yet reads exactly as a GitHub one did.
// missingTool reports whether err says tool is not installed, however deeply
// it is wrapped.
func missingTool(err error, tool string) bool {
	var missing *forge.MissingToolError
	return errors.As(err, &missing) && missing.Tool == tool
}

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
