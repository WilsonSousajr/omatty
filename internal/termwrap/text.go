// Stream selection over a pane's own grid (#360). omatty draws the sidebar,
// the session pane and the review column on the same screen rows, so the host
// terminal's selection - which knows only the whole screen - picks all three
// up as soon as a drag wraps. Reading the pane's cells is how a drag copies
// the pane alone.
//
// This reads the cell grid, which invariant 2 forbids for inferring session
// state. Copying what is displayed is not inferring status: nothing here
// reaches watcher, and no status is derived from a cell. The invariant guards
// against guessing at state the JSONL already reports, and a clipboard is not
// that.

package termwrap

import "strings"

// cellAt reports a cell's content and mono-spaced width. A width of 0 marks
// the placeholder half of a double-width grapheme, whose content belongs to
// the cell before it.
type cellAt func(x, y int) (content string, width int)

// streamText joins the cells from (x0,y0) to (x1,y1) the way a terminal's own
// selection does: the rest of the first row, every row between, then the last
// row up to x1. Rows join with "\n" and each drops its trailing blanks, so a
// wrapped sentence arrives as text rather than padded fragments.
//
// width bounds every row but the last. A selection dragged up or to the left
// arrives with its ends reversed, so they are put back in order here rather
// than in each caller.
func streamText(x0, y0, x1, y1, width int, at cellAt) string {
	if y1 < y0 || (y1 == y0 && x1 < x0) {
		x0, y0, x1, y1 = x1, y1, x0, y0
	}
	rows := make([]string, 0, y1-y0+1)
	for y := y0; y <= y1; y++ {
		from, to := 0, width-1
		if y == y0 {
			from = x0
		}
		if y == y1 {
			to = x1
		}
		rows = append(rows, strings.TrimRight(rowText(y, from, to, at), " "))
	}
	return strings.Join(rows, "\n")
}

// rowText walks one row's run, skipping the placeholder cells that carry no
// content of their own.
func rowText(y, from, to int, at cellAt) string {
	var b strings.Builder
	for x := from; x <= to; x++ {
		content, cellWidth := at(x, y)
		if cellWidth == 0 {
			continue
		}
		if content == "" {
			content = " "
		}
		b.WriteString(content)
	}
	return b.String()
}

// Text returns the pane's own cells for a stream selection, so a drag copies
// the session without the panes beside it (#360).
//
//	text := term.Text(2, 0, 3, 1) // the rest of row 0, then row 1 up to x=3
func (b *bubble) Text(x0, y0, x1, y1 int) string {
	emu := b.m.GetEmulator()
	return streamText(x0, y0, x1, y1, b.w, func(x, y int) (string, int) {
		cell := emu.CellAt(x, y)
		if cell == nil {
			return "", 1
		}
		return cell.Content, cell.Width
	})
}

// Text reads the grid a test set on Grid, one rune per cell, so ui can assert
// what a drag copies without a real emulator behind it (#360).
func (f *Fake) Text(x0, y0, x1, y1 int) string {
	rows := make([][]rune, len(f.Grid))
	width := 0
	for i, line := range f.Grid {
		rows[i] = []rune(line)
		width = max(width, len(rows[i]))
	}
	return streamText(x0, y0, x1, y1, width, func(x, y int) (string, int) {
		if y < 0 || y >= len(rows) || x < 0 || x >= len(rows[y]) {
			return "", 1
		}
		return string(rows[y][x]), 1
	})
}
