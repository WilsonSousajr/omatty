package termwrap_test

import (
	"github.com/taigrr/bubbleterm/emulator"
	"os/exec"
	"regexp"
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

// firstRow is the first non-blank row of a frame, escapes stripped and
// trimmed.
func firstRow(frame string) string {
	for _, l := range strings.Split(frame, "\n") {
		if s := strings.TrimSpace(sgr.ReplaceAllString(l, "")); s != "" {
			return s
		}
	}
	return ""
}

var sgr = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// claude sets its window title to "✳️ Claude Code". x/ansi's parser takes
// the 0x9C inside ✳ (E2 9C B3) for the 8-bit string terminator even in
// UTF-8, ends the OSC there, and prints the rest of the title onto the grid
// (#192). The emulator must draw BODY and nothing of the title.
func TestTerminal_anOSCTitleWithADingbatIsNotDrawn_issue192(t *testing.T) {
	term, err := termwrap.Start(40, 5, exec.Command("printf", "\033]0;\342\234\263\357\270\217 Claude Code\007BODY"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = term.Close() }()

	frame := pump(t, term, "BODY", 5*time.Second)

	if got := firstRow(frame); got != "BODY" {
		t.Errorf("first row = %q, want BODY alone: the title leaked onto the grid", got)
	}
}

func TestTerminal_aRawSTByteInATitleIsNotDrawn_issue192(t *testing.T) {
	term, err := termwrap.Start(40, 5, exec.Command("printf", "\033]0;a\234b title\007BODY"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = term.Close() }()

	frame := pump(t, term, "BODY", 5*time.Second)

	if got := firstRow(frame); got != "BODY" {
		t.Errorf("first row = %q, want BODY alone", got)
	}
}

func TestTerminal_aDCSPayloadWithADingbatIsNotDrawn_issue192(t *testing.T) {
	// x/vt swallows a byte or two after a DCS's ST whatever the guard does
	// (a plain payload with no 0x9C loses them too, checked against the raw
	// emulator), which is not the bug here. The assertion is that the payload
	// never reaches the grid and the text after it still does.
	term, err := termwrap.Start(40, 5, exec.Command("printf", "\033P1$r\342\234\263 payload\033\\\nBODY"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = term.Close() }()

	frame := pump(t, term, "DY", 5*time.Second)

	if got := firstRow(frame); strings.Contains(frame, "payload") || !strings.HasSuffix(got, "DY") {
		t.Errorf("first row = %q, want the text after the DCS and no payload text anywhere", got)
	}
}

// The child sees xterm-256color whatever the host's TERM is: x/vt does not
// implement every sequence a fancier terminfo entry would make claude emit.
// bubbleterm's StartCommand overwrote TERM; termwrap owning the PTY must too
// (#192).
func TestStart_SetsTermTo256Color_issue192(t *testing.T) {
	cmd := exec.Command("sh", "-c", `printf "term=%s" "$TERM"`)
	cmd.Env = []string{"TERM=xterm-ghostty", "PATH=/usr/bin:/bin"}
	term, err := termwrap.Start(40, 5, cmd)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = term.Close() }()

	frame := pump(t, term, "term=", 5*time.Second)

	if got := firstRow(frame); got != "term=xterm-256color" {
		t.Errorf("child saw %q, want term=xterm-256color", got)
	}
}

// Close twice is nil twice: closeTerminals logs every error per session on
// every quit, and a double-closed pty would say so every time (#192).
func TestTerminal_CloseIsIdempotentAndReturnsNil_issue192(t *testing.T) {
	term, err := termwrap.Start(40, 5, exec.Command("cat"))
	if err != nil {
		t.Fatal(err)
	}
	if err := term.Close(); err != nil {
		t.Errorf("first Close() = %v, want nil", err)
	}
	if err := term.Close(); err != nil {
		t.Errorf("second Close() = %v, want nil", err)
	}
}
