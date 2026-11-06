// Syntax and word-level highlighting in the diff (#435).
//
// M5 deferred it as "one more caller of #197's package once that exists"; it
// exists. A line's text takes its syntax colours and its sign keeps the diff's
// meaning - green added, red removed - so neither colour fights the other. In
// a removed/added pair the words that changed get a stronger background, as
// delta and GitHub draw them: a one-token edit reads without diffing by eye.

package app

import (
	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
	"github.com/WilsonSousajr/omatty/internal/tui/theme"
	"unicode"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// hunkStyle is a hunk drawn: each line's text finished - syntax-coloured, its
// changed words emphasised, or in the line's one colour where chroma had
// nothing. Memoised per hunk until the entries are rebuilt, so a frame costs a
// lookup and the pan, not a lexer and a cut per row (#314's budget).
type hunkStyle struct {
	drawn []string
}

// wordSpan is the cells [from, to) of a line's changed words; empty when the
// line has none to show.
type wordSpan struct{ from, to int }

// hunkKey names one hunk of the shown diff.
type hunkKey struct{ file, hunk int }

// emphasisStyle is a pair's changed words: the line's own hue on a darker
// ground of the same meaning - 22 a green, 52 a red - so the emphasis says
// "this part" without saying anything new.
func emphasisStyle(k dreview.LineKind) lipgloss.Style {
	if k == dreview.LineRemoved {
		return theme.RemovedEmphasis
	}
	return theme.AddedEmphasis
}

// hunkStyleAt is p's hunk, drawn once and remembered.
func (m *Model) hunkStyleAt(p dreview.Position) hunkStyle {
	key := hunkKey{p.File, p.Hunk}
	if hs, ok := m.review.HunkStyles[key]; ok {
		return hs
	}
	f := m.shownDiff().Files[p.File]
	hs := drawHunk(m.highlighter, f.Path, f.Hunks[p.Hunk].Lines)
	if m.review.HunkStyles == nil {
		m.review.HunkStyles = map[hunkKey]hunkStyle{}
	}
	m.review.HunkStyles[key] = hs
	return hs
}

// drawHunk finishes every line of a hunk once: syntax colours from chroma,
// the line's one colour where chroma had none, and a pair's changed words.
func drawHunk(hl Highlighter, path string, lines []dreview.Line) hunkStyle {
	plain := make([]string, len(lines))
	for i, l := range lines {
		plain[i] = expandTabs(l.Text)
	}
	styled, spans := lexSides(hl, path, lines, plain), wordSpans(lines, plain)
	drawn := make([]string, len(lines))
	for i, l := range lines {
		drawn[i] = drawLine(styled[i], plain[i], spans[i], l.Kind)
	}
	return hunkStyle{drawn: drawn}
}

// lexSides colours a hunk as the two files it compares. The new side - context
// and added lines - and the old side - context and removed lines - are each
// one stream to the lexer, and each line takes its colours from its own side.
// Lexed together, a quote a removed line left open turned every line after it
// into a string (#764). Context lines are lexed in both and drawn from the
// new side, the file as it reads now.
//
// Each stream still starts at the hunk, not at the file: a hunk that opens
// inside a raw string or a block comment is lexed as code. Priming the lexer
// from the file's start is #800.
func lexSides(hl Highlighter, path string, lines []dreview.Line, plain []string) []string {
	styled := append([]string(nil), plain...)
	lexSide(hl, path, sideLines(lines, dreview.LineRemoved), plain, styled)
	lexSide(hl, path, sideLines(lines, dreview.LineAdded), plain, styled) // second: context is drawn from the new side
	return styled
}

// lexSide lexes the lines at idx as one stream and writes their colours into
// styled. A highlighter that answers with a different number of lines leaves
// them plain rather than shifting one line's colours onto another.
func lexSide(hl Highlighter, path string, idx []int, plain, styled []string) {
	text := make([]string, len(idx))
	for k, i := range idx {
		text[k] = plain[i]
	}
	out := hl.Lines(path, text)
	if len(out) != len(idx) {
		return
	}
	for k, i := range idx {
		styled[i] = out[k]
	}
}

// sideLines is the indexes of the lines one side of the hunk holds: its own
// kind and the context the two share.
func sideLines(lines []dreview.Line, side dreview.LineKind) []int {
	var idx []int
	for i, l := range lines {
		if l.Kind == side || l.Kind == dreview.LineContext {
			idx = append(idx, i)
		}
	}
	return idx
}

// drawLine is one line's text as drawn: its changed words on the emphasis
// ground, the rest in the syntax's colours or, with none, the line's own.
func drawLine(styled, plain string, sp wordSpan, kind dreview.LineKind) string {
	if styled == plain {
		styled = lineStyle(kind).Render(plain)
	}
	if sp.to <= sp.from {
		return styled
	}
	return ansi.Cut(styled, 0, sp.from) + emphasisStyle(kind).Render(ansi.Strip(ansi.Cut(styled, sp.from, sp.to))) +
		ansi.Cut(styled, sp.to, sp.to+lipgloss.Width(plain))
}

// styledLine is a diff line off the cursor: its sign in the line's colour -
// the diff's meaning, kept where the syntax cannot overwrite it - then its
// finished text, panned.
func (m *Model) styledLine(e dreview.Entry, w int) string {
	kind := m.shownDiff().LineAt(e.Pos).Kind
	text := m.hunkStyleAt(e.Pos).drawn[e.Pos.Line]
	return m.fitStyled(lineStyle(kind).Render(m.linePrefix(e))+text, w)
}

// wordSpans pairs each run of removed lines with the run of added lines after
// it, line by line, and finds what changed in each pair.
func wordSpans(lines []dreview.Line, plain []string) []wordSpan {
	spans := make([]wordSpan, len(lines))
	for i := 0; i < len(lines); {
		removed := runOf(lines, i, dreview.LineRemoved)
		added := runOf(lines, i+removed, dreview.LineAdded)
		for k := range min(removed, added) {
			spans[i+k], spans[i+removed+k] = changedWords(plain[i+k], plain[i+removed+k])
		}
		i += max(removed+added, 1)
	}
	return spans
}

// runOf is how many lines from i are of kind.
func runOf(lines []dreview.Line, i int, kind dreview.LineKind) int {
	n := 0
	for i+n < len(lines) && lines[i+n].Kind == kind {
		n++
	}
	return n
}

// changedWords is the differing middle of a and b, widened to whole words, as
// cells in each. Nothing when either line changed entirely: emphasising all of
// a line says no more than its colour already does.
func changedWords(a, b string) (wordSpan, wordSpan) {
	ra, rb := []rune(a), []rune(b)
	pre := commonPrefix(ra, rb)
	suf := commonSuffix(ra[pre:], rb[pre:])
	for pre > 0 && isWordRune(ra[pre-1]) {
		pre--
	}
	for suf > 0 && isWordRune(ra[len(ra)-suf]) {
		suf--
	}
	if pre == 0 && suf == 0 {
		return wordSpan{}, wordSpan{}
	}
	return spanOf(ra, pre, suf), spanOf(rb, pre, suf)
}

// commonPrefix is how many leading runes a and b share.
func commonPrefix(a, b []rune) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

// commonSuffix is how many trailing runes a and b share.
func commonSuffix(a, b []rune) int {
	n := 0
	for n < len(a) && n < len(b) && a[len(a)-1-n] == b[len(b)-1-n] {
		n++
	}
	return n
}

// spanOf is the cells between a common prefix and suffix of r.
func spanOf(r []rune, pre, suf int) wordSpan {
	return wordSpan{from: lipgloss.Width(string(r[:pre])), to: lipgloss.Width(string(r[:len(r)-suf]))}
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }
