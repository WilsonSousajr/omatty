package registry_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

func TestPlaceholderTitle_IsTheFirstEightCharacters_issue127(t *testing.T) {
	if got := registry.PlaceholderTitle("abc12345-6789-0000"); got != "abc12345" {
		t.Errorf("PlaceholderTitle() = %q, want abc12345", got)
	}
	if got := registry.PlaceholderTitle("abc"); got != "abc" {
		t.Errorf("PlaceholderTitle() on a short id = %q, want abc", got)
	}
}

func TestSlug_ReducesUntrustedTextToABranchSafeName_issue127(t *testing.T) {
	for _, tt := range []struct{ in, want string }{
		{"Fix the horizontal wheel pan!", "fix-the-horizontal-wheel-pan"},
		{"../../etc/passwd", "etc-passwd"},
		{"--dangerously-skip", "dangerously-skip"},
		{"a---b", "a-b"},
		{"café \u202e naming", "caf-naming"},
		{"---", ""},
		{"", ""},
		{strings.Repeat("abcde-", 20), strings.Repeat("abcde-", 6) + "abcd"},
	} {
		if got := registry.Slug(tt.in); got != tt.want {
			t.Errorf("Slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// The strongest statement of "untrusted twice over": for any input, the
// output is empty or matches the branch-and-path-safe shape.
func FuzzSlug_NeverProducesAPathOrADotDot_issue127(f *testing.F) {
	f.Add("../x")
	f.Add("a b")
	f.Add("--x")
	f.Add("\u202e")
	re := regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)
	f.Fuzz(func(t *testing.T, s string) {
		got := registry.Slug(s)
		if got != "" && (!re.MatchString(got) || strings.HasSuffix(got, "-") || strings.Contains(got, "..")) {
			t.Errorf("Slug(%q) = %q", s, got)
		}
	})
}
