package highlight_test

import (
	"fmt"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/highlight"
)

// The number behind the ui's highlight budget: previewFile reads on the
// Update goroutine, so lexing has to stay cheap. Measured on 2026-09-09:
// about 40 ms at 64 KiB, 170 ms at the 256 KiB read bound (#197).
func BenchmarkLines(b *testing.B) {
	for _, size := range []int{64 << 10, 256 << 10} {
		var lines []string
		for n := 0; n < size; {
			for _, l := range goSnippet {
				lines = append(lines, l)
				n += len(l) + 1
			}
		}
		b.Run(fmt.Sprintf("%dKiB", size>>10), func(b *testing.B) {
			for b.Loop() {
				highlight.Lines("main.go", lines)
			}
		})
	}
}
