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
// ready, copies exactly n bytes of input to a file - what the agent would
// read, byte for byte - and then says it is done.
func rawSink(t *testing.T, n int) (terminal.Terminal, string) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "got")
	script := "stty raw -echo; printf ready; head -c " + strconv.Itoa(n) + " > " + out + "; printf done"
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

// readSunk waits for the child to say it has read everything, then returns
// what it read. The child's own "done" is the event, so nothing polls the
// file on a timer (#734).
func readSunk(t *testing.T, term terminal.Terminal, path string) string {
	t.Helper()
	if !strings.Contains(pump(t, term, "done", 5*time.Second), "done") {
		t.Error("the child never finished reading")
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

	if got := readSunk(t, term, out); got != burst {
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

	if got := readSunk(t, term, out); got != want {
		t.Errorf("the child read %q, want %q", got, want)
	}
}
