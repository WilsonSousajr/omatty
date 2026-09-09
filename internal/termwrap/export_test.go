package termwrap

import (
	"io"

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
