package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

var fixedNow = time.Date(2026, 9, 2, 12, 0, 0, 0, time.UTC)

func modelWithEvents(t *testing.T) (*ui.Model, chan watcher.Event, map[string]*termwrap.Fake) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	events := make(chan watcher.Event, 8)
	d := baseDeps(twoProjectState(), terms)
	d.Events = events
	d.Clock = func() time.Time { return fixedNow }
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	return m, events, fakes
}

// rowOf returns the rendered sidebar line naming title.
func rowOf(t *testing.T, m *ui.Model, title string) string {
	t.Helper()
	for _, line := range strings.Split(m.View().Content, "\n") {
		if strings.Contains(line, title) {
			return line
		}
	}
	t.Fatalf("no row for %q:\n%s", title, m.View().Content)
	return ""
}

func TestModel_StatusMsgUpdatesTheGlyph_issue20(t *testing.T) {
	m, _, _ := modelWithEvents(t)

	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: fixedNow})

	// s1 is titled "main"; the waiting glyph "!" must sit on its row.
	if got := rowOf(t, m, "main"); !strings.Contains(got, "!") {
		t.Errorf("the waiting glyph is not shown after a PermissionRequested event: %q", got)
	}
}

func TestModel_StatusMsgReArmsTheWait_issue20(t *testing.T) {
	m, _, _ := modelWithEvents(t)

	_, cmd := m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.ToolStarted, At: fixedNow})

	if cmd == nil {
		t.Error("onStatus did not re-arm the wait; only one event would ever be read")
	}
}

func TestModel_OlderStatusMsgIsIgnored_issue20(t *testing.T) {
	m, _, _ := modelWithEvents(t)
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: fixedNow})

	// A stale "thinking" from before must not overwrite the fresh "waiting".
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PromptSubmitted, At: fixedNow.Add(-time.Minute)})

	if got := rowOf(t, m, "main"); !strings.Contains(got, "!") || strings.Contains(got, "*") {
		t.Errorf("an older event overwrote the newer waiting status: %q", got)
	}
}

func TestSidebarRows_ShowsAgeFromStatus_issue37(t *testing.T) {
	m, _, _ := modelWithEvents(t)

	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PromptSubmitted, At: fixedNow.Add(-4 * time.Minute)})

	if !strings.Contains(m.View().Content, "4m") {
		t.Errorf("the sidebar does not show the 4m age:\n%s", m.View().Content)
	}
}

func TestModel_HeaderShowsTokens_issue39(t *testing.T) {
	m, _, _ := modelWithEvents(t)

	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.UsageUpdated, At: fixedNow,
		Tokens: watcher.Tokens{In: 12345, Out: 3100}})

	got := m.View().Content
	if !strings.Contains(got, "12.3k in") || !strings.Contains(got, "3.1k out") {
		t.Errorf("the header does not show abbreviated tokens:\n%s", got)
	}
}

// Regression, issue #71: the age was computed at render time, but nothing
// triggered a render on a quiet session, so "<1m" stayed on screen for hours.
func TestModel_TickReArmsItself_issue71(t *testing.T) {
	m, _ := modelWithFakes(t)

	_, cmd := m.Update(ui.TickMsg(fixedNow))

	if cmd == nil {
		t.Error("a tick returned no command; the age column would freeze after the first second")
	}
}

func TestModel_InitSchedulesATick_issue71(t *testing.T) {
	m := ui.NewModel(baseDeps(emptyState(), map[string]termwrap.Terminal{}))

	if m.Init() == nil {
		t.Error("Init scheduled nothing with no terminals; the tick must be there regardless")
	}
}

// A model built with only the required dependencies must still handle a
// status event while blurred: the clock and notifier default (issue #76).
func TestNewModel_DefaultsTheOptionalDeps_issue76(t *testing.T) {
	terms, _ := fakeTerms(t)
	m := ui.NewModel(baseDeps(twoProjectState(), terms))
	m.Update(tea.BlurMsg{})

	_, cmd := m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: time.Now().Add(time.Second)})

	runCmd(cmd) // the silent notifier must not panic
	if got := rowOf(t, m, "main"); !strings.Contains(got, "!") {
		t.Errorf("status was not applied with default deps: %q", got)
	}
}
