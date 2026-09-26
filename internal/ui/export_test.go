package ui

import (
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/review"
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

// SearchHit is s as the gate draws a search match (#429).
func SearchHit(s string) string { return searchStyle.Render(s) }

// UseNerdIcons switches the model to the Nerd Font glyph set, as
// [ui] icons = "nerd" does through Deps (#425).
func (m *Model) UseNerdIcons() { m.glyphs, m.nerdIcons = nerdGlyphs, true }

// EmphasisAdded and EmphasisRemoved are the changed words of a pair (#435).
func EmphasisAdded(s string) string   { return emphasisStyle(review.LineAdded).Render(s) }
func EmphasisRemoved(s string) string { return emphasisStyle(review.LineRemoved).Render(s) }

// ColumnKeyTables is each review-column face's documented keys, by face, plus
// "column" for the keys every face shares - the tables helpBody renders, so
// the #422 test checks the handlers against what the operator actually sees.
func ColumnKeyTables() map[string][]string {
	keysOf := func(table []keyHelp) []string {
		out := make([]string, 0, len(table))
		for _, k := range table {
			out = append(out, k.Key)
		}
		return out
	}
	return map[string][]string{
		"column": keysOf(columnKeys), "diff": keysOf(diffKeys), "tree": keysOf(treeKeys),
		"gate": keysOf(gateKeys), "tracker": keysOf(trackerKeys),
	}
}

// Footers is every footer constant by name, so a width assertion measures the
// constant rather than the rendered line - which fitLine has already capped to
// the window and which therefore cannot fail for an over-long footer.
func Footers(leader string) map[string]string {
	return map[string]string{
		"footer":       footerLine(leader),
		"reviewFooter": reviewFooterLine(leader),
		"treeFooter":   treeFooterLine(leader),
		// #426's whole-entry test needs every face's line, not only the three
		// the width test was written for.
		"gateFooter":        gateFooterLine(leader),
		"trackerFooter":     trackerFooterLine(leader),
		"trackerItemFooter": trackerItemFooterLine(leader),
	}
}

// SidebarRows is how many rows the sidebar holds, so a test helper that walks
// the cursor can bound its loop instead of spinning when it never arrives.
func (m *Model) SidebarRows() int { return len(m.sidebar.Rows()) }

// SidebarTop is the window row of the first scrollable sidebar row, so a
// click test can aim at a row by index (#45).
func SidebarTop() int { return sidebarTop() }

// RowHeight is the lines a row draws and RowAtLine the row under a drawn
// line, the two halves of the card geometry the window and the click share
// (#176).
func RowHeight(r Row) int                         { return rowHeight(r) }
func (s *Sidebar) RowAtLine(line int) (int, bool) { return s.rowAtLine(line) }

// StatusGlyphs is every status marker, so a width test measures the real
// table (#128).
func StatusGlyphs() []string {
	out := make([]string, 0, len(statusGlyphs))
	for _, g := range statusGlyphs {
		out = append(out, g)
	}
	return out
}

// CardOf is a session's whole card (#176); Rail the accent cursor cell a
// selected card's lines open with.
func (m *Model) CardOf(id string) []string {
	for _, row := range m.sidebar.Rows() {
		if row.Session != nil && row.Session.ID == id {
			return m.renderRow(row, m.clock())
		}
	}
	return nil
}
func Rail() string { return accentStyle.Render(rail) }

// Amber renders s in the waiting colour, so a footer test can find the
// waiting count by its colour (#178).
func Amber(s string) string { return amberStyle.Render(s) }

// Added and Removed render s in the diff colours, so a card test can find
// the diffstat by colour (#180).
func Added(s string) string   { return addedStyle.Render(s) }
func Removed(s string) string { return removedStyle.Render(s) }

// PollAll is one stat tick's worth of polls without the tick that re-arms
// it, so a test can run them without blocking on tea.Tick (#180). RepoStatOf
// is what the model holds for a session.
func (m *Model) PollAll() tea.Cmd { return m.pollAll() }
func (m *Model) RepoStatOf(id string) (review.Stat, bool) {
	st, ok := m.repoStat[id]
	return st, ok
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
func RuleRow(widths []int) string { return RuleRowClosable(widths, -1) }

// RuleRowClosable is RuleRow with the segment at closable carrying the
// review column's label and close glyph; -1 for none (#168).
func RuleRowClosable(widths []int, closable int) string {
	segs := make([]segment, len(widths))
	for i, w := range widths {
		segs[i] = segment{width: w, closable: i == closable, label: "diff"}
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

// Blend and MeterCellColor are the ramp; StatusColor a status's palette
// entry (#154).
func Blend(a, b color.Color, t float64) color.Color { return blend(a, b, t) }
func MeterCellColor(i int) color.Color              { return meterCellColor(i) }
func StatusColor(s watcher.Status) color.Color      { return statusColors[s] }

// AccentColor, AmberColor and TextColor are the palette entries the colour
// rule binds (#175); AllStatuses is every status the tables must cover.
func MeterRamp() (warm, cool color.Color) { return rampWarm, rampCool }
func AccentColor() color.Color            { return colorAccent }
func AmberColor() color.Color             { return colorAmber }
func TextColor() color.Color              { return colorText }
func AllStatuses() []watcher.Status {
	return []watcher.Status{watcher.StatusIdle, watcher.StatusThinking, watcher.StatusTool,
		watcher.StatusWaiting, watcher.StatusDone, watcher.StatusError, watcher.StatusExited}
}

// TokensPart is the rule's whole usage segment - meter, share and counts - so
// a test can assert what "in" counts without rebuilding the rule (#170).
func TokensPart(t watcher.Tokens) string { return tokensPart(t) }

// HeaderParts and Collapse are the pane segment's pieces and the collapse
// order, for the width table (#177). ModalNames is every surface's name.
type HeaderParts = headerParts

func Collapse(width int, p HeaderParts) string { return collapse(width, p) }
func ModalNames() []string {
	out := []string{}
	for _, md := range []modal{
		{Kind: modalPrompt}, {Kind: modalPrompt, Editor: lineEditor{Worktree: true}}, {Kind: modalRename},
		{Kind: modalConfirm}, {Kind: modalList}, {Kind: modalPicker}, {Kind: modalAdopt}, {Kind: modalHelp},
	} {
		out = append(out, modalName(md))
	}
	return out
}

// WaitForClipboard is the wait a pane's OSC 52 copies are picked up by, so a
// test can arm one without draining Init's ticks (#212).
func (m *Model) WaitForClipboard(id string) tea.Cmd { return m.waitForClipboard(id) }

// SetGateReport plants a gate result so the card strip can be rendered without
// running one (#230).
func (m *Model) SetGateReport(id string, rep gate.Report) { m.gates[id] = rep }

// SetRepoStat plants a diffstat, which READY reads (#230).
func (m *Model) SetRepoStat(id string, st review.Stat) { m.repoStat[id] = st }

// CardLines is the height of a session card, so a test asserts against the
// constant the renderer and the click inverse share rather than a literal
// that has to be chased every time it changes (#230).
func CardLines() int { return cardLines }

// SidebarOffset is the first row the last View drew, so a click test can
// resolve a drawn line back to the row under it instead of hard-coding one
// (#45, #230).
func (m *Model) SidebarOffset() int { return m.sidebar.Offset() }

// GateReportCount is how many sessions hold a gate report, so a test can
// assert that a report for an unregistered session was dropped (#231).
func (m *Model) GateReportCount() int { return len(m.gates) }

// CoverageOf is a session's overlay as line verdicts per file, so a test can
// assert what a finished gate loaded without reaching into the model (#254).
// The second return is whether an overlay is held at all, which is distinct
// from one that holds nothing.
func (m *Model) CoverageOf(id string) (map[string]map[int]bool, bool) {
	p, held := m.covers[id]
	if !held {
		return nil, false
	}
	lines := map[string]map[int]bool{}
	for path, f := range p.Files {
		lines[path] = f.Lines
	}
	return lines, true
}

// ArmGateWait is the command Init uses to wait on the next gate report, so a
// test can prove a report actually crosses the channel (#231).
func (m *Model) ArmGateWait() tea.Cmd { return m.waitForGate() }

// SessionBranch is the branch the model believes a session is on, so a test
// can see a rename land without reaching into state.json (#151).
func (m *Model) SessionBranch(id string) string {
	sess, ok := m.session(id)
	if !ok {
		return ""
	}
	return sess.Branch
}

// PollPRs is one PR tick without the tick that re-arms it, PRsOf what the
// model holds for a project and PRFailed whether its last poll failed (#310).
func (m *Model) PollPRs() tea.Cmd                { return m.pollPRs() }
func (m *Model) PRsOf(project string) []forge.PR { return m.prs[project] }
func (m *Model) PRFailed(project string) bool    { return m.prFailed[project] }

// PollIssues is one issue tick without the tick that re-arms it, IssuesOf what
// the model holds for a project and IssuesFailed whether its last poll failed
// (#394).
func (m *Model) PollIssues() tea.Cmd                   { return m.pollIssues() }
func (m *Model) IssuesOf(project string) []forge.Issue { return m.issues[project] }
func (m *Model) IssuesFailed(project string) bool      { return m.issueFailed[project] }

// FitCounts is the header's rule for counts that will not fit beside a name
// (#395), asserted directly because the sidebar is 28 cells and a repository
// with four-digit counts cannot be built out of a fixture.
func FitCounts(parts []string) string { return fitCounts(parts) }

// TrackerRowCount is how many rows the tracker draws, the rule included (#396).
func (m *Model) TrackerRowCount() int { return len(m.trackerRows()) }

// DiffSeq and TurnSeq are the latest load's numbers, so a test that hands in
// a loaded diff of its own stamps it as the answer to that load (#352).
func (m *Model) DiffSeq() uint64 { return m.diffSeq }

// TurnSeq is DiffSeq for the turn diff.
func (m *Model) TurnSeq() uint64 { return m.turnSeq }

// MetaCols is card line two's width for the branch and the diffstat.
func MetaCols() int { return metaCols }

// SpinFrames is one turn of the working spinner, SpinEvery a frame's time on
// screen and SpinFrameAt the frame at a moment (#410). SpinArmed is whether
// a spin tick is pending (#412).
func SpinFrames() []string             { return spinFrames[:] }
func SpinEvery() time.Duration         { return spinEvery }
func SpinFrameAt(now time.Time) string { return spinnerFrame(now) }
func (m *Model) SpinArmed() bool       { return m.spinArmed }

// GeneratedMsgFor is the message the detection command produces, so a test can
// deliver a classification the way Update receives one (#338).
func GeneratedMsgFor(id string, gen map[string]bool) tea.Msg {
	return generatedMsg{id: id, gen: gen}
}
