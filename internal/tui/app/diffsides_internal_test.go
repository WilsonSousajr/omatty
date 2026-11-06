package app

import (
	"testing"

	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
	"github.com/WilsonSousajr/omatty/internal/infra/highlight"
)

// Regression, issue #764: a hunk's removed and added lines were lexed as one
// stream, so a quote a removed line left open turned every added and context
// line after it into a string. The new side - context and added lines - is
// lexed as the file now reads, and the removed lines cannot reach it.
func TestDiffHighlight_removedLineDoesNotBleedIntoAdded_issue764(t *testing.T) {
	// The context line between keeps the removed and added lines from pairing,
	// so no changed-word emphasis differs between the two drawings: only the
	// syntax can.
	hunk := []dreview.Line{
		{Kind: dreview.LineContext, Text: "package p"},
		{Kind: dreview.LineRemoved, Text: `var s = "unterminated`},
		{Kind: dreview.LineContext, Text: "var x = 1"},
		{Kind: dreview.LineAdded, Text: `var t = "fixed"`},
	}
	newSide := []dreview.Line{hunk[0], hunk[2], hunk[3]}

	got := drawHunk(highlight.Chroma{}, "p.go", hunk).drawn
	want := drawHunk(highlight.Chroma{}, "p.go", newSide).drawn

	if got[2] != want[1] {
		t.Errorf("the context line is coloured by the removed line before it:\ngot  %q\nwant %q", got[2], want[1])
	}
	if got[3] != want[2] {
		t.Errorf("so is the added line after it:\ngot  %q\nwant %q", got[3], want[2])
	}
}

// Issue #764: the old side is lexed on its own as well, so a removed line is
// coloured as the file read before the edit - context then removed - and not
// by an added line between them.
func TestDiffHighlight_removedLinesAreLexedAsTheOldFile_issue764(t *testing.T) {
	hunk := []dreview.Line{
		{Kind: dreview.LineAdded, Text: `var s = "open`},
		{Kind: dreview.LineRemoved, Text: `var y = "two"`},
	}
	oldSide := []dreview.Line{hunk[1]}

	got := drawHunk(highlight.Chroma{}, "p.go", hunk).drawn
	want := drawHunk(highlight.Chroma{}, "p.go", oldSide).drawn

	if got[1] != want[0] {
		t.Errorf("the removed line is coloured by the quote an added line opened:\ngot  %q\nwant %q", got[1], want[0])
	}
}

// ShortHighlighter answers with one line fewer than it was given: a broken
// highlighter, for the guard in lexSide.
type ShortHighlighter struct{}

func (ShortHighlighter) Lines(_ string, lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines[1:] {
		out = append(out, "\x1b[31m"+l+"\x1b[m")
	}
	return out
}

// Issue #764: a side the highlighter answers short is left plain, rather than
// sliding each line's colours onto the line before it.
func TestDiffHighlight_aShortAnswerLeavesTheSidePlain_issue764(t *testing.T) {
	plain := []string{"a", "b"}
	lines := []dreview.Line{{Kind: dreview.LineContext, Text: "a"}, {Kind: dreview.LineAdded, Text: "b"}}

	got := lexSides(ShortHighlighter{}, "p.go", lines, plain)

	if got[0] != "a" || got[1] != "b" {
		t.Errorf("lexSides = %q, want the plain lines", got)
	}
}
