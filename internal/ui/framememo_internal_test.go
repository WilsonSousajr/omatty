package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// mutablePane is a Fake whose rendered grid a test can change between
// frames, standing in for an emulator that has just read from its PTY. The
// Fake's own view is fixed at construction, and the whole point here is
// content that moves without a message reaching the model.
type mutablePane struct {
	*termwrap.Fake
	view string
}

func (p *mutablePane) View() string { return p.view }

// unknownMsg is a message no table handles, so it falls all the way through
// to the broadcast - the path emulator traffic takes.
type unknownMsg struct{}

func memoModel() (*Model, *mutablePane) {
	pane := &mutablePane{Fake: termwrap.NewFake(""), view: "pane one\npane two"}
	st := registry.State{
		Projects: []registry.Project{{Name: "p", Root: "/tmp/p"}},
		Sessions: []registry.Session{
			{ID: "s1", Project: "p", Title: "first session", Dir: "/tmp/p"},
			{ID: "s2", Project: "p", Title: "second session", Dir: "/tmp/p"},
		},
	}
	m := NewModel(Deps{State: st, Terms: map[string]termwrap.Terminal{
		"s1": pane, "s2": termwrap.NewFake("the other pane"),
	}})
	m.width, m.height = 120, 40
	return m, pane
}

// freshFrame is the frame with the memo bypassed, which is what the memo is
// measured against.
func freshFrame(m *Model) string {
	m.frameMemo.valid = false
	return m.frame()
}

// The negative control for the memo. Whatever the message, the frame served
// afterwards must equal the frame that would have been built: a missed
// invalidation is a screen showing something that is no longer true, which
// is a worse bug than the cost the memo saves.
//
// A message type added later that changes the frame and is not covered here
// is exactly what this cannot catch, which is why the memo is invalidated by
// default and kept only on the one path proven not to touch model state.
func TestFrame_MemoMatchesAFreshFrameAfterEveryMessage(t *testing.T) {
	for _, tt := range []struct {
		name string
		msg  tea.Msg
	}{
		{"tick", TickMsg(time.Unix(1, 0))},
		{"stat tick", StatTickMsg(time.Unix(1, 0))},
		{"status", StatusMsg(watcher.Event{SessionID: "s1", Kind: watcher.PermissionRequested, At: time.Unix(2, 0)})},
		{"repo stat", RepoStatMsg{SessionID: "s1", Stat: review.Stat{Branch: "topic", Added: 9, Removed: 4}}},
		{"diff loaded", DiffLoadedMsg{SessionID: "s1"}},
		{"files loaded", FilesLoadedMsg{SessionID: "s1", Paths: []string{"a.go"}}},
		{"named", NamedMsg{SessionID: "s1", Title: "a new title"}},
		{"model named", ModelNamedMsg{SessionID: "s1", Title: "a better title"}},
		{"branch named", BranchNamedMsg{SessionID: "s1", Branch: "topic"}},
		{"projects proposed", ProjectsProposedMsg{}},
		{"sessions proposed", SessionsProposedMsg{}},
		{"worktree removed", WorktreeRemovedMsg{SessionID: "s1", Dir: "/tmp/p"}},
		{"clipboard", ClipboardMsg{SessionID: "s1"}},
		{"window size", tea.WindowSizeMsg{Width: 100, Height: 30}},
		{"leader key", tea.KeyPressMsg{Code: 'o', Mod: tea.ModCtrl}},
		{"focus", tea.FocusMsg{}},
		{"blur", tea.BlurMsg{}},
		{"emulator traffic", unknownMsg{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := memoModel()
			m.frame() // prime the memo, as a real run always has

			m.Update(tt.msg)

			got := m.frame()
			if want := freshFrame(m); got != want {
				t.Errorf("after %s the memo served a frame a rebuild does not agree with:\ngot  %q\nwant %q",
					tt.name, got, want)
			}
		})
	}
}

// The case the memo exists for. Emulator traffic mutates the terminal and
// never the model, so the frame survives it - proven by changing model state
// behind Update's back, which a served memo cannot see and a rebuild would.
func TestFrame_EmulatorTrafficKeepsTheMemo(t *testing.T) {
	m, _ := memoModel()
	before := m.frame()

	m.Update(unknownMsg{})
	m.status["s1"] = watcher.SessionState{Status: watcher.StatusWaiting}

	if got := m.frame(); got != before {
		t.Error("emulator traffic dropped the memo; the frame was rebuilt although nothing on it had changed")
	}
}

// The other half: the one thing emulator traffic can change is the focused
// pane's content, and that is part of the memo's key.
func TestFrame_NewPaneContentRebuildsTheFrame(t *testing.T) {
	m, pane := memoModel()
	before := m.frame()

	pane.view = "pane one\npane CHANGED"
	m.Update(unknownMsg{})

	if got := m.frame(); got == before {
		t.Error("the focused pane's new content never reached the frame")
	}
}
