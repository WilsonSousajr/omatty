package termwrap_test

import (
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// driveFor polls the terminal for d without dropping a frame, unlike pump,
// which discards any poll slower than 200 ms; a child that writes only on a
// signal would otherwise look blank.
func driveFor(term termwrap.Terminal, d time.Duration) {
	deadline := time.Now().Add(d)
	cmd := term.Init()
	for time.Now().Before(deadline) {
		if cmd == nil {
			cmd = term.Init()
		}
		done := make(chan struct{})
		go func() { msg := cmd(); cmd = term.Update(msg); close(done) }()
		select {
		case <-done:
		case <-time.After(time.Until(deadline)):
			return
		}
	}
}

var sgrOnly = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// The Fake records every size, not only the last: a nudge to h-1 and back
// would otherwise read as h, whether or not h-1 ever happened (#191).
func TestFake_RecordsEveryResizeAndRepaint_issue191(t *testing.T) {
	f := termwrap.NewFake("")

	f.Resize(80, 24)
	f.Resize(80, 23)
	f.Repaint()

	if len(f.Sizes) != 2 || f.Sizes[1] != [2]int{80, 23} || f.Width != 80 || f.Height != 23 {
		t.Errorf("Sizes = %v (last %dx%d), want both resizes in order", f.Sizes, f.Width, f.Height)
	}
	if f.Repaints != 1 {
		t.Errorf("Repaints = %d, want 1", f.Repaints)
	}
}

// After a dtach re-attach the pane is blank: dtach cleared it and sent
// SIGWINCH, and claude, whose size did not change, repainted nothing.
// Repaint makes the size genuinely change - h-1, then h - so the child
// hears two signals and repaints (#191). The stand-in prints W per signal.
func TestTerminal_RepaintNudgesThePTYThroughADifferentSize_issue191(t *testing.T) {
	term, err := termwrap.Start(40, 6, exec.Command("sh", "-c", "trap 'echo W' WINCH; echo F; while :; do sleep 1; done"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = term.Close() }()
	// Not zero: two ioctls close together coalesce into one SIGWINCH whose
	// observed size is the original, the very no-op being fixed; 600 ms is
	// the first pause that delivered both here, so this is the real one.
	termwrap.SetRepaintDelay(term, termwrap.RepaintDelay)
	if frame := pump(t, term, "F", 5*time.Second); !strings.Contains(frame, "F") {
		t.Fatalf("the child never painted: %q", frame)
	}

	if cmd := term.Repaint(); cmd != nil {
		cmd()
	}
	driveFor(term, 2*termwrap.RepaintDelay+time.Second)

	plain := sgrOnly.ReplaceAllString(term.View(), "")
	if got := strings.Count(plain, "W"); got < 2 {
		t.Errorf("%d WINCH lines after Repaint, want 2 (one per size change):\n%s", got, plain)
	}
}
