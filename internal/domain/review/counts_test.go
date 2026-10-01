package review_test

import "testing"

// Counts moved here with File (migration step 3.6a, #635); it was tested only
// through internal/review's parser, so it is pinned in its own package: the
// file header's +N -M.
func TestFile_CountsAddedAndRemovedLines_issue635(t *testing.T) {
	d := parse(t, twoFileDiff)
	cases := []struct {
		file           int
		added, removed int
	}{{0, 2, 1}, {1, 2, 0}}
	for _, c := range cases {
		added, removed := d.Files[c.file].Counts()
		if added != c.added || removed != c.removed {
			t.Errorf("%s: Counts() = +%d -%d, want +%d -%d", d.Files[c.file].Path, added, removed, c.added, c.removed)
		}
	}
}
