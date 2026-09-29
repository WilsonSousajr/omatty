package forge_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
)

// Label moved here from internal/infra/forge (migration step 3.5, #635); its
// users test it through the forge's folds, so Ref is pinned in its own
// package: a reference is the forge's sigil and the number, nothing else.
func TestLabel_RefIsTheSigilAndTheNumber_issue635(t *testing.T) {
	if got := forge.GitHub.Ref(402); got != "#402" {
		t.Errorf("GitHub.Ref(402) = %q, want #402", got)
	}
	gitlab := forge.Label{Forge: "GitLab", Change: "merge request", Short: "MR", Sigil: "!"}
	if got := gitlab.Ref(7); got != "!7" {
		t.Errorf("GitLab.Ref(7) = %q, want !7", got)
	}
}
