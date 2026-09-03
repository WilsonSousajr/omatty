package review_test

// twoFileDiff is a modified file with one hunk and a brand-new file, in the
// exact shape `git diff` and `git diff --no-index /dev/null` emit.
const twoFileDiff = `diff --git a/internal/ui/model.go b/internal/ui/model.go
index 1111111..2222222 100644
--- a/internal/ui/model.go
+++ b/internal/ui/model.go
@@ -10,4 +10,5 @@ func (m *Model) onKey() {
 	a := 1
-	b := 2
+	b := 3
+	c := 4
 	return
 }
diff --git a/new.txt b/new.txt
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/new.txt
@@ -0,0 +1,2 @@
+fresh
+file
`

// dupBraceDiff has the same line twice in one hunk, for the nth-occurrence
// anchor tests.
const dupBraceDiff = `diff --git a/x.go b/x.go
index 1111111..2222222 100644
--- a/x.go
+++ b/x.go
@@ -1,3 +1,3 @@
 }
-old
+new
 }
`
