package vcs_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// Each clause is optional and singular or plural; a clean tree prints nothing.
func TestParseShortstat_Table_issue180(t *testing.T) {
	for _, tt := range []struct {
		line string
		want vcs.Shortstat
	}{
		{"", vcs.Shortstat{}},
		{" 1 file changed", vcs.Shortstat{Files: 1}},
		{" 3 files changed, 12 insertions(+), 4 deletions(-)", vcs.Shortstat{Files: 3, Added: 12, Removed: 4}},
		{" 1 file changed, 2 deletions(-)", vcs.Shortstat{Files: 1, Removed: 2}},
		{" 1 file changed, 1 insertion(+)", vcs.Shortstat{Files: 1, Added: 1}},
		{" 2 files changed, 1 insertion(+), 1 deletion(-)\n", vcs.Shortstat{Files: 2, Added: 1, Removed: 1}},
	} {
		got, err := vcs.ParseShortstat(tt.line)
		if err != nil || got != tt.want {
			t.Errorf("ParseShortstat(%q) = %+v, %v; want %+v", tt.line, got, err, tt.want)
		}
	}
}

func TestParseShortstat_RefusesANonNumericCount_issue180(t *testing.T) {
	_, err := vcs.ParseShortstat(" many files changed")
	if err == nil || !strings.Contains(err.Error(), "many") {
		t.Errorf("error = %v, want one quoting the bad clause", err)
	}
}

// The working tree against the merge-base, like Diff (#21): a commit on the
// branch and an edit on top of it count as one change.
func TestCLI_ShortstatCountsCommittedAndUncommittedTogether_issue180(t *testing.T) {
	repo := newRepo(t)
	writeFile(t, repo, "a.txt", "one\n")
	gitOut(t, repo, "add", "a.txt")
	gitOut(t, repo, "commit", "-m", "a")
	gitOut(t, repo, "checkout", "-b", "feat")
	writeFile(t, repo, "a.txt", "two\nthree\n")
	gitOut(t, repo, "commit", "-am", "two")
	writeFile(t, repo, "a.txt", "two\nthree\nfour\n")
	g := vcs.NewCLI()
	base, err := g.MergeBase(repo, "main")
	if err != nil {
		t.Fatal(err)
	}

	got, err := g.Shortstat(repo, base)

	if err != nil {
		t.Fatalf("Shortstat() error = %v", err)
	}
	if want := (vcs.Shortstat{Files: 1, Added: 3, Removed: 1}); got != want {
		t.Errorf("Shortstat() = %+v, want %+v", got, want)
	}
}

func TestCLI_ShortstatOfACleanTreeIsZero_issue180(t *testing.T) {
	got, err := vcs.NewCLI().Shortstat(newRepo(t), "HEAD")
	if err != nil || got != (vcs.Shortstat{}) {
		t.Errorf("Shortstat() = %+v, %v; want the zero value and no error", got, err)
	}
}
