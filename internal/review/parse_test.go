package review_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

func parse(t *testing.T, raw string) review.Diff {
	t.Helper()
	d, err := review.ParseDiff(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseDiff() error = %v", err)
	}
	return d
}

func TestParseDiff_FilesHunksAndNumberedLines_issue21(t *testing.T) {
	d := parse(t, twoFileDiff)

	if len(d.Files) != 2 {
		t.Fatalf("parsed %d files, want 2", len(d.Files))
	}
	f := d.Files[0]
	if f.Path != "internal/ui/model.go" || f.Status != review.FileModified {
		t.Errorf("file 0 = %q %v, want internal/ui/model.go FileModified", f.Path, f.Status)
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("hunks = %+v, want one", f.Hunks)
	}
	want := []review.Line{
		{Kind: review.LineContext, Text: "\ta := 1", OldNo: 10, NewNo: 10},
		{Kind: review.LineRemoved, Text: "\tb := 2", OldNo: 11},
		{Kind: review.LineAdded, Text: "\tb := 3", NewNo: 11},
		{Kind: review.LineAdded, Text: "\tc := 4", NewNo: 12},
		{Kind: review.LineContext, Text: "\treturn", OldNo: 12, NewNo: 13},
		{Kind: review.LineContext, Text: "}", OldNo: 13, NewNo: 14},
	}
	for i, l := range want {
		if got := f.Hunks[0].Lines[i]; got != l {
			t.Errorf("line %d = %+v, want %+v", i, got, l)
		}
	}
	if n := d.Files[1]; n.Path != "new.txt" || n.Status != review.FileAdded {
		t.Errorf("file 1 = %q %v, want new.txt FileAdded", n.Path, n.Status)
	}
}

// The header is the middle element of every anchor (invariant 7), so it must
// be git's own line and stable across parses.
func TestParseDiff_HunkHeaderIsGitsOwnLine_issue22(t *testing.T) {
	d := parse(t, twoFileDiff)

	got := d.Files[0].Hunks[0].Header

	if !strings.HasPrefix(got, "@@ -10,4 +10,5 @@") {
		t.Errorf("hunk header = %q, want it to start with git's @@ -10,4 +10,5 @@", got)
	}
}

func TestParseDiff_EmptyInputIsAnEmptyDiff(t *testing.T) {
	d := parse(t, "")
	if len(d.Files) != 0 {
		t.Errorf("empty input parsed %d files, want 0", len(d.Files))
	}
}

// A hunk that promises more lines than it carries is what a truncated or
// interrupted `git diff` looks like; the error must say what omatty was
// doing, not only what go-gitdiff saw.
func TestParseDiff_TruncatedHunkNamesTheProblem(t *testing.T) {
	truncated := "diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,5 +1,5 @@\n context\n"

	_, err := review.ParseDiff(strings.NewReader(truncated))

	if err == nil {
		t.Fatal("ParseDiff(truncated hunk) returned nil, want an error")
	}
	if !strings.Contains(err.Error(), "parsing unified diff") {
		t.Errorf("error %q does not say omatty was parsing a unified diff", err)
	}
	if !strings.Contains(err.Error(), "miscounts") {
		t.Errorf("error %q drops git's own diagnostic", err)
	}
}

func TestFile_CountsAddedAndRemoved(t *testing.T) {
	d := parse(t, twoFileDiff)
	if a, r := d.Files[0].Counts(); a != 2 || r != 1 {
		t.Errorf("Counts() = +%d -%d, want +2 -1", a, r)
	}
}

func TestDiff_LineAt(t *testing.T) {
	d := parse(t, twoFileDiff)
	if got := d.LineAt(review.Position{File: 1, Hunk: 0, Line: 1}); got.Text != "file" {
		t.Errorf("LineAt = %+v, want the second added line of new.txt", got)
	}
}

// A rename keeps both names so the header can show "old → new"; a delete is
// named by the path that existed, because the new name is /dev/null.
func TestParseDiff_RenameDeleteAndBinary_issue21(t *testing.T) {
	raw := `diff --git a/old.go b/new.go
similarity index 100%
rename from old.go
rename to new.go
diff --git a/gone.txt b/gone.txt
deleted file mode 100644
index 1111111..0000000
--- a/gone.txt
+++ /dev/null
@@ -1,2 +0,0 @@
-one
-two
diff --git a/logo.png b/logo.png
index 1111111..2222222 100644
Binary files a/logo.png and b/logo.png differ
`

	d := parse(t, raw)

	if len(d.Files) != 3 {
		t.Fatalf("parsed %d files, want 3", len(d.Files))
	}
	if f := d.Files[0]; f.Status != review.FileRenamed || f.OldPath != "old.go" || f.Path != "new.go" {
		t.Errorf("file 0 = %+v, want a rename from old.go to new.go", f)
	}
	if f := d.Files[1]; f.Status != review.FileDeleted || f.Path != "gone.txt" {
		t.Errorf("file 1 = %q %v, want gone.txt FileDeleted", f.Path, f.Status)
	}
	if f := d.Files[2]; !f.Binary || len(f.Hunks) != 0 {
		t.Errorf("file 2 = %+v, want a binary with no hunks", f)
	}
}

func TestFile_CountsOfAFileWithNoHunksIsZero(t *testing.T) {
	a, r := review.File{Path: "logo.png", Binary: true}.Counts()
	if a != 0 || r != 0 {
		t.Errorf("Counts() = +%d -%d, want +0 -0", a, r)
	}
}
