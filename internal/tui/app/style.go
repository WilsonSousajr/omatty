package app

import (
	"github.com/WilsonSousajr/omatty/internal/tui/theme"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/service/review"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// glyphStyle colours a status glyph.
func glyphStyle(s status.Status) lipgloss.Style {
	return theme.Foreground(glyphColor(s))
}

// statusCells is every status's coloured glyph, rendered once.
//
// The pair (colour, glyph) is fixed per status, so a card that redraws the
// same status draws the same cell - but it was restyled and re-rendered on
// every card of every frame, and a frame is drawn on every message. Built on
// first use rather than in an initialiser, the pattern internal/infra/highlight
// uses for its style.
var statusCells = sync.OnceValue(func() map[status.Status]string {
	cells := make(map[status.Status]string, len(statusGlyphs))
	for st := range statusGlyphs {
		cells[st] = glyphStyle(st).Render(statusGlyph(st))
	}
	return cells
})

// statusCell is a status's coloured glyph. A status with no glyph of its own
// falls through to the same rendering statusGlyph's "-" default would give.
func statusCell(s status.Status) string {
	if c, ok := statusCells()[s]; ok {
		return c
	}
	return glyphStyle(s).Render(statusGlyph(s))
}

// statusGlyphs pairs each status with its one-column marker (#175). The
// filled dot is waiting, the loudest glyph for the loudest state; the shapes
// differ enough to read with colour off. All are Geometric Shapes or
// Mathematical Operators, so no font draws them two cells wide - the gear and
// the pause sign #128 warned about are gone. ○ ◐ ◆ ● are East Asian
// Ambiguous like the rail: RUNEWIDTH_EASTASIAN=1 doubles
// all of them or none. A status not listed renders "-".
var statusGlyphs = map[status.Status]string{
	status.StatusIdle: "○", status.StatusThinking: "◐", status.StatusTool: "◆",
	status.StatusWaiting: "●", status.StatusDone: "✓", status.StatusError: "✕",
	status.StatusExited: "∅",
}

func statusGlyph(s status.Status) string {
	if g, ok := statusGlyphs[s]; ok {
		return g
	}
	return "-"
}

// entryStyle colours a review row by what it is; a line row is coloured by
// what the diff did to it.
func entryStyle(e review.Entry, d review.Diff) lipgloss.Style {
	switch e.Kind {
	case review.EntryFile:
		return theme.Header
	case review.EntryHunk:
		return theme.Muted
	case review.EntryComment, review.EntryOrphan:
		return theme.Comment
	}
	return lineStyle(d.LineAt(e.Pos).Kind)
}

func lineStyle(k review.LineKind) lipgloss.Style {
	switch k {
	case review.LineAdded:
		return theme.Added
	case review.LineRemoved:
		return theme.Removed
	}
	return theme.Plain()
}
