// Package highlight is omatty's seam over chroma, the way termwrap is over
// bubbleterm and vcs over git: nothing else imports the highlighter, so a
// change in it lands in one package (invariant 4 in spirit, #197).
//
//	styled := highlight.Lines("internal/ui/model.go", plainLines)
package highlight

import (
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
)

// Lines colours lines as the file at path, one SGR-styled line per plain
// line, every line closing its own styling. A path chroma has no lexer for,
// and any input chroma would re-line (a bare carriage return becomes a
// newline in its normalisation), come back as they went in: the preview's
// gutter numbers the plain lines, and a styled line that is not the same
// line would number the wrong text.
func Lines(path string, lines []string) []string {
	lexer := lexers.Match(path)
	if lexer == nil || len(lines) == 0 {
		return lines
	}
	styled, err := format(chroma.Coalesce(lexer), strings.Join(lines, "\n"))
	if err != nil {
		return lines
	}
	out := splitLines(styled, lines[len(lines)-1] == "")
	if len(out) != len(lines) {
		return lines
	}
	return out
}

// format runs the whole file through the 256-colour terminal formatter. One
// pass over the joined text rather than one per line, because a lexer's
// state (a block comment, a raw string) spans lines.
func format(lexer chroma.Lexer, text string) (string, error) {
	tokens, err := lexer.Tokenise(nil, text)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := formatters.TTY256.Format(&b, style(), tokens); err != nil {
		return "", err
	}
	return b.String(), nil
}

// splitLines splits the formatter's output on the newlines it kept. chroma
// appends one newline when the text lacks a final one, which would read as
// an extra empty row, so that row is dropped - unless the input's own last
// line was empty, in which case the join produced the final newline itself
// and the empty row is the file's.
func splitLines(styled string, endsEmpty bool) []string {
	out := strings.Split(styled, "\n")
	if n := len(out); !endsEmpty && n > 1 && out[n-1] == "" {
		out = out[:n-1]
	}
	return out
}
