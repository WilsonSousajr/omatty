package termwrap

import (
	tea "charm.land/bubbletea/v2"
	"github.com/taigrr/bubbleterm/emulator"
)

// CaretShape exposes the style mapping so its arms can be asserted directly.
// The real-process test only ever produces a block, and the Fake bypasses the
// mapping, so two of its three arms were unreachable from any test (#106).
func CaretShape(s emulator.CursorStyle) tea.CursorShape { return caretShape(s) }
