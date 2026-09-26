package ui_test

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// The rule under the header names the face on show, not "review" on all of
// them: with five faces the rule is where a glance says which one this is.
func TestColumn_TheRuleNamesTheFace_issue426(t *testing.T) {
	want := map[string]string{"diff": "diff", "tree": "files", "gate": "gate", "tracker": "tracker"}
	for _, face := range cursorFaces {
		m := face.open(t)
		rule := stripSGR(frameLines(m)[1])
		if !strings.Contains(rule, "─ "+want[face.name]+" ─") || strings.Contains(rule, "review") {
			t.Errorf("%s: the rule does not name the face %q:\n%s", face.name, want[face.name], rule)
		}
	}
}

// The gate's title had no budget and was cut at the column edge mid-name; it
// shortens the session name the way the tree's title does (#285).
func TestGate_TitleShortensTheSessionName_issue426(t *testing.T) {
	m := modelWithNamedTree(t, "a-very-long-session-name-that-cannot-fit-here")
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.SetGateReport("s1", gateReport(gate.Pass))
	leader(m, key('g'))

	title := strings.TrimSpace(columnTitle(m))
	if !strings.HasPrefix(title, "gate · ") || !strings.Contains(title, "…") || !strings.HasSuffix(title, "here") {
		t.Errorf("the gate title is not the name shortened in its middle: %q", title)
	}
}

// facts is the footer's right side: counts, not keys.
var facts = regexp.MustCompile(`^\d+ (session|sessions|waiting)`)

// On a narrow window each face's footer gives up whole entries, lowest first,
// and never the help key: a key cut in half reads as a different key, and a cut
// help key leaves the rest of the keymap unreachable (#103).
func TestFooter_GivesUpWholeEntriesAndNeverTheHelpKey_issue426(t *testing.T) {
	footers := ui.Footers(ui.DefaultLeader)
	byFace := map[string]string{"diff": "reviewFooter", "tree": "treeFooter", "gate": "gateFooter", "tracker": "trackerFooter"}
	for _, face := range cursorFaces {
		m := face.open(t)
		m.Update(tea.WindowSizeMsg{Width: 50, Height: 24})
		full := footers[byFace[face.name]]
		foot := strings.TrimSpace(stripSGR(frameLines(m)[len(frameLines(m))-1]))
		if lipgloss.Width(full) <= 50 {
			t.Fatalf("%s: the fixture's footer already fits 50 columns: %q", face.name, full)
		}
		for _, entry := range regexp.MustCompile(`\s{2,}`).Split(foot, -1) {
			if !facts.MatchString(entry) && !strings.Contains("  "+full+"  ", "  "+entry+"  ") {
				t.Errorf("%s: footer entry %q is not a whole entry of %q", face.name, entry, full)
			}
		}
		if !strings.Contains(foot, ui.DefaultLeader+" ? keys") {
			t.Errorf("%s: the narrow footer lost the help key: %q", face.name, foot)
		}
	}
}
