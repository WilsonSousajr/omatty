package termwrap

import (
	"io"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/taigrr/bubbleterm/emulator"
)

// CaretShape exposes the style mapping so its arms can be asserted directly.
// The real-process test only ever produces a block, and the Fake bypasses the
// mapping, so two of its three arms were unreachable from any test (#106).
func CaretShape(s emulator.CursorStyle) tea.CursorShape { return caretShape(s) }

// GuardC1 exposes the reader that keeps 0x9C out of string payloads, so it can
// be fed chunked input without a process behind it (#192).
func GuardC1(r io.Reader) io.Reader { return &c1Guard{src: r} }

// SetRepaintDelay removes the pauses from Repaint, so its test does not wait
// on the wall clock (#191). A Terminal that is not the real emulator is left
// alone.
func SetRepaintDelay(t Terminal, d time.Duration) {
	if b, ok := t.(*bubble); ok {
		b.repaintDelay = d
	}
}

// RepaintDelay is the production pause, so the test proves the real number
// delivers two signals rather than a shorter one that would not (#191).
const RepaintDelay = repaintDelay
