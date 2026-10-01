package cli

import (
	"strings"
	"testing"
)

// Moved from cmd/omatty with ReadLine (migration step 7.2, #653).
func TestReadLine_TakesTheAnswerAndTrimsIt(t *testing.T) {
	for _, tt := range []struct {
		name, in, want string
	}{
		{"a selection", "1 3\n", "1 3"},
		{"trailing spaces", "  all  \n", "all"},
		{"just enter", "\n", ""},
		// The EOF path: a closed stdin is no answer, which is the same as
		// choosing nothing. The old guard for it was dead code - both branches
		// returned "" - and nothing covered either (#91).
		{"eof with no newline", "all", "all"},
		{"eof with nothing at all", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ReadLine(strings.NewReader(tt.in)); got != tt.want {
				t.Errorf("ReadLine(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
