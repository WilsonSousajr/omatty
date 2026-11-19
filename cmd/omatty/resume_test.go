package main

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/config"
)

// claude 2.1.323 prints `$CLAUDE_CODE_RESUME_HINT_COMMAND --resume <id>` as
// its exit hint, and only when the value matches ^\w[\w-]*$ (read in the
// 2.1.327 binary). A path or a flag would be ignored and the hint would fall
// back to `claude --resume`, which resumes outside omatty (#815).
func TestClaudeProfile_namesOmattyInTheResumeHint_issue815(t *testing.T) {
	env := claudeProfile().Env
	if !slices.Contains(env, "CLAUDE_CODE_RESUME_HINT_COMMAND=omatty") {
		t.Fatalf("claude's Env = %q, want CLAUDE_CODE_RESUME_HINT_COMMAND=omatty", env)
	}
	if !regexp.MustCompile(`^\w[\w-]*$`).MatchString("omatty") {
		t.Errorf("the hint command is not a bare word claude accepts")
	}
}

// `omatty --resume <id>` is what that hint runs, so dispatch must know it,
// and the unknown-command error must name it.
func TestDispatch_KnowsResume_issue815(t *testing.T) {
	err := dispatch("bogus", nil, t.TempDir(), config.Config{}, storeIn(t))
	if err == nil || !strings.Contains(err.Error(), "--resume") {
		t.Errorf("the unknown-command error %v does not list --resume", err)
	}
	err = dispatch("--resume", []string{"nowhere"}, t.TempDir(), config.Config{}, storeIn(t))
	if err == nil || !strings.Contains(err.Error(), `"nowhere"`) {
		t.Errorf("dispatch(--resume nowhere) = %v, want the no-such-conversation error", err)
	}
}
