package ui_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// recordRebind is a named fake for Deps.Rebind and Deps.TailStart: what the
// model persisted, and which sessions it re-tailed.
type recordRebind struct {
	SessionID, Conversation string
	Calls                   int
	Err                     error
	Tailed                  []registry.Session
}

func (r *recordRebind) rebind(sessionID, conversation string) error {
	r.Calls++
	r.SessionID, r.Conversation = sessionID, conversation
	return r.Err
}

func (r *recordRebind) tail(sess registry.Session) { r.Tailed = append(r.Tailed, sess) }

func modelWithRebind(t *testing.T, r *recordRebind) *ui.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Rebind, d.TailStart = r.rebind, r.tail
	d.Clock = func() time.Time { return fixedNow }
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	return m
}

// cleared is the SessionStart a /clear in s1's pane produces: claude's new
// conversation id, and the pane's registry id from the hook's environment.
func cleared(owner string) ui.StatusMsg {
	return ui.StatusMsg{SessionID: "after-clear", Owner: owner, Kind: watcher.SessionRebound, At: fixedNow}
}

// Regression, issue #316: a /clear left the row on the pre-clear uuid, so
// the tailer watched a dead file, the new conversation's hooks were dropped,
// and the next start resumed the old conversation. The row is re-bound on
// disk and in memory, and its tailer follows.
func TestModel_ClearRebindsTheRowAndTheTailer_issue316(t *testing.T) {
	r := &recordRebind{}
	m := modelWithRebind(t, r)

	m.Update(cleared("s1"))

	if r.Calls != 1 || r.SessionID != "s1" || r.Conversation != "after-clear" {
		t.Errorf("Rebind got %d calls, last (%q, %q), want one (s1, after-clear)", r.Calls, r.SessionID, r.Conversation)
	}
	if len(r.Tailed) != 1 || r.Tailed[0].ID != "s1" || r.Tailed[0].ConversationID() != "after-clear" {
		t.Errorf("TailStart got %+v, want s1 on after-clear", r.Tailed)
	}
}

// The status half of the same bug: after the clear, every hook arrives under
// the new conversation id, and it must reach the row it belongs to.
func TestModel_StatusUnderTheNewConversationReachesTheRow_issue316(t *testing.T) {
	m := modelWithRebind(t, &recordRebind{})
	m.Update(cleared("s1"))

	m.Update(ui.StatusMsg{SessionID: "after-clear", Kind: watcher.PermissionRequested, At: fixedNow.Add(time.Second)})

	if got := rowOf(t, m, "main"); !strings.Contains(got, "●") {
		t.Errorf("a hook under the post-clear id did not reach s1's row: %q", got)
	}
}

// A payload whose owner is no row of ours - a stale pane, a claude from an
// older omatty - changes nothing.
func TestModel_ReboundForAnUnknownOwnerChangesNothing_issue316(t *testing.T) {
	r := &recordRebind{}
	m := modelWithRebind(t, r)

	m.Update(cleared("nope"))

	if r.Calls != 0 || len(r.Tailed) != 0 {
		t.Errorf("an unknown owner re-bound something: %d rebinds, tails %+v", r.Calls, r.Tailed)
	}
}

// The trap #316 names: a desktop Claude Code forked from s1's pane carries
// s1's environment and starts under its own new id with source resume. That
// is a plain SessionStarted, and s1 must stay on its own conversation.
func TestModel_ForkedSessionStartDoesNotRebind_issue316(t *testing.T) {
	r := &recordRebind{}
	m := modelWithRebind(t, r)

	m.Update(ui.StatusMsg{SessionID: "fork", Owner: "s1", Kind: watcher.SessionStarted, At: fixedNow})

	if r.Calls != 0 || len(r.Tailed) != 0 {
		t.Errorf("a forked session re-bound s1: %d rebinds, tails %+v", r.Calls, r.Tailed)
	}
}

// A rebind that cannot be saved is said, and leaves the row where state.json
// has it, so memory and disk never disagree about what to resume.
func TestModel_RebindFailureSurfacesAndKeepsTheRow_issue316(t *testing.T) {
	r := &recordRebind{Err: errors.New("state.json is read-only")}
	m := modelWithRebind(t, r)

	m.Update(cleared("s1"))

	if got := m.View().Content; !strings.Contains(got, "read-only") {
		t.Errorf("View() does not surface the failed rebind:\n%s", got)
	}
	if len(r.Tailed) != 0 {
		t.Errorf("TailStart ran after a failed rebind: %+v", r.Tailed)
	}
}

// Unwired, the default says so in the footer rather than appearing to follow
// the clear: the pane would otherwise show a conversation state.json does not
// know, and the next start would resume the old one anyway (#316).
func TestModel_UnwiredRebindNamesTheMissingWiring_issue316(t *testing.T) {
	terms, _ := fakeTerms(t)
	m := ui.NewModel(baseDeps(twoProjectState(), terms))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})

	m.Update(cleared("s1"))

	if got := m.View().Content; !strings.Contains(got, "no rebind source") {
		t.Errorf("View() does not name the missing rebind wiring:\n%s", got)
	}
}
