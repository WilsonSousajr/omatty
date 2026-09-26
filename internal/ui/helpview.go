// The help modal's layout (#438): opened on the face in front of you, styled
// so it scans, and filtered with / like the tree and the tracker.

package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// helpOrder is the modal's sections in the order they are drawn. With the
// review column open, the face on show comes first and the column's shared
// keys after it: the same ? teaches whatever is in front of you (lazygit,
// diffview's g?). The leader's own keys lead otherwise, as they always did.
func (m *Model) helpOrder() []helpSection {
	leader := helpSection{Title: "after " + m.leader, Keys: leaderKeys, Prefix: m.leader + " "}
	if !m.review.Open {
		return append([]helpSection{leader}, helpSections...)
	}
	face := m.faceHelp()
	order := []helpSection{face, helpSections[0], leader}
	for _, s := range helpSections[1:] {
		if s.Title != face.Title {
			order = append(order, s)
		}
	}
	return order
}

// faceHelp is the section of the face on show.
func (m *Model) faceHelp() helpSection {
	title := map[ReviewView]string{
		ViewTree: "in the file tree", ViewPreview: "in the file tree", ViewGate: "in the gate",
		ViewTracker: "in the tracker", ViewTrackerItem: "in the tracker",
	}[m.review.View]
	for _, s := range helpSections {
		if s.Title == title {
			return s
		}
	}
	return helpSections[1] // the diff
}

// helpBody is the keymap as drawn: each section's title and its rows, rows
// narrowed to the query, a section with none left dropped. The first
// section's title is left out while it is the leader's - the header row
// already names the modal (#188) - so an unfiltered help still opens on its
// first binding.
func (m *Model) helpBody(width int) []string {
	gutter, query := helpGutter(m.leader), strings.ToLower(m.modal.HelpQuery)
	var lines []string
	for i, s := range m.helpOrder() {
		rows := helpRows(s, query, gutter, width)
		if len(rows) == 0 {
			continue
		}
		if i > 0 || s.Prefix == "" {
			lines = append(lines, "", headerStyle.Render(s.Title))
		}
		lines = append(lines, rows...)
	}
	if len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	return lines
}

// helpRows is a section's bindings that hold query, drawn.
func helpRows(s helpSection, query string, gutter, width int) []string {
	var rows []string
	for _, k := range s.Keys {
		key := s.Prefix + k.Key
		if query != "" && !strings.Contains(strings.ToLower(key+" "+k.Does), query) {
			continue
		}
		rows = append(rows, helpRow(key, k.Does, gutter, width))
	}
	return rows
}

// helpRow draws one binding: the key in the accent, what it does as text.
func helpRow(key, does string, gutter, width int) string {
	return fitLine("  "+accentStyle.Render(padRight(key, gutter))+"  "+textStyle.Render(does), width)
}

// helpGutter is the key column's width: the longest key in any table, so a
// new binding widens the column instead of pushing its description out of line
// with every other one (#103).
func helpGutter(leader string) int {
	w := 0
	for _, k := range leaderKeys {
		w = max(w, lipgloss.Width(leader+" "+k.Key))
	}
	for _, section := range helpSections {
		for _, k := range section.Keys {
			w = max(w, lipgloss.Width(k.Key))
		}
	}
	return w
}

// helpFoot is the modal's last line: the filter while it is typed or kept,
// or the keys that close and scroll it, muted.
func (m *Model) helpFoot(width int, scrolls bool) string {
	if m.modal.HelpFiltering || m.modal.HelpQuery != "" {
		return editLine("filter", m.modal.HelpQuery, width)
	}
	if scrolls {
		return mutedStyle.Render("j/k scroll  / filter  esc close  ctrl+c quit")
	}
	return mutedStyle.Render("/ filter  esc close  ctrl+c quit")
}

// editHelpFilter edits the query with the shared editline: enter keeps it,
// esc drops it, and the window goes back to the top of what is left.
func (m *Model) editHelpFilter(msg tea.KeyPressMsg) {
	buffer, action := editKey(m.modal.HelpQuery, msg)
	switch action {
	case editCommit:
		m.modal.HelpFiltering = false
	case editCancel:
		m.modal.HelpFiltering, m.modal.HelpQuery = false, ""
	default:
		m.modal.HelpQuery = buffer
	}
	m.modal.HelpOffset = 0
}

// leaveHelp is esc: a kept filter is lifted first, then the modal closes.
func (m *Model) leaveHelp() {
	if m.modal.HelpQuery != "" {
		m.modal.HelpQuery, m.modal.HelpOffset = "", 0
		return
	}
	m.modal = modal{}
}
