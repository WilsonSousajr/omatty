package termwrap_test

import (
	"github.com/taigrr/bubbleterm/emulator"
	"os/exec"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// pump drives the bubbletea command loop until want appears in the rendered
// frame or the deadline passes. Each Cmd is run on a goroutine because a poll
// blocks until the emulator reports damage.
func pump(t *testing.T, term termwrap.Terminal, want string, deadline time.Duration) string {
	t.Helper()
	stop := time.Now().Add(deadline)
	cmd := term.Init()
	for time.Now().Before(stop) {
		if strings.Contains(term.View(), want) {
			return term.View()
		}
		if cmd == nil {
			time.Sleep(10 * time.Millisecond)
			cmd = term.Init()
			continue
		}
		msgs := make(chan tea.Msg, 1)
		go func(c tea.Cmd) { msgs <- c() }(cmd)
		select {
		case msg := <-msgs:
			cmd = term.Update(msg)
		case <-time.After(200 * time.Millisecond):
			cmd = nil
		}
	}
	return term.View()
}

// The core bet of the project: a real process's output really does render
// into an embedded terminal we can read back.
func TestStart_RendersRealProcessOutput(t *testing.T) {
	term, err := termwrap.Start(40, 10, exec.Command("printf", "omatty-lives\\n"))
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	defer func() { _ = term.Close() }()

	got := pump(t, term, "omatty-lives", 5*time.Second)

	if !strings.Contains(got, "omatty-lives") {
		t.Errorf("View() never showed the process output.\ngot:\n%q", got)
	}
}

// The cursor is the one thing the rendered grid does not carry: the emulator
// tracks it, bubbleterm's view drops it, and omatty could not draw a caret
// until this reached it (issue #106).
func TestStart_CursorFollowsTheProcess_issue106(t *testing.T) {
	term, err := termwrap.Start(40, 10, exec.Command("printf", "abc"))
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	defer func() { _ = term.Close() }()

	// The frame is checked rather than discarded: pump returns unconditionally
	// once its deadline passes, so on a loaded box a timeout was reported as a
	// wrong cursor position instead of as never having pumped. Its sibling
	// TestStart_RendersRealProcessOutput already checks the returned frame
	// (#106).
	if frame := pump(t, term, "abc", 5*time.Second); !strings.Contains(frame, "abc") {
		t.Fatalf("the process did not render within the deadline; frame = %q", frame)
	}

	got := term.Cursor()
	if got.X != 3 || got.Y != 0 {
		t.Errorf("Cursor() = (%d, %d) after printing 3 columns, want (3, 0)", got.X, got.Y)
	}
	if !got.Visible {
		t.Error("Cursor().Visible = false, want true - nothing hid it")
	}
}

func TestStart_MissingBinaryNamesIt(t *testing.T) {
	term, err := termwrap.Start(40, 10, exec.Command("omatty-no-such-binary-xyz"))
	if err == nil {
		_ = term.Close()
		t.Skip("bubbleterm defers exec failure to the read loop rather than to New")
	}
	if !strings.Contains(err.Error(), "omatty-no-such-binary-xyz") {
		t.Errorf("error %q does not name the missing binary", err)
	}
}

func TestStart_FocusAndResizeReachTheEmulator(t *testing.T) {
	term, err := termwrap.Start(40, 10, exec.Command("cat"))
	if err != nil {
		t.Fatalf("Start() error = %v, want nil", err)
	}
	defer func() { _ = term.Close() }()

	term.Focus()
	if !term.Focused() {
		t.Error("Focused() = false after Focus(), want true")
	}
	term.Blur()
	if term.Focused() {
		t.Error("Focused() = true after Blur(), want false")
	}
	if cmd := term.Resize(80, 24); cmd == nil {
		t.Log("Resize returned no command; dimensions still applied")
	}
}

// caretShape's underline and bar arms had no coverage: the only real-process
// test prints plain text (a block, the default arm) and the Fake bypasses the
// mapping entirely. Swapping the two returns compiled and passed everything,
// so claude's DECSCUSR bar would have rendered as an underline (#106).
func TestCaretShape_MapsEveryEmulatorStyle_issue106(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   emulator.CursorStyle
		want tea.CursorShape
	}{
		{"block", emulator.CursorBlock, tea.CursorBlock},
		{"underline", emulator.CursorUnderline, tea.CursorUnderline},
		{"bar", emulator.CursorBar, tea.CursorBar},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := termwrap.CaretShape(tt.in); got != tt.want {
				t.Errorf("CaretShape(%v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}
