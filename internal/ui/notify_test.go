package ui_test

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/notify"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// modelWithNotifier returns a model whose clock the test can move, with no
// event channel so the command Update returns is the notification alone.
func modelWithNotifier(t *testing.T) (*ui.Model, *notify.Fake, *time.Time) {
	t.Helper()
	terms, _ := fakeTerms(t)
	now := fixedNow
	n := &notify.Fake{}
	d := baseDeps(twoProjectState(), terms)
	d.Clock = func() time.Time { return now }
	d.Notifier = n
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	return m, n, &now
}

// runCmd executes a command tree synchronously, so a notification posted as a
// tea.Cmd lands in the fake before the assertion (issue #69).
func runCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	if batch, ok := cmd().(tea.BatchMsg); ok {
		for _, c := range batch {
			runCmd(c)
		}
	}
}

func status(m *ui.Model, id string, kind watcher.Kind, at time.Time) {
	_, cmd := m.Update(ui.StatusMsg{SessionID: id, Kind: kind, At: at})
	runCmd(cmd)
}

func TestModel_NotifiesWhenAWaitingSessionArrivesWhileBlurred_issue38(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "s1", watcher.PermissionRequested, fixedNow)

	if len(n.Sent) != 1 {
		t.Fatalf("sent %d notifications, want 1 for a waiting session while blurred", len(n.Sent))
	}
	if got := n.Sent[0].Body; got == "" {
		t.Error("the notification body is empty")
	}
}

func TestModel_DoesNotNotifyWhileFocused_issue38(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.FocusMsg{})

	status(m, "s1", watcher.PermissionRequested, fixedNow)

	if len(n.Sent) != 0 {
		t.Errorf("sent %d notifications while focused, want 0", len(n.Sent))
	}
}

func TestModel_DoesNotNotifyTwiceForTheSameState_issue38(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "s1", watcher.PermissionRequested, fixedNow)
	status(m, "s1", watcher.PermissionRequested, fixedNow.Add(time.Second))

	if len(n.Sent) != 1 {
		t.Errorf("sent %d notifications, want 1: a repeated waiting state must not re-notify", len(n.Sent))
	}
}

func TestModel_DoesNotNotifyForThinkingOrTool_issue38(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "s1", watcher.PromptSubmitted, fixedNow)
	status(m, "s1", watcher.ToolStarted, fixedNow.Add(time.Second))

	if len(n.Sent) != 0 {
		t.Errorf("sent %d notifications for thinking/tool, want 0 (those do not need you)", len(n.Sent))
	}
}

func TestModel_NotifiesWhenADoneSessionArrivesWhileBlurred_issue38(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "s1", watcher.TurnEnded, fixedNow)

	if len(n.Sent) != 1 {
		t.Errorf("sent %d notifications for a finished turn while blurred, want 1", len(n.Sent))
	}
}

// Regression, issue #69: the notifier ran inline in Update, freezing every
// pane for the ~40 ms osascript takes. It must come back as a command.
func TestModel_NotificationIsACommandNotAnInlineCall_issue69(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	_, cmd := m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: fixedNow})

	if len(n.Sent) != 0 {
		t.Fatal("Notify was called inside Update; it must run as a command off the event loop")
	}
	runCmd(cmd)
	if len(n.Sent) != 1 {
		t.Errorf("running the returned command sent %d notifications, want 1", len(n.Sent))
	}
}

// Regression, issue #69: any session id on the socket grew the status map and
// reached the notification body. Unregistered ids are dropped.
func TestModel_IgnoresAStatusEventForAnUnknownSession_issue69(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "not-registered", watcher.PermissionRequested, fixedNow)

	if len(n.Sent) != 0 {
		t.Errorf("an unregistered session id produced %+v, want nothing", n.Sent)
	}
}

// Regression, issue #69: a permission loop notified on every transition.
func TestModel_RateLimitsNotificationsPerSession_issue69(t *testing.T) {
	m, n, now := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "s1", watcher.PermissionRequested, fixedNow)
	status(m, "s1", watcher.TurnEnded, fixedNow.Add(time.Second))
	if len(n.Sent) != 1 {
		t.Fatalf("sent %d within the cooldown, want 1", len(n.Sent))
	}

	*now = fixedNow.Add(10 * time.Second)
	status(m, "s1", watcher.PermissionRequested, *now)
	if len(n.Sent) != 2 {
		t.Errorf("sent %d after the cooldown elapsed, want 2", len(n.Sent))
	}
}

// Regression, issue #70: the first tailer poll replays the transcript, and a
// "finished" for a turn that ended days ago fired as soon as omatty was
// backgrounded.
func TestModel_DoesNotNotifyForATurnThatEndedBeforeStart_issue70(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "s1", watcher.TurnEnded, fixedNow.Add(-168*time.Hour))
	if len(n.Sent) != 0 {
		t.Fatalf("notified about a turn that ended a week before this run: %+v", n.Sent)
	}

	status(m, "s1", watcher.PermissionRequested, fixedNow.Add(time.Second))
	if len(n.Sent) != 1 {
		t.Errorf("a transition after start sent %d notifications, want 1", len(n.Sent))
	}
}
