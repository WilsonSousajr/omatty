package ui_test

import (
	"regexp"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// columnFace opens one face of the review column on a model wide enough that
// its title keeps the position (#424 drops it first when room runs out).
type columnFace struct {
	name string
	open func(t *testing.T) *ui.Model
}

var cursorFaces = []columnFace{
	{"diff", func(t *testing.T) *ui.Model {
		m, _, _ := modelWithDiff(t)
		m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
		leader(m, key('d'))
		return m
	}},
	{"tree", func(t *testing.T) *ui.Model {
		m, _, _, _ := modelWithTree(t)
		m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
		leader(m, key('f'))
		return m
	}},
	{"gate", func(t *testing.T) *ui.Model {
		m, _, _ := modelWithDiff(t)
		m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
		m.SetGateReport("s1", gateReport(gate.Pass, gate.Pass, gate.Fail, gate.Pending))
		leader(m, key('g'))
		return m
	}},
	{"tracker", func(t *testing.T) *ui.Model {
		m, _, _ := trackerModel(t)
		m.Update(tea.WindowSizeMsg{Width: 200, Height: 30})
		openTracker(m)
		return m
	}},
}

var positionRE = regexp.MustCompile(`(\d+)/(\d+)`)

// columnTitle is the review column's part of the header row, unstyled.
func columnTitle(m *ui.Model) string {
	head := stripSGR(frameLines(m)[0])
	return head[strings.LastIndex(head, "│")+len("│"):]
}

// titlePosition reads the "N/M" the column's title carries (#424).
func titlePosition(t *testing.T, m *ui.Model) (n, total int) {
	t.Helper()
	got := positionRE.FindStringSubmatch(columnTitle(m))
	if got == nil {
		t.Fatalf("the column title carries no N/M position: %q", columnTitle(m))
	}
	n, _ = strconv.Atoi(got[1])
	total, _ = strconv.Atoi(got[2])
	return n, total
}

// cursorRow is the review column's part of the reverse-video row, unstyled, or
// "" when no row is drawn reversed: reverse video is the cursor on every face.
func cursorRow(m *ui.Model) string {
	for _, line := range frameLines(m) {
		if strings.Contains(line, "\x1b[7m") {
			plain := stripSGR(line)
			return plain[strings.LastIndex(plain, "│")+len("│"):]
		}
	}
	return ""
}

// pressUntilCursor presses k up to limit times and reports whether the cursor
// row came to hold want - before the first press or after any of them.
func pressUntilCursor(m *ui.Model, k tea.KeyPressMsg, limit int, want string) bool {
	for range limit {
		if strings.Contains(cursorRow(m), want) {
			return true
		}
		press(m, k)
	}
	return strings.Contains(cursorRow(m), want)
}

// G and g reach the last row and the first on every cursor face, and the title
// says where the cursor is.
func TestColumn_GAndgReachTheEndsOnEveryFace_issue424(t *testing.T) {
	for _, face := range cursorFaces {
		m := face.open(t)
		press(m, key('G'))
		if n, total := titlePosition(t, m); n != total || total < 2 {
			t.Errorf("%s: after G the title says %d/%d, want the last of several", face.name, n, total)
		}
		press(m, key('g'))
		if n, _ := titlePosition(t, m); n != 1 {
			t.Errorf("%s: after g the title says row %d, want 1", face.name, n)
		}
	}
}

// ctrl+d and ctrl+u move half a page, the pager keys every terminal reader knows.
func TestColumn_CtrlDAndCtrlUMoveHalfAPage_issue424(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	// 21 rows, so half is 10; 120 wide so the title keeps its position beside
	// the run's counts (#428).
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	m.SetGateReport("s1", longGate(40))
	leader(m, key('g'))

	press(m, ctrl('d'))
	if n, _ := titlePosition(t, m); n != 11 {
		t.Errorf("after ctrl+d the gate is on step %d, want 11", n)
	}
	press(m, ctrl('u'))
	if n, _ := titlePosition(t, m); n != 1 {
		t.Errorf("after ctrl+u the gate is on step %d, want 1", n)
	}
}

// The cursor is reverse video on every face: the gate drew "▸ " and the tracker
// an accent colour, so the same thing looked different on each face.
func TestColumn_TheCursorIsReverseVideoOnEveryFace_issue424(t *testing.T) {
	for _, face := range cursorFaces {
		m := face.open(t)
		if cursorRow(m) == "" {
			t.Errorf("%s: no row is drawn in reverse video:\n%s", face.name, m.View().Content)
		}
	}
	m := cursorFaces[2].open(t)
	if strings.Contains(stripSGR(m.View().Content), "▸ ✓") {
		t.Errorf("the gate still marks its cursor with ▸:\n%s", m.View().Content)
	}
}

// The pick list's cursor is reverse video too, not "» ".
func TestPickList_TheCursorIsReverseVideo_issue424(t *testing.T) {
	m, _ := modelWithFakes(t)
	leader(m, key('/'))
	body := m.View().Content
	if !strings.Contains(body, "\x1b[7m") || strings.Contains(body, "» ") {
		t.Errorf("the pick list's cursor is not reverse video:\n%s", body)
	}
}

// A click selects a row on the gate and the tracker, as it has on the diff and
// the tree since #168.
func TestColumn_AClickSelectsOnTheGateAndTheTracker_issue424(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	m.SetGateReport("s1", gateReport(gate.Pass, gate.Pass, gate.Fail, gate.Pending))
	leader(m, key('g'))
	m.Update(clickAt(reviewHairlineX+5, reviewRowY(2)))
	if row := cursorRow(m); !strings.Contains(row, "test") {
		t.Errorf("a click on the third step left the cursor on %q", row)
	}

	m, _, _ = trackerModel(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	openTracker(m)
	m.Update(clickAt(reviewHairlineX+5, reviewRowY(1)))
	if got := m.TrackerCursor(); got != 1 {
		t.Errorf("a click on the second tracker row left the cursor at %d", got)
	}
}

// longGate is n passing steps named s00, s01, ...
func longGate(n int) gate.Report {
	results := make([]gate.StepResult, n)
	for i := range results {
		name := "s" + strconv.Itoa(i/10) + strconv.Itoa(i%10)
		results[i] = gate.StepResult{Step: gate.Step{Name: name, Run: "true"}, Verdict: gate.Pass}
	}
	return gate.Report{ID: "s1", Results: results}
}
