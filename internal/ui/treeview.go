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
	nodes := m.treeRows()
	if nodes == nil {
		return []string{mutedStyle.Render("listing files...")}
	}
	off := ScrollOffset(m.review.TreeCursor, m.review.TreeOffset, rows)
	out := make([]string, 0, rows)
	for i := off; i < min(off+rows, len(nodes)); i++ {
		text := m.fitContent(treeText(nodes[i], m.review.Tree.Collapsed(nodes[i].Path)), w)
		out = append(out, treeStyle(nodes[i], i == m.review.TreeCursor).Render(text))
	}
	return out
}

// treeText is "  ▾ dir/" or "  * file", indented by depth; * marks a file the
// session changed, or a directory holding one.
func treeText(n review.TreeNode, collapsed bool) string {
	mark := " "
	if n.Touched {
		mark = "*"
	}
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

func treeStyle(n review.TreeNode, cursor bool) lipgloss.Style {
	switch {
	case cursor:
		return cursorStyle
	case n.Touched:
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
