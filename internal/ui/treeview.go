package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
)

// renderTree draws the worktree listing with the cursor row reversed. The
// offset is recomputed rather than trusted, because a resize can shrink the
// window after the cursor last moved.
func (m *Model) renderTree(w, rows int) []string {
	if m.review.TreeErr != "" {
		return []string{errorStyle.Render(fitLine(m.review.TreeErr, w))}
	}
	// Three states, told apart explicitly: no *Tree is "not listed yet", a
	// *Tree with no rows is "listed, and there is nothing" (#131).
	if m.review.Tree == nil {
		return []string{mutedStyle.Render("listing files...")}
	}
	nodes := m.treeRows()
	if len(nodes) == 0 {
		return []string{mutedStyle.Render(fitLine(emptyTreeHint, w))}
	}
	return m.treeLines(nodes, w, rows)
}

// treeLines draws the window of rows around the cursor.
func (m *Model) treeLines(nodes []review.TreeNode, w, rows int) []string {
	off := ScrollOffset(m.review.TreeCursor, m.review.TreeOffset, rows)
	out := make([]string, 0, rows)
	for i := off; i < min(off+rows, len(nodes)); i++ {
		text := m.fitContent(treeText(nodes[i], m.review.Tree.Collapsed(nodes[i].Path)), w)
		out = append(out, treeStyle(nodes[i], i == m.review.TreeCursor).Render(text))
	}
	return out
}

// treeText is "M ▾ dir/" or "A file", indented by depth; the letter is the
// kind of change the session made - M A D R - or a space for none, and a
// directory holding a changed file reads as M (#196). Before that one *
// marked every kind.
func treeText(n review.TreeNode, collapsed bool) string {
	mark := changeLetter(n.Change)
	indent := strings.Repeat("  ", n.Depth)
	if !n.IsDir {
		return indent + mark + " " + n.Name
	}
	arrow := "▾"
	if collapsed {
		arrow = "▸"
	}
	return indent + mark + " " + arrow + " " + n.Name + "/"
}

// changeLetter is the one-cell mark column: nvim-tree, yazi and lazygit all
// show the kind of change, not just that there was one.
func changeLetter(c review.Change) string {
	switch c {
	case review.ChangeModified:
		return "M"
	case review.ChangeAdded:
		return "A"
	case review.ChangeDeleted:
		return "D"
	case review.ChangeRenamed:
		return "R"
	}
	return " "
}

// treeStyle colours a row by its change in the hues the diff already gives
// those states - green added, red removed - and amber for a modified or
// renamed file, the operator's-attention hue; that is what keeps style.go's
// one-hue-one-meaning rule with no new colour (#196). The cursor's reverse
// wins over all of them so the row is found at a glance.
func treeStyle(n review.TreeNode, cursor bool) lipgloss.Style {
	switch {
	case cursor:
		return cursorStyle
	case n.Change == review.ChangeAdded:
		return addedStyle
	case n.Change == review.ChangeDeleted:
		return removedStyle
	case n.Change != review.ChangeNone:
		return commentStyle
	case n.IsDir:
		return headerStyle
	}
	return lipgloss.NewStyle()
}

// renderPreview draws the file's lines from the scroll offset, numbered, and
// from the column offset horizontally (#94).
func (m *Model) renderPreview(w, rows int) []string {
	p := m.review.Preview
	if p.Binary {
		return []string{mutedStyle.Render(p.Path + " is a binary file")}
	}
	// The path is in the title; the body only needs the reason, which then
	// also fits the narrowest column.
	if p.Deleted {
		return []string{mutedStyle.Render(fitLine("deleted in this session", w))}
	}
	// The offset is re-clamped here rather than trusted, for the reason
	// renderEntries and renderTree give: a resize changes rows after the cursor
	// last moved. Growing the window used to strand a bottom-scrolled preview
	// against a taller pane, showing its last few lines above a column of
	// blanks (#94, #95).
	start := min(m.review.PreviewOffset, previewLast(p, rows))
	end := min(start+rows, len(p.Lines))
	out := make([]string, 0, rows)
	for i := start; i < end; i++ {
		out = append(out, m.previewLine(p, i, w))
	}
	if p.Truncated && end == len(p.Lines) {
		out = append(out, mutedStyle.Render("... truncated at 256 KiB"))
	}
	if p.Unhighlighted && end == len(p.Lines) {
		// Short enough for the narrowest column (23 cells), like the
		// truncation note above it.
		out = append(out, mutedStyle.Render("... not highlighted"))
	}
	return out
}

// previewLast is the furthest offset the preview scrolls to: the one that
// shows the last line and, under it, the notes about what the view is not
// showing. Without the notes in the count the last offset put them past the
// pane's bottom, where fitBlock cut them off unseen (#197).
func previewLast(p review.Preview, rows int) int {
	return max(len(p.Lines)+previewNotes(p)-rows, 0)
}

// previewNotes counts the muted lines renderPreview appends after the file.
func previewNotes(p review.Preview) int {
	n := 0
	if p.Truncated {
		n++
	}
	if p.Unhighlighted {
		n++
	}
	return n
}

// previewLine draws one row: the styled line through the ANSI-aware cut when
// the file was highlighted, the plain one through the cell cut otherwise. The
// gutter is prepended after highlighting so a number is never coloured (#197).
func (m *Model) previewLine(p review.Preview, i, w int) string {
	if p.Styled == nil {
		return m.fitContent(previewRow(i, p.Lines[i]), w)
	}
	return m.fitStyled(previewGutter(i)+p.Styled[i], w)
}
