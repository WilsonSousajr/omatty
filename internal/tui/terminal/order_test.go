package terminal_test

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

// rawSink starts a child that puts its terminal in raw mode, says it is
// ready, and copies exactly n bytes of input to a file: what the agent
// would read, byte for byte.
func rawSink(t *testing.T, n int) (terminal.Terminal, string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "got")
	script := "stty raw -echo; printf ready; exec head -c " + strconv.Itoa(n) + " > " + out
	term, err := terminal.Start(40, 10, session.Launch{Argv: []string{"sh", "-c", script}})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = term.Close() })
	if !strings.Contains(pump(t, term, "ready", 5*time.Second), "ready") {
		t.Fatal("the child never reported raw mode")
	}
	term.Focus()
	return term, out
}

// runConcurrently runs every Cmd on its own goroutine, as bubbletea's
// program loop does: nothing orders two commands returned one after the
// other.
func runConcurrently(cmds []tea.Cmd) {
	for _, c := range cmds {
		if c != nil {
			go c()
		}
	}
}

// waitForFile waits until path holds n bytes and returns them.
func waitForFile(t *testing.T, path string, n int) string {
	t.Helper()
	stop := time.Now().Add(5 * time.Second)
	for time.Now().Before(stop) {
		if b, err := os.ReadFile(path); err == nil && len(b) >= n {
			return string(b)
		}
		time.Sleep(10 * time.Millisecond)
	}
	b, _ := os.ReadFile(path)
	return string(b)
}

// A burst of keystrokes - fast typing, or a terminal's unbracketed paste -
// arrives as one key message after another. Each became its own concurrent
// tea.Cmd writing to the PTY, so two could land in either order: "word"
// reached codex as "wodr" (#725). The agent must read exactly what was
// typed, in order.
func TestTerminal_ABurstOfKeysReachesTheChildInOrder_issue725(t *testing.T) {
	const want = "abcdefghijklmnopqrstuvwxyz0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	burst := strings.Repeat(want, 8)
	term, out := rawSink(t, len(burst))

	var cmds []tea.Cmd
	for _, r := range burst {
		cmds = append(cmds, term.Update(tea.KeyPressMsg{Code: r, Text: string(r)}))
	}
	runConcurrently(cmds)

	if got := waitForFile(t, out, len(burst)); got != burst {
		t.Errorf("the child read\n%q\nwant\n%q", got, burst)
	}
}

// SendInput - a review's paste, an @path - shares the stream with typed
// keys: a key typed right after a paste must not overtake it (#725,
// invariant 8).
func TestTerminal_SendInputKeepsItsPlaceAmongKeys_issue725(t *testing.T) {
	want := "ab" + "\x1b[200~pasted\x1b[201~" + "cd"
	term, out := rawSink(t, len(want))

	cmds := []tea.Cmd{
		term.Update(tea.KeyPressMsg{Code: 'a', Text: "a"}),
		term.Update(tea.KeyPressMsg{Code: 'b', Text: "b"}),
		term.SendInput("\x1b[200~pasted\x1b[201~"),
		term.Update(tea.KeyPressMsg{Code: 'c', Text: "c"}),
		term.Update(tea.KeyPressMsg{Code: 'd', Text: "d"}),
	}
	runConcurrently(cmds)

	if got := waitForFile(t, out, len(want)); got != want {
		t.Errorf("the child read %q, want %q", got, want)
	}
}
