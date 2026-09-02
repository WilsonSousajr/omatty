package paths_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/paths"
)

func TestTranscriptSlug(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{
			name: "plain project path",
			dir:  "/Users/will/Documents/Projects/lazyvim-ai-harness-setup",
			want: "-Users-will-Documents-Projects-lazyvim-ai-harness-setup",
		},
		{
			name: "leading digit in a segment",
			dir:  "/Users/will/Documents/2nd-brain",
			want: "-Users-will-Documents-2nd-brain",
		},
		{
			name: "dotted segment collapses to a double dash",
			dir:  "/Users/will/Work/Guia/api-guiaflix/.worktrees/p2-questoes",
			want: "-Users-will-Work-Guia-api-guiaflix--worktrees-p2-questoes",
		},
		{
			// Regression, issue #60: claude replaces every character outside
			// [a-zA-Z0-9]; omatty mapped only '/' and '.', so this project's
			// transcript was never found and its status stayed blank forever.
			name: "space and underscore become dashes",
			dir:  "/Users/will/Documents/University/2026.1/LAB SD/my_proj",
			want: "-Users-will-Documents-University-2026-1-LAB-SD-my-proj",
		},
		{
			name: "unicode and punctuation become dashes",
			dir:  "/Users/will/Ação (v2)+notes",
			want: "-Users-will-A--o--v2--notes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := paths.TranscriptSlug(tt.dir); got != tt.want {
				t.Errorf("TranscriptSlug(%q) = %q, want %q", tt.dir, got, tt.want)
			}
		})
	}
}

func TestTranscript(t *testing.T) {
	got := paths.Transcript("/home/u", "/home/u/p", "abc-123")
	want := "/home/u/.claude/projects/-home-u-p/abc-123.jsonl"
	if got != want {
		t.Errorf("Transcript() = %q, want %q", got, want)
	}
}

func TestOmattyLocations(t *testing.T) {
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"root", paths.Root("/home/u"), "/home/u/.omatty"},
		{"state", paths.StateFile("/home/u"), "/home/u/.omatty/state.json"},
		{"hooks", paths.HooksFile("/home/u"), "/home/u/.omatty/hooks.json"},
		{"socket", paths.HookSocket("/home/u"), "/home/u/.omatty/sock"},
		{"logs", paths.LogDir("/home/u"), "/home/u/.omatty/logs"},
		{"worktree", paths.WorktreeDir("/home/u", "omatty", "fix"), "/home/u/.omatty/wt/omatty/fix"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}
}
