package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// turn drives s1 through one turn: a prompt, then the end of the turn. Each
// event is delivered the way the runtime would, so the relist that a turn end
// schedules actually runs.
func turn(m *ui.Model, at time.Time) {
	_, cmd := m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PromptSubmitted, At: at})
	deliver(m, cmd)
	_, cmd = m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.TurnEnded, At: at.Add(time.Second)})
	deliver(m, cmd)
}

// A file claude created during the turn is on screen when the turn ends,
// without the operator pressing r (#195).
func TestModel_ANewFileAppearsWhenTheTurnEnds_issue195(t *testing.T) {
	m, _, lister, _ := modelWithTree(t)
	leader(m, key('f'))
	lister.Paths = append(lister.Paths, "b.go")

	turn(m, time.Now())

	if len(lister.Asked) != 2 {
		t.Fatalf("listed %d times, want the opening listing and one for the turn end", len(lister.Asked))
	}
	lineWith(t, m.View().Content, "b.go")
}

func TestModel_AFoldedDirectoryStaysFoldedAcrossARelist_issue195(t *testing.T) {
	m, _, lister, _ := modelWithTree(t)
	leader(m, key('f'))
	press(m, special(tea.KeyEnter)) // fold internal/, the first row (#194)
	lister.Paths = append(lister.Paths, "internal/ui/b.go")

	turn(m, time.Now())

	view := m.View().Content
	if strings.Contains(view, "b.go") || strings.Contains(view, "model.go") {
		t.Errorf("internal/ sprang open on the relist:\n%s", view)
	}
	lineWith(t, view, "▸ internal/")
}

// The cursor follows its path, not its index: a file listed above it moves
// the row down and the cursor with it.
func TestModel_TheCursorStaysOnItsPathAcrossARelist_issue195(t *testing.T) {
	m, _, lister, reader := modelWithTree(t)
	leader(m, key('f'))
	foldToGoMod(m)
	lister.Paths = append(lister.Paths, "cmd/main.go") // sorts above go.mod

	turn(m, time.Now())

	pressAndSettle(m, special(tea.KeyEnter))
	if m.ReviewView() != ui.ViewPreview || len(reader.Read) != 1 || reader.Read[0] != "go.mod" {
		t.Errorf("view=%v read=%v after the relist; want the cursor still on go.mod",
			m.ReviewView(), reader.Read)
	}
}

// A closed column costs no git fork on a turn end (#124); the reopen pays
// for both the diff and the listing instead.
func TestModel_AClosedColumnRelistsOnReopenNotOnTheTurnEnd_issue195(t *testing.T) {
	m, _, lister, _ := modelWithTree(t)
	leader(m, key('f'))
	press(m, special(tea.KeyEscape))
	leader(m, key('f')) // closes it (#124)
	lister.Paths = append(lister.Paths, "b.go")

	turn(m, time.Now())
	if len(lister.Asked) != 1 {
		t.Fatalf("listed %d times with the column closed, want 1", len(lister.Asked))
	}

	leader(m, key('f'))
	if len(lister.Asked) != 2 {
		t.Fatalf("listed %d times after the reopen, want 2", len(lister.Asked))
	}
	lineWith(t, m.View().Content, "b.go")
}

// The tree is loaded but the diff is showing: the turn end still refreshes
// the listing, so switching back to the tree shows the new file.
func TestModel_ARelistBehindTheDiffViewStillLands_issue195(t *testing.T) {
	m, _, lister, _ := modelWithTree(t)
	leader(m, key('f'))
	leader(m, key('d'))
	lister.Paths = append(lister.Paths, "b.go")

	turn(m, time.Now())

	leader(m, key('f'))
	if len(lister.Asked) != 2 {
		t.Fatalf("listed %d times, want 2: one to open, one for the turn end", len(lister.Asked))
	}
	lineWith(t, m.View().Content, "b.go")
}

// Two turn ends before the first listing returns fork git once, the way
// pollStat guards its own in-flight read.
func TestModel_ASecondTurnEndWaitsForTheListingInFlight_issue195(t *testing.T) {
	m, _, lister, _ := modelWithTree(t)
	leader(m, key('f'))
	now := time.Now()

	_, first := m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.TurnEnded, At: now})
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PromptSubmitted, At: now.Add(time.Second)})
	_, second := m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.TurnEnded, At: now.Add(2 * time.Second)})
	deliver(m, first)
	deliver(m, second)

	if len(lister.Asked) != 2 {
		t.Errorf("listed %d times, want 2: the opening listing and one relist for both turn ends",
			len(lister.Asked))
	}
	turn(m, now.Add(3*time.Second))
	if len(lister.Asked) != 3 {
		t.Errorf("listed %d times after a later turn, want 3: the guard must clear once the listing lands",
			len(lister.Asked))
	}
}
