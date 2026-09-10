package highlight_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/highlight"
)

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripSGR(s string) string { return sgr.ReplaceAllString(s, "") }

var goSnippet = []string{
	"package main",
	"",
	"// add is a comment",
	"func add(a int) int {",
	"\treturn a + 42 // \"quoted\"",
	"}",
	`var s = "text"`,
}

// A Go file comes back styled, line for line, and stripping the styling
// gives the input back exactly: the gutter and the width measurement rely on
// that (#197).
func TestLines_StylesAGoFileLineForLine_issue197(t *testing.T) {
	got := highlight.Lines("main.go", goSnippet)

	if len(got) != len(goSnippet) {
		t.Fatalf("Lines returned %d lines for %d in", len(got), len(goSnippet))
	}
	styled := 0
	for i := range got {
		if stripSGR(got[i]) != goSnippet[i] {
			t.Errorf("line %d: stripSGR(%q) = %q, want the input %q", i, got[i], stripSGR(got[i]), goSnippet[i])
		}
		if got[i] != goSnippet[i] {
			styled++
		}
	}
	if styled < 3 {
		t.Errorf("only %d lines carry any styling; want keywords, strings and comments coloured", styled)
	}
}

// Every escape a line carries is closed on that line: a row is cut and padded
// on its own, so styling must never leak into the next row. Text after the
// last reset is plain by definition, so it may carry no escape at all.
func TestLines_EveryLineClosesItsOwnStyling_issue197(t *testing.T) {
	for i, line := range highlight.Lines("main.go", goSnippet) {
		tail := line[strings.LastIndex(line, "\x1b[0m")+1:]
		if strings.Contains(tail, "\x1b[") {
			t.Errorf("line %d leaves styling open past its last reset: %q", i, line)
		}
	}
}

func TestLines_AnUnknownExtensionComesBackUnchanged_issue197(t *testing.T) {
	in := []string{"some text", "more text"}
	got := highlight.Lines("notes.zzz", in)
	if strings.Join(got, "|") != strings.Join(in, "|") {
		t.Errorf("Lines = %q, want the input unchanged", got)
	}
}

func TestLines_NoLinesIsNoLines_issue197(t *testing.T) {
	if got := highlight.Lines("main.go", nil); len(got) != 0 {
		t.Errorf("Lines(nil) = %q, want none", got)
	}
}

// chroma normalises a bare carriage return into a newline, which would give
// the preview one more row than the file has and shift every gutter number
// below it. Any input the highlighter would re-line comes back plain.
func TestLines_ABareCarriageReturnFallsBackToPlain_issue197(t *testing.T) {
	in := []string{"package main\rvar x = 1", "func f() {}"}
	got := highlight.Lines("main.go", in)
	if strings.Join(got, "|") != strings.Join(in, "|") {
		t.Errorf("Lines = %q, want the input unchanged rather than a re-lined file", got)
	}
}

// A trailing empty line is a line: a file ending in a newline reads as one
// more row than a file that does not, and the count must survive.
func TestLines_KeepsATrailingEmptyLine_issue197(t *testing.T) {
	in := []string{"package main", ""}
	got := highlight.Lines("main.go", in)
	if len(got) != 2 || stripSGR(got[1]) != "" || got[0] == in[0] {
		t.Errorf("Lines = %q, want two lines, the first styled and the last empty", got)
	}
}

// The style is omatty's own: style.go gives the accent (75) to focus alone
// and amber (214), green (78) and red (203) to comment, added and removed.
// Stock chroma themes spend all four on keywords and strings; the terminal
// formatter maps hex to the nearest index, so this asserts on what is
// emitted, not on what the style file says.
func TestLines_NeverEmitsAReservedHue_issue197(t *testing.T) {
	out := strings.Join(highlight.Lines("main.go", goSnippet), "\n")
	for _, reserved := range []string{"38;5;75m", "38;5;214m", "38;5;78m", "38;5;203m"} {
		if strings.Contains(out, reserved) {
			t.Errorf("highlighted Go carries the reserved hue %s:\n%s", reserved, out)
		}
	}
	hues := map[string]bool{}
	for _, m := range regexp.MustCompile(`38;5;\d+m`).FindAllString(out, -1) {
		hues[m] = true
	}
	if len(hues) < 4 {
		t.Errorf("only %d distinct hues in highlighted Go, want comment, keyword, string and number apart: %v", len(hues), hues)
	}
}
