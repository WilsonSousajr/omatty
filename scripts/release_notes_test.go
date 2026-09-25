// These tests guard scripts/release-notes.sh (#327): a release's notes are
// its own CHANGELOG section, so the changelog stays the one place a release
// is described (#134), and a tag with no section must not publish at all.
package scripts_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func releaseNotes(t *testing.T, version string) (string, error) {
	t.Helper()
	root := repoRoot(t)
	cmd := exec.Command(filepath.Join(root, "scripts", "release-notes.sh"), version)
	cmd.Dir = root
	out, err := cmd.Output()
	return string(out), err
}

// v0.2.0's section is everything between its heading and v0.1.0's: its
// opening line, its Added list, and its last known limitation - never the
// Unreleased section above it or v0.1.0 below.
func TestReleaseNotes_PrintsExactlyThatVersionsSection_issue327(t *testing.T) {
	notes, err := releaseNotes(t, "v0.2.0")
	if err != nil {
		t.Fatalf("release-notes.sh v0.2.0: %v", err)
	}
	for _, want := range []string{"M9, M10, M11 and M13", "### Added", "(#315)"} {
		if !strings.Contains(notes, want) {
			t.Errorf("the v0.2.0 notes lack %q:\n%s", want, notes)
		}
	}
	for _, unwanted := range []string{"## [", "Unreleased", "First release. Eight milestones"} {
		if strings.Contains(notes, unwanted) {
			t.Errorf("the v0.2.0 notes contain %q, which belongs to another section:\n%s", unwanted, notes)
		}
	}
}

// The last section runs to the link references at the bottom, not into them.
func TestReleaseNotes_TheOldestSectionStopsBeforeTheLinks_issue327(t *testing.T) {
	notes, err := releaseNotes(t, "v0.1.0")
	if err != nil {
		t.Fatalf("release-notes.sh v0.1.0: %v", err)
	}
	if !strings.Contains(notes, "First release. Eight milestones") {
		t.Errorf("the v0.1.0 notes lack its opening line:\n%s", notes)
	}
	if strings.Contains(notes, "releases/tag/") {
		t.Errorf("the v0.1.0 notes run into the link references:\n%s", notes)
	}
}

// A tag the changelog does not describe must stop the release: publishing it
// with empty notes would be a release nobody wrote down.
func TestReleaseNotes_AnUndescribedVersionFails_issue327(t *testing.T) {
	notes, err := releaseNotes(t, "v9.9.9")
	if err == nil {
		t.Errorf("release-notes.sh v9.9.9 succeeded with %q; want a failure", notes)
	}
	if strings.TrimSpace(notes) != "" {
		t.Errorf("printed %q for an undescribed version; want nothing on stdout", notes)
	}
}
