package app

import (
	"strings"
	"testing"

	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
	"github.com/WilsonSousajr/omatty/internal/infra/highlight"
)

// rawStringFile is p.go after the edit: "second" added inside a raw string
// that opened three lines above the hunk.
var rawStringFile = []string{"package p", "", "var s = `", "first", "second", "`", "", "func f() {}"}

// rawStringHunk is the hunk git shows for it: it starts inside the string.
var rawStringHunk = []dreview.Line{
	{Kind: dreview.LineContext, Text: "first", OldNo: 4, NewNo: 4},
	{Kind: dreview.LineAdded, Text: "second", NewNo: 5},
	{Kind: dreview.LineContext, Text: "`", OldNo: 5, NewNo: 6},
	{Kind: dreview.LineContext, Text: "", OldNo: 6, NewNo: 7},
	{Kind: dreview.LineContext, Text: "func f() {}", OldNo: 7, NewNo: 8},
}

// lexedNew is rawStringFile lexed whole, as its new side.
func lexedNew(t *testing.T) *lexedFile {
	t.Helper()
	styled := highlight.Chroma{}.Lines("p.go", rawStringFile)
	return &lexedFile{new: styled, newPlain: rawStringFile}
}

// Regression, issue #800: each side of a hunk was lexed from the hunk's first
// line, so a hunk that opens inside a raw string read its string as code and
// the code after the closing backtick as a string. Primed from the file's
// start, the added line is coloured as the string it is in, and the line
// after the backtick as code.
func TestDiffHighlight_hunkInsideARawStringIsPrimed_issue800(t *testing.T) {
	whole := highlight.Chroma{}.Lines("p.go", rawStringFile)

	got := drawHunk(highlight.Chroma{}, "p.go", rawStringHunk, lexedNew(t)).drawn

	if got[1] != whole[4] {
		t.Errorf("the added line inside the raw string is not coloured as the string:\ngot  %q\nwant %q", got[1], whole[4])
	}
	if got[4] != whole[7] {
		t.Errorf("the code after the closing backtick is not coloured as code:\ngot  %q\nwant %q", got[4], whole[7])
	}
}

// A file that no longer reads as the diff does - edited again since git
// answered - gives a line nothing: the hunk's own lexing stands (#800).
func TestDiffHighlight_aLineTheFileNoLongerHasIsLexedFromTheHunk_issue800(t *testing.T) {
	lf := lexedNew(t)
	lf.newPlain = append([]string(nil), lf.newPlain...)
	lf.newPlain[4] = "edited since"
	cold := drawHunk(highlight.Chroma{}, "p.go", rawStringHunk, nil).drawn

	got := drawHunk(highlight.Chroma{}, "p.go", rawStringHunk, lf).drawn

	if got[1] != cold[1] {
		t.Errorf("a line the file no longer has took the file's colours:\ngot  %q\nwant %q", got[1], cold[1])
	}
	if strings.Contains(got[1], "edited") {
		t.Errorf("the file's text replaced the diff's: %q", got[1])
	}
}
