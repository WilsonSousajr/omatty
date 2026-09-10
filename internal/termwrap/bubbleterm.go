package termwrap

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/creack/pty"
	"github.com/taigrr/bubbleterm"
	"github.com/taigrr/bubbleterm/emulator"
)

// bubble adapts bubbleterm.Model to Terminal. It is unexported: callers get
// the interface from Start, never the concrete type.
//
// termwrap owns the PTY rather than letting bubbleterm open one, so the bytes
// the child writes pass through c1Guard before the emulator parses them
// (#192). That costs three things bubbleterm's command mode did for free,
// each here: the window size (Resize), closing both ends and reaping the
// process (Close, attach), and TERM (withTerm).
type bubble struct {
	m         *bubbleterm.Model
	ptmx, tty *os.File
	w, h      int // the last size given, for Repaint; bubbleterm's are unexported
	// repaintDelay is the pause before and between Repaint's two size
	// changes. A field so a test can set it to zero.
	repaintDelay time.Duration
	closeOnce    sync.Once
	closeErr     error
}

// repaintDelay is how long Repaint waits before the first size change and
// again between the two. Before: dtach's own clear and SIGWINCH land after
// the client attaches, and the nudge must follow them, not precede them.
// Between: two ioctls close together coalesce into one SIGWINCH whose
// observed size is the original, the very no-op being fixed. Measured
// against a shell that echoes per signal: 100 and 300 ms delivered one, 600
// delivered two, so the pause is well over that (#191).
const repaintDelay = 800 * time.Millisecond

// Start launches cmd inside a w by h embedded terminal.
//
//	term, err := termwrap.Start(80, 24, exec.Command("claude", "--session-id", id))
func Start(w, h int, cmd *exec.Cmd) (Terminal, error) {
	ptmx, tty, err := openPTY(w, h)
	if err != nil {
		return nil, fmt.Errorf("termwrap: opening a %dx%d pty for %q: %w", w, h, cmd.Path, err)
	}
	if err := attach(cmd, tty); err != nil {
		closeBoth(ptmx, tty)
		return nil, fmt.Errorf("termwrap: starting %q in a %dx%d terminal: %w", cmd.Path, w, h, err)
	}
	m, err := bubbleterm.NewWithPipes(w, h, &c1Guard{src: ptmx}, nopCloser{ptmx})
	if err != nil {
		closeBoth(ptmx, tty)
		return nil, fmt.Errorf("termwrap: wrapping %q in a %dx%d emulator: %w", cmd.Path, w, h, err)
	}
	return &bubble{m: m, ptmx: ptmx, tty: tty, w: w, h: h, repaintDelay: repaintDelay}, nil
}

// openPTY opens a pair sized w by h, pixels included as bubbleterm set them,
// so a child reading TIOCGWINSZ sees what it saw before.
func openPTY(w, h int) (ptmx, tty *os.File, err error) {
	ptmx, tty, err = pty.Open()
	if err != nil {
		return nil, nil, err
	}
	if err := pty.Setsize(ptmx, winsize(w, h)); err != nil {
		closeBoth(ptmx, tty)
		return nil, nil, err
	}
	return ptmx, tty, nil
}

func winsize(w, h int) *pty.Winsize {
	return &pty.Winsize{Rows: uint16(h), Cols: uint16(w), X: uint16(w * 8), Y: uint16(h * 16)}
}

// attach makes tty the child's controlling terminal and starts it. The
// process is reaped in the background: nothing in omatty reads its exit
// status (the watcher's transcript is the source of truth, invariant 2), and
// a zombie per closed session would be the alternative.
func attach(cmd *exec.Cmd, tty *os.File) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setsid, cmd.SysProcAttr.Setctty = true, true
	cmd.Env = withTerm(cmd.Env)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// withTerm replaces TERM with xterm-256color, the terminal x/vt implements.
// omatty never sets cmd.Env, so without this the host's own value (an
// xterm-ghostty, say) would reach claude and make it emit sequences the
// emulator does not know. bubbleterm's StartCommand did the same.
func withTerm(env []string) []string {
	if env == nil {
		env = os.Environ()
	}
	out := make([]string, 0, len(env)+1)
	for _, e := range env {
		if !strings.HasPrefix(e, "TERM=") {
			out = append(out, e)
		}
	}
	return append(out, "TERM=xterm-256color")
}

func closeBoth(ptmx, tty *os.File) {
	_ = ptmx.Close()
	_ = tty.Close()
}

// nopCloser hands the emulator the ptmx to write to without the right to
// close it: the emulator's pipe-mode Close closes its writer, and the ptmx
// is closed once, here, in bubble.Close.
type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

func (b *bubble) Init() tea.Cmd              { return b.m.Init() }
func (b *bubble) SendInput(s string) tea.Cmd { return b.m.SendInput(s) }
func (b *bubble) Focus()                     { b.m.Focus() }
func (b *bubble) Blur()                      { b.m.Blur() }
func (b *bubble) Focused() bool              { return b.m.Focused() }

// Resize sizes the PTY here - the emulator skips that in pipe mode - and
// then reflows the grid. The interface has no error to return, so a failed
// ioctl is logged; the grid still reflows, and the child hears about it on
// the next size change.
func (b *bubble) Resize(w, h int) tea.Cmd {
	b.w, b.h = w, h
	b.setsize(w, h)
	return b.m.Resize(w, h)
}

func (b *bubble) setsize(w, h int) {
	if err := pty.Setsize(b.ptmx, winsize(w, h)); err != nil {
		slog.Warn("resizing a pty", "cols", w, "rows", h, "err", err)
	}
}

// Repaint changes the PTY's size to one row less and back, off the Update
// goroutine, with the delays repaintDelay explains. Only the PTY is
// touched: the grid keeps its size, so nothing reflows on omatty's side and
// the child's own repaint lands on a grid of the size it believes in. A
// failed ioctl is logged by setsize; the pane then stays as it was, which
// is the status quo this exists to improve on (#191).
func (b *bubble) Repaint() tea.Cmd {
	w, h, delay := b.w, b.h, b.repaintDelay
	return func() tea.Msg {
		time.Sleep(delay)
		b.setsize(w, h-1)
		time.Sleep(delay)
		b.setsize(w, h)
		return nil
	}
}

// Close stops the emulator and closes both ends of the PTY, once. Closing
// the parent's ends is what bubbleterm's command mode did too: the child
// keeps its own descriptors, so a dtach-held claude survives (M6).
func (b *bubble) Close() error {
	b.closeOnce.Do(func() {
		_ = b.m.Close()
		b.closeErr = errors.Join(b.tty.Close(), b.ptmx.Close())
	})
	return b.closeErr
}

// Cursor reads the cursor straight off the emulator. bubbleterm's own view
// carries none, and the rendered grid does not paint the cell, so this is the
// only route to the caret in Claude's prompt (issue #106).
func (b *bubble) Cursor() Caret {
	emu := b.m.GetEmulator()
	pos, visible := emu.Cursor()
	look := emu.CursorAppearance()
	return Caret{X: pos.X, Y: pos.Y, Visible: visible, Shape: caretShape(look.Style), Blink: look.Blink}
}

// caretShape maps the emulator's cursor style to bubbletea's.
//
// Named cases rather than an int cast, so reordering either iota keeps the
// mapping correct instead of silently shifting every shape by one. That is the
// benefit; it is not a compile-time guarantee, and the earlier comment claimed
// one it does not provide: a new style added upstream falls into default and
// renders as a block, which no test would catch (#106).
func caretShape(s emulator.CursorStyle) tea.CursorShape {
	switch s {
	case emulator.CursorUnderline:
		return tea.CursorUnderline
	case emulator.CursorBar:
		return tea.CursorBar
	default:
		return tea.CursorBlock
	}
}

// View returns the rendered cell grid. tea.View exposes no String method;
// Content is the field holding the styled screen text.
func (b *bubble) View() string { return b.m.View().Content }

// Update folds bubbleterm's returned model back in, so callers keep a stable
// Terminal reference across updates.
func (b *bubble) Update(msg tea.Msg) tea.Cmd {
	next, cmd := b.m.Update(msg)
	if m, ok := next.(*bubbleterm.Model); ok {
		b.m = m
	}
	return cmd
}
