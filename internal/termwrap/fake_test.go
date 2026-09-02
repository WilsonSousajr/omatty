package termwrap_test

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// Compile-time proof the Fake satisfies the interface it stands in for.
var _ termwrap.Terminal = (*termwrap.Fake)(nil)

func TestFake_RecordsInputAndSize(t *testing.T) {
	f := termwrap.NewFake("hello")

	f.SendInput("refactor the parser")
	f.Resize(80, 24)

	if got := f.View(); got != "hello" {
		t.Errorf("View() = %q, want %q", got, "hello")
	}
	if len(f.Sent) != 1 || f.Sent[0] != "refactor the parser" {
		t.Errorf("Sent = %v, want one entry %q", f.Sent, "refactor the parser")
	}
	if f.Width != 80 || f.Height != 24 {
		t.Errorf("size = %dx%d, want 80x24", f.Width, f.Height)
	}
}

func TestFake_FocusAndClose(t *testing.T) {
	f := termwrap.NewFake("")

	if f.Focused() {
		t.Error("Focused() = true on a new Fake, want false")
	}
	f.Focus()
	if !f.Focused() {
		t.Error("Focused() = false after Focus(), want true")
	}
	f.Blur()
	if f.Focused() {
		t.Error("Focused() = true after Blur(), want false")
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	if !f.Closed {
		t.Error("Closed = false after Close(), want true")
	}
}

// Init returns a command rather than nil. The old assertion here required
// nil, which encoded the bug in issue #33: a Fake that returns nothing lets a
// model whose Init discards terminals look correct. The Fake must behave like
// bubbleterm, whose Init returns the poll that keeps the terminal reading.
func TestFake_InitRecordsTheCallAndReturnsACommand(t *testing.T) {
	f := termwrap.NewFake("")

	cmd := f.Init()

	if !f.Inited {
		t.Error("Inited = false after Init(), want true")
	}
	if cmd == nil {
		t.Error("Init() = nil; a terminal that returns no command is never pumped")
	}
}

func TestFake_UpdateRecordsTheMessage(t *testing.T) {
	f := termwrap.NewFake("")

	if cmd := f.Update("a message"); cmd != nil {
		t.Errorf("Update() = %v, want nil", cmd)
	}
	if len(f.Msgs) != 1 || f.Msgs[0] != "a message" {
		t.Errorf("Msgs = %v, want one recorded message", f.Msgs)
	}
}
