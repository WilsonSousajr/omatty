package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// benchDiffModel is a model showing one 200-line Go hunk of paired edits in
// the diff column: what #435's highlighting costs a frame.
func benchDiffModel(b *testing.B) *Model {
	b.Helper()
	var d strings.Builder
	d.WriteString("diff --git a/x.go b/x.go\n--- a/x.go\n+++ b/x.go\n@@ -1,100 +1,100 @@\n")
	for i := range 100 {
		fmt.Fprintf(&d, "-\tvalue%d := compute(%d, \"old\")\n+\tvalue%d := compute(%d, \"new\")\n", i, i, i, i)
	}
	diff, err := review.ParseDiff(strings.NewReader(d.String()))
	if err != nil {
		b.Fatal(err)
	}
	m := benchModel(b)
	m.review = ReviewPane{Open: true, View: ViewDiff, Diff: diff}
	m.review.Entries = review.Flatten(diff, review.Placed{})
	return m
}

// BenchmarkDiffRows draws a screen of diff rows, the memo warm after the
// first frame as it is for every frame after a diff lands.
func BenchmarkDiffRows(b *testing.B) {
	m := benchDiffModel(b)
	_ = m.renderEntries(80, 40)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		_ = m.renderEntries(80, 40)
	}
}
