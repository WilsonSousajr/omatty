package forge_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
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

// A token refused over plain http and a token the forge refused each say what
// would fix them, and neither can carry the token itself (#584, #453).
func TestRefusals_NameTheHostAndTheCredential_issue653(t *testing.T) {
	plain := (&forge.PlainHTTPError{Host: "git.example", TokenEnv: "GITEA_TOKEN"}).Error()
	if plain != "forge: refusing to send GITEA_TOKEN's token to git.example over plain http; the remote must be https" {
		t.Errorf("PlainHTTPError = %q", plain)
	}
	auth := (&forge.AuthError{Host: "gitlab.example:8443", TokenEnv: "GITLAB_TOKEN", Status: 401}).Error()
	if auth != "forge: gitlab.example:8443 refused GITLAB_TOKEN (401)" {
		t.Errorf("AuthError = %q", auth)
	}
}

// Two names are one commit when equal, or when the shorter is a long enough
// prefix of the longer: Bitbucket names a head by twelve characters (#331).
func TestSameCommit_issue331(t *testing.T) {
	full := "31b8ff8dad0a4c6e8f1a2b3c4d5e6f708192a3b4"
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{full, full, true},
		{"31b8ff8dad0a", full, true},
		{full, "31b8ff8dad0a", true},
		{"31b8ff8", full, false},
		{"41b8ff8dad0a", full, false},
	} {
		if got := forge.SameCommit(c.a, c.b); got != c.want {
			t.Errorf("SameCommit(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
