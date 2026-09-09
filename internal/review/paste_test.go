package review_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/review"
)

// Invariant 8: a multi-line prompt written raw would submit at each newline,
// so the body travels between paste delimiters and only one CR follows.
func TestBracketedPaste_WrapsTheBodyAndSubmitsOnce_issue23(t *testing.T) {
	got := review.BracketedPaste("line one\nline two")

	if !strings.HasPrefix(got, "\x1b[200~line one\nline two\x1b[201~") {
		t.Errorf("envelope = %q, want ESC[200~ body ESC[201~", got)
	}
	if !strings.HasSuffix(got, "\x1b[201~\r") || strings.Count(got, "\r") != 1 {
		t.Errorf("envelope = %q, want exactly one trailing CR after the paste end", got)
	}
}

// Attaching a path must not submit: the operator keeps typing after it, so
// the body travels between the same delimiters with no CR at all (#199).
func TestBracketedText_WrapsTheBodyWithoutSubmitting_issue199(t *testing.T) {
	got := review.BracketedText("@internal/ui/tree.go ")

	if got != "\x1b[200~@internal/ui/tree.go \x1b[201~" {
		t.Errorf("envelope = %q, want ESC[200~ body ESC[201~ and nothing after", got)
	}
}
