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
	start := min(m.review.PreviewOffset, max(len(p.Lines)-rows, 0))
	end := min(start+rows, len(p.Lines))
	out := make([]string, 0, rows)
	for i := start; i < end; i++ {
		out = append(out, m.fitContent(previewRow(i, p.Lines[i]), w))
	}
	if p.Truncated && end == len(p.Lines) {
		out = append(out, mutedStyle.Render("... truncated at 256 KiB"))
	}
	return out
}
