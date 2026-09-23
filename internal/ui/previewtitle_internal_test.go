package ui

import (
	"strings"
	"testing"
)

// A path degrades from the front: the directories above a file are context and
// the filename is what says which file is on screen, so a cut from the right -
// which is what fitLine does - loses the only part that identifies it (#287).
func TestPreviewTitle(t *testing.T) {
	const path = "internal/ui/reviewview.go"
	cases := []struct {
		name   string
		path   string
		budget int
		want   string
	}{
		{
			name:   "room for the whole path",
			path:   path,
			budget: 40,
			want:   "internal/ui/reviewview.go",
		},
		{
			name:   "exactly enough is not shortened",
			path:   path,
			budget: 25,
			want:   "internal/ui/reviewview.go",
		},
		{
			name:   "the top of the path goes first",
			path:   path,
			budget: 24,
			want:   "…/ui/reviewview.go",
		},
		{
			name:   "then the rest of it",
			path:   path,
			budget: 17,
			want:   "…/reviewview.go",
		},
		{
			name:   "a file at the repository root has nothing to give up",
			path:   "go.mod",
			budget: 4,
			want:   "g…od",
		},
		{
			name:   "no room even for the name: both its ends survive",
			path:   path,
			budget: 10,
			want:   "revi…ew.go",
		},
		{
			name:   "a deep path keeps its file",
			path:   "a/b/c/d/e/main.go",
			budget: 12,
			want:   "…/e/main.go",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := previewTitle(c.path, c.budget); got != c.want {
				t.Errorf("previewTitle(%q, %d) = %q, want %q", c.path, c.budget, got, c.want)
			}
		})
	}
}

// The marker is not decoration: `ui/reviewview.go` alone reads as a complete
// repo-relative path, which is a lie about where the file is - the same
// failure as a filtered tree that looks complete (#285).
func TestPreviewTitle_saysThatSomethingWasGivenUp(t *testing.T) {
	got := previewTitle("internal/ui/reviewview.go", 20)

	if !strings.HasPrefix(got, "…/") {
		t.Errorf("previewTitle() = %q, want it to say the path was shortened", got)
	}
}
