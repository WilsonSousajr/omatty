package ui

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

// PanStep is how far one h, one l or one sideways wheel notch moves the review
// column, so a test spins a real gesture rather than hard-coding 8 (#125).
const PanStep = panStep
