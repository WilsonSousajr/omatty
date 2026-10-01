package review_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/review"
)

// What a file's bytes are, for the preview (#24): text split into lines, a
// NUL meaning binary, and more than the limit truncated back to the last
// whole line. Pure since migration step 5.8 (#653); the read is fsread's.
func TestPreviewOf_issue24(t *testing.T) {
	text := review.PreviewOf("a.go", []byte("one\ntwo\n"))
	if len(text.Lines) != 2 || text.Lines[1] != "two" || text.Binary || text.Truncated {
		t.Errorf("text = %+v, want two lines", text)
	}
	if bin := review.PreviewOf("a.bin", []byte("x\x00y")); !bin.Binary || bin.Lines != nil {
		t.Errorf("binary = %+v, want Binary and no lines", bin)
	}
	long := []byte(strings.Repeat("0123456789\n", review.PreviewLimit/11+2))
	if big := review.PreviewOf("big.txt", long); !big.Truncated || strings.HasSuffix(strings.Join(big.Lines, "\n"), "01234") {
		t.Errorf("big: Truncated=%v, want truncated back to a whole line", big.Truncated)
	}
}
