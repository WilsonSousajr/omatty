package ui_test

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/notify"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func modelWithNotifier(t *testing.T) (*ui.Model, chan watcher.Event, *notify.Fake) {
	t.Helper()
	m, events, _ := modelWithEvents(t)
	n := &notify.Fake{}
	m.SetNotifier(n)
	return m, events, n
}

func TestModel_NotifiesWhenAWaitingSessionArrivesWhileBlurred_issue38(t *testing.T) {
	m, _, n := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: fixedNow})

	if len(n.Sent) != 1 {
		t.Fatalf("sent %d notifications, want 1 for a waiting session while blurred", len(n.Sent))
	}
	if got := n.Sent[0].Body; got == "" {
		t.Error("the notification body is empty")
	}
}

func TestModel_DoesNotNotifyWhileFocused_issue38(t *testing.T) {
	m, _, n := modelWithNotifier(t)
	m.Update(tea.FocusMsg{})

	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: fixedNow})

	if len(n.Sent) != 0 {
		t.Errorf("sent %d notifications while focused, want 0", len(n.Sent))
	}
}

func TestModel_DoesNotNotifyTwiceForTheSameState_issue38(t *testing.T) {
	m, _, n := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: fixedNow})
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: fixedNow.Add(time.Second)})

	if len(n.Sent) != 1 {
		t.Errorf("sent %d notifications, want 1: a repeated waiting state must not re-notify", len(n.Sent))
	}
}

func TestModel_DoesNotNotifyForThinkingOrTool_issue38(t *testing.T) {
	m, _, n := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PromptSubmitted, At: fixedNow})
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.ToolStarted, At: fixedNow.Add(time.Second)})

	if len(n.Sent) != 0 {
		t.Errorf("sent %d notifications for thinking/tool, want 0 (those do not need you)", len(n.Sent))
	}
}

func TestModel_NotifiesWhenADoneSessionArrivesWhileBlurred_issue38(t *testing.T) {
	m, _, n := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.TurnEnded, At: fixedNow})

	if len(n.Sent) != 1 {
		t.Errorf("sent %d notifications for a finished turn while blurred, want 1", len(n.Sent))
	}
}
