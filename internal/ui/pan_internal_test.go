package ui

import (
	"strings"
	"testing"
)

// Regression, issue #94: the review column had no horizontal axis at all, so
// anything past the pane width was unreachable. panLine is the cut that makes
// one possible; it counts display cells rather than bytes or runes, because a
// wide rune occupies two columns and a byte count would slice one in half.
func TestPanLine_DropsLeadingCells_issue94(t *testing.T) {
	tests := []struct {
		name string
		s    string
		cols int
		want string
	}{
		{"zero keeps the line", "hello", 0, "hello"},
		{"drops leading cells", "hello world", 6, "world"},
		{"past the end is empty", "hello", 99, ""},
		{"exactly the end is empty", "hello", 5, ""},
		{"negative is treated as zero", "hello", -3, "hello"},
		// A CJK rune is two cells wide: panning by one lands mid-rune, and the
		// whole rune has to go rather than half of its bytes.
		{"wide runes count two cells", "中文x", 2, "文x"},
		{"a cut inside a wide rune drops it", "中文x", 1, "文x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := panLine(tt.s, tt.cols); got != tt.want {
				t.Errorf("panLine(%q, %d) = %q, want %q", tt.s, tt.cols, got, tt.want)
			}
		})
	}
}

// benchPreview is a preview at the 256 KiB bound review.Preview is read to,
// which is the most reviewMaxWidth can ever be asked to walk.
func benchPreview(lines int) *Model {
	m := &Model{width: 100, height: 30}
	m.review.Open, m.review.View = true, ViewPreview
	m.review.Preview.Lines = make([]string, lines)
	for i := range m.review.Preview.Lines {
		m.review.Preview.Lines[i] = strings.Repeat("x", 40)
	}
	return m
}

// The number behind panReview's short-circuit, so the comment there is a
// measurement rather than a claim: a rightward pan rebuilds and measures every
// row, and a trackpad flick asks for dozens of them in a burst on the goroutine
// PTY output queues on. Panning left skips the walk entirely (#125).
func BenchmarkPanReview(b *testing.B) {
	for _, tt := range []struct {
		name  string
		delta int
	}{
		{"right walks the content", panStep},
		{"left is free", -panStep},
	} {
		b.Run(tt.name, func(b *testing.B) {
			m := benchPreview(6500)
			for b.Loop() {
				m.review.ColOffset = 16
				m.panReview(tt.delta)
			}
		})
	}
}
