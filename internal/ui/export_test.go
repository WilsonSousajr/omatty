package ui

import (
	"image/color"

	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// Test-only accessors, so the external ui_test package can assert against the
// real tables and constants rather than hand-copied duplicates of them.
//
// The duplicates are the point. Issue #103 was a key that existed in the router
// and in no keymap the operator could see; a test that spells the keymap out a
// third time cannot catch that, because it drifts with neither. These make the
// production values themselves the fixture.

// LeaderKeys is every documented leader binding's key.
func LeaderKeys() []string {
	out := make([]string, 0, len(leaderKeys))
	for _, k := range leaderKeys {
		out = append(out, k.Key)
	}
	return out
}

// Footers is every footer constant by name, so a width assertion measures the
// constant rather than the rendered line - which fitLine has already capped to
// the window and which therefore cannot fail for an over-long footer.
func Footers(leader string) map[string]string {
	return map[string]string{
		"footer":       footerLine(leader),
		"reviewFooter": reviewFooterLine(leader),
		"treeFooter":   treeFooterLine(leader),
	}
}

// SidebarRows is how many rows the sidebar holds, so a test helper that walks
// the cursor can bound its loop instead of spinning when it never arrives.
func (m *Model) SidebarRows() int { return len(m.sidebar.Rows()) }

// SidebarTop is the window row of the first scrollable sidebar row, so a
// click test can aim at a row by index (#45).
func SidebarTop() int { return sidebarTop() }

// StatusGlyphs is every status marker, so a width test measures the real
// table (#128).
func StatusGlyphs() []string {
	out := make([]string, 0, len(statusGlyphs))
	for _, g := range statusGlyphs {
		out = append(out, g)
	}
	return out
}

// LaneBlocks is the status-to-cell table and LaneCells the lane's width.
func LaneBlocks() map[watcher.Status]string { return laneBlock }
func LaneCells() int                        { return laneCells }

// LaneOf is a session's rendered lane; RowOf its whole sidebar row.
func (m *Model) LaneOf(id string) string { return m.renderLane(id) }
func (m *Model) RowOf(id string) string {
	for _, row := range m.sidebar.Rows() {
		if row.Session != nil && row.Session.ID == id {
			return m.renderRow(row, sidebarContentCols)
		}
	}
	return ""
}

// HairlineCell is one rendered hairline cell, accent or plain, so a test can
// find which edge the accent stands on (#174). HeaderRow and RuleRow build
// the two chrome lines from titles and widths; owner is the index of the
// segment that owns the keys, -1 for none.
func HairlineCell(accent bool) string { return hairlineStyle(accent).Render(hairline) }
func HeaderRow(titles []string, widths []int, owner int) string {
	segs := make([]segment, len(titles))
	for i := range titles {
		segs[i] = segment{title: titles[i], width: widths[i], owns: i == owner}
	}
	return headerRow(segs)
}
func RuleRow(widths []int) string {
	segs := make([]segment, len(widths))
	for i, w := range widths {
		segs[i] = segment{width: w}
	}
	return ruleRow(segs)
}

// PanStep is how far one h, one l or one sideways wheel notch moves the review
// column, so a test spins a real gesture rather than hard-coding 8 (#125).
const PanStep = panStep

// RenderMeter is the rule's meter for t, "" with no input; MeterGlyphs its two
// cells and MeterCells its width (#153).
func RenderMeter(t watcher.Tokens) string {
	share, ok := cacheShare(t)
	if !ok {
		return ""
	}
	return renderMeter(share)
}
func MeterGlyphs() []string { return []string{meterFull, meterEmpty} }
func MeterCells() int       { return meterCells }

// Blend, LaneCellColor and MeterCellColor are the ramps; StatusColor and
// MutedColor their endpoints (#154).
func Blend(a, b color.Color, t float64) color.Color       { return blend(a, b, t) }
func LaneCellColor(s watcher.Status, age int) color.Color { return laneCellColor(s, age) }
func MeterCellColor(i int) color.Color                    { return meterCellColor(i) }
func StatusColor(s watcher.Status) color.Color            { return statusColors[s] }
func MutedColor() color.Color                             { return colorMuted }

// TokensPart is the rule's whole usage segment - meter, share and counts - so
// a test can assert what "in" counts without rebuilding the rule (#170).
func TokensPart(t watcher.Tokens) string { return tokensPart(t) }
