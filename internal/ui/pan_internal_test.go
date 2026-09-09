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

// benchStyledPreview is benchPreview with every line highlighted, the shape
// #197 added: the pan clamp must still measure the plain lines, and the
// budget #133 set must survive the styled row's ANSI-aware cut.
func benchStyledPreview(lines int) *Model {
	m := benchPreview(lines)
	m.review.Preview.Styled = make([]string, lines)
	for i := range m.review.Preview.Styled {
		m.review.Preview.Styled[i] = "\x1b[38;5;111m" + strings.Repeat("x", 20) + "\x1b[0m" + strings.Repeat("x", 20)
	}
	return m
}

// A highlighted preview clamps to the same ceiling as the plain one: the
// escapes are not cells, and the plain lines are what is measured (#197).
func TestPanReview_AStyledPreviewClampsToItsPlainWidth_issue197(t *testing.T) {
	plain, styled := benchPreview(50), benchStyledPreview(50)
	for range 60 {
		plain.panReview(panStep)
		styled.panReview(panStep)
	}
	if plain.review.ColOffset != styled.review.ColOffset {
		t.Errorf("styled ceiling %d, plain ceiling %d; want the same", styled.review.ColOffset, plain.review.ColOffset)
	}
}

// The styled row's cut is on the render path, not the pan path, so the pan
// budget is unchanged; this benchmark pins the cost of drawing a styled
// row against the plain one, which is where #197's cost lives.
func BenchmarkPreviewLine(b *testing.B) {
	for _, tt := range []struct {
		name string
		m    *Model
	}{
		{"plain", benchPreview(10)},
		{"styled", benchStyledPreview(10)},
	} {
		b.Run(tt.name, func(b *testing.B) {
			tt.m.review.ColOffset = 16
			for b.Loop() {
				tt.m.previewLine(tt.m.review.Preview, 3, 27)
			}
		})
	}
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

// A burst of rightward notches must cost one walk, not one per notch. The
// test proves it without a counter: after the first pan, a line is widened
// behind the cache's back; the ceiling must not move until contentChanged
// is called, which is what every real mutation site does (#133).
func TestPanReview_ABurstWalksTheContentOnce_issue133(t *testing.T) {
	m := benchPreview(50)
	w := reviewContentWidth(m.width)
	ceiling := 46 - w // previewRow of forty x's is "%4d  " plus forty cells
	m.panReview(panStep)
	if !m.review.Widest.valid {
		t.Fatal("the first pan did not memoize the width")
	}

	m.review.Preview.Lines[0] = strings.Repeat("x", 400)
	for range 60 {
		m.panReview(panStep)
	}
	if m.review.ColOffset != ceiling {
		t.Fatalf("ColOffset after a burst = %d, want the memoized ceiling %d: the content was re-walked", m.review.ColOffset, ceiling)
	}

	m.contentChanged()
	m.panReview(panStep)
	if m.review.ColOffset != ceiling+panStep {
		t.Errorf("after contentChanged ColOffset = %d, want %d: the new width was not walked", m.review.ColOffset, ceiling+panStep)
	}
}

// Switching views walks the other view's content: the cache is per view.
func TestPanReview_TheCacheIsPerView_issue133(t *testing.T) {
	m := benchPreview(50)
	m.panReview(panStep)
	m.review.View = ViewTree
	if got := m.reviewMaxWidth(); got != 0 {
		t.Errorf("reviewMaxWidth on an unlisted tree = %d; the preview's width served the tree", got)
	}
}
