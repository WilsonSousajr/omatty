package gitdiff_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
)

const genSessionDiff = `diff --git a/f.go b/f.go
index 1111111..2222222 100644
--- a/f.go
+++ b/f.go
@@ -1 +1,5 @@
 package f
+func a() {
+}
+func b() {
+}
`

const genTurnDiff = `diff --git a/f.go b/f.go
index 3333333..2222222 100644
--- a/f.go
+++ b/f.go
@@ -1,3 +1,5 @@
 package f
 func a() {
 }
+func b() {
+}
`

const genDigestBefore = `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -10,3 +10,3 @@ func f() {
 	a := 1
-	b := 2
+	b := 3
 }
`
const genDigestAfter = `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -10,3 +10,3 @@ func f() {
 	a := 1
-	b := 2
+	b := 4
 }
`
const genDigestMoved = `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -40,3 +41,3 @@ func f() {
 	a := 1
-	b := 2
+	b := 3
 }
`

// TestGenerateParsedFixtures writes the body of the pre-parsed diff table that
// domain/review's parsed_test.go and this package's parsed_guard_test.go both
// hold (migration step 3.6a, #635). Run it when the guard fails:
//
//	GEN_FIXTURES=1 GEN_DOMAIN=/tmp/d GEN_GUARD=/tmp/g go test ./internal/infra/gitdiff -run TestGenerateParsedFixtures
//
// then paste the output between each table's braces.
func TestGenerateParsedFixtures(t *testing.T) {
	if os.Getenv("GEN_FIXTURES") == "" {
		t.Skip("set GEN_FIXTURES=1 to regenerate")
	}
	edited := strings.Replace(twoFileDiff, "+\tb := 3", "+\tb := 99", 1)
	raws := []string{
		twoFileDiff, dupBraceDiff, genSessionDiff, genTurnDiff,
		strings.Replace(genTurnDiff, "@@ -1,3 +1,5 @@\n package f\n", "@@ -2,2 +2,4 @@\n", 1),
		strings.Replace(twoFileDiff, "@@ -10,4 +10,5 @@", "@@ -30,4 +31,5 @@", 1),
		edited,
		edited[:strings.Index(edited, "diff --git a/new.txt")],
		genDigestBefore, genDigestAfter, genDigestMoved,
		twoFileDiff[:strings.Index(twoFileDiff, "diff --git a/new.txt")],
	}
	sort.Strings(raws)
	var b strings.Builder
	for _, raw := range raws {
		fmt.Fprintf(&b, "\t%q: %#v,\n", raw, parse(t, raw))
	}
	for _, out := range []string{os.Getenv("GEN_DOMAIN"), os.Getenv("GEN_GUARD")} {
		if err := os.WriteFile(out, []byte(b.String()), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}
