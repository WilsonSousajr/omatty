package review_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// oneFile parses src and returns its only file.
func oneFile(t *testing.T, src string) review.File {
	t.Helper()
	d, err := review.ParseDiff(strings.NewReader(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Files) != 1 {
		t.Fatalf("parsed %d files, want 1", len(d.Files))
	}
	return d.Files[0]
}

const digestBefore = `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -10,3 +10,3 @@ func f() {
 	a := 1
-	b := 2
+	b := 3
 }
`

// The same change, rewritten: what the session did to the file is different,
// so the reviewer has not read this.
const digestAfter = `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -10,3 +10,3 @@ func f() {
 	a := 1
-	b := 2
+	b := 4
 }
`

// The same change, further down the file: an edit above it rewrote the hunk
// header, and nothing about this hunk's own lines moved.
const digestMoved = `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -40,3 +41,3 @@ func f() {
 	a := 1
-	b := 2
+	b := 3
 }
`

// #337 needs an identity for "the change I read", and invariant 7 already
// says what that identity is made of: content, never line numbers and never
// an mtime. FileDigest is the file-level version of LineHash.
func TestFileDigest_IsTheSameForTheSameChange_issue337(t *testing.T) {
	first, second := oneFile(t, digestBefore), oneFile(t, digestBefore)

	if review.FileDigest(first) != review.FileDigest(second) {
		t.Errorf("the same diff digested twice gave %q and %q",
			review.FileDigest(first), review.FileDigest(second))
	}
	if review.FileDigest(first) == "" {
		t.Error("FileDigest returned the empty string, which no file can be told apart by")
	}
}

func TestFileDigest_DiffersWhenTheChangeDoes_issue337(t *testing.T) {
	before, after := review.FileDigest(oneFile(t, digestBefore)), review.FileDigest(oneFile(t, digestAfter))

	if before == after {
		t.Errorf("b := 3 and b := 4 both digest to %q, so a file that changed "+
			"after it was marked reviewed would go on reading as reviewed", before)
	}
}

// A hunk header carries line numbers, so it moves whenever anything above it
// in the same file is edited - which is a change to this file, and the
// reviewer should be told. This is the deliberate difference from Anchor,
// where resolve() falls back past a moved header on purpose (#22).
func TestFileDigest_DiffersWhenTheHunkMovesInTheFile_issue337(t *testing.T) {
	if review.FileDigest(oneFile(t, digestBefore)) == review.FileDigest(oneFile(t, digestMoved)) {
		t.Error("the same hunk at a different place in the file digests the same; " +
			"an edit above a reviewed hunk would go unreported")
	}
}

// A file the session added and a file it modified are not the same review,
// even in the impossible case that their hunks match.
func TestFileDigest_DiffersByStatus_issue337(t *testing.T) {
	modified := oneFile(t, digestBefore)
	added := modified
	added.Status = review.FileAdded

	if review.FileDigest(modified) == review.FileDigest(added) {
		t.Error("a modified and an added file with the same hunks digest the same")
	}
}
