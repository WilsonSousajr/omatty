package ui_test

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/charmbracelet/x/ansi"
)

// osc52Hello is the sequence a child writes to copy "hello", and the one
// bubbletea must put on the host's terminal for the copy to have happened.
const osc52Hello = "\x1b]52;c;aGVsbG8=\x07"

// spare is queued behind whatever a test asks for, so the wait a handler
// re-arms resolves rather than parking drainCmd on an empty channel. It is
// deliberately distinguishable: a test asserting on it would be asserting on
// the fixture.
var spare = termwrap.ClipboardWrite{Selection: 'c', Text: "queued behind the test's own writes"}

// clipboardModel gives session s1 a fed clipboard stream carrying queued,
// then spare.
func clipboardModel(t *testing.T, queued ...termwrap.ClipboardWrite) *ui.Model {
	t.Helper()
	m, fakes := modelWithFakes(t)
	clips := make(chan termwrap.ClipboardWrite, len(queued)+1)
	for _, w := range append(queued, spare) {
		clips <- w
	}
	fakes["s1"].Clips = clips
	return m
}

// drainCmd runs cmd and returns every message it produced, unwrapping
// batches the way the runtime does. Unlike settle it does not feed them
// back, so a test can assert on what was scheduled rather than on the model
// that consumed it.
func drainCmd(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case nil:
		return nil
	case tea.BatchMsg:
		out := make([]tea.Msg, 0, len(msg))
		for _, c := range msg {
			out = append(out, drainCmd(c)...)
		}
		return out
	default:
		return []tea.Msg{msg}
	}
}

// The other half of #212: a write lifted out of a pane's output is put on the
// host's clipboard. bubbletea owns the write, so the model's job is done when
// it has produced the command SetClipboard produces - nothing here touches
// stdout, which belongs to the TUI (invariant 5).
func TestModel_osc52FromAPaneReachesTheHost_issue212(t *testing.T) {
	m := clipboardModel(t)

	_, cmd := m.Update(ui.ClipboardMsg{
		SessionID: "s1",
		Write:     termwrap.ClipboardWrite{Selection: 'c', Text: "hello"},
	})

	want := tea.SetClipboard("hello")()
	if !hasMsg(drainCmd(cmd), want) {
		t.Errorf("the copy did not reach the host: scheduled %+v, want %+v", drainCmd(cmd), want)
	}
}

// The round trip the issue asks for, spelled out: the bytes the child wrote
// are the bytes the host is given back. Decoding and re-encoding is not a
// detour, it is how the payload crosses a boundary that speaks text.
func TestModel_osc52ArrivesAtTheHostAsTheSameSequence_issue212(t *testing.T) {
	if got := ansi.SetSystemClipboard("hello"); got != osc52Hello {
		t.Errorf("the host would be sent %q, want the sequence the child wrote, %q", got, osc52Hello)
	}
}

// OSC 52's 'p' is the primary selection, a different clipboard from 'c'.
func TestModel_osc52PrimarySelectionIsNotTheSystemOne_issue212(t *testing.T) {
	m := clipboardModel(t)

	_, cmd := m.Update(ui.ClipboardMsg{
		SessionID: "s1",
		Write:     termwrap.ClipboardWrite{Selection: 'p', Text: "hello"},
	})

	msgs := drainCmd(cmd)
	if !hasMsg(msgs, tea.SetPrimaryClipboard("hello")()) {
		t.Errorf("a primary-selection copy scheduled %+v", msgs)
	}
	if hasMsg(msgs, tea.SetClipboard("hello")()) {
		t.Errorf("a primary-selection copy also wrote the system clipboard: %+v", msgs)
	}
}

// A pane copies more than once, so handling one write must leave the next
// one waited on. Without the re-arm the first copy of a session would be the
// only one that ever reached the host.
func TestModel_osc52ReArmsTheWait_issue212(t *testing.T) {
	m := clipboardModel(t, termwrap.ClipboardWrite{Selection: 'c', Text: "second"})

	_, cmd := m.Update(ui.ClipboardMsg{
		SessionID: "s1",
		Write:     termwrap.ClipboardWrite{Selection: 'c', Text: "first"},
	})

	want := ui.ClipboardMsg{
		SessionID: "s1",
		Write:     termwrap.ClipboardWrite{Selection: 'c', Text: "second"},
	}
	if !hasMsg(drainCmd(cmd), want) {
		t.Errorf("the wait was not re-armed: scheduled %+v", drainCmd(cmd))
	}
}

// A terminal reporting no clipboard stream is not waited on at all: a nil
// channel would park a goroutine that nothing can ever wake.
func TestModel_osc52IsNotArmedWithoutAStream_issue212(t *testing.T) {
	m, _ := modelWithFakes(t)

	if cmd := m.WaitForClipboard("s1"); cmd != nil {
		t.Error("a terminal with no clipboard stream was waited on")
	}
	if cmd := m.WaitForClipboard("nosuchsession"); cmd != nil {
		t.Error("a session with no terminal was waited on")
	}
}

// An armed wait delivers the pane's copy as a ClipboardMsg naming the
// session it came from, so the re-arm knows which stream to go back to.
func TestModel_osc52NamesTheSessionItCameFrom_issue212(t *testing.T) {
	m := clipboardModel(t, termwrap.ClipboardWrite{Selection: 'c', Text: "hello"})

	got := drainCmd(m.WaitForClipboard("s1"))

	want := ui.ClipboardMsg{
		SessionID: "s1",
		Write:     termwrap.ClipboardWrite{Selection: 'c', Text: "hello"},
	}
	if !hasMsg(got, want) {
		t.Errorf("the wait delivered %+v, want %+v", got, want)
	}
}

func hasMsg(msgs []tea.Msg, want tea.Msg) bool {
	for _, got := range msgs {
		if reflect.DeepEqual(got, want) {
			return true
		}
	}
	return false
}
