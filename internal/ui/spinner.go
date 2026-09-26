package ui

import (
	"sync"
	"time"

	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// The working spinner (#410): a session that is thinking or running a tool
// turns its card glyph through braille frames, the one small moving mark
// every other tool in the field uses for "busy" (docs/research). It replaced
// #128's activity lane, six block cells whose height said the same thing as
// the glyph beside them and made every busy card a tall grey wall.
//
// Braille rather than claude's own ✻ family: ✳ has an emoji presentation some
// fonts draw two cells wide, and braille is not East Asian Ambiguous, so
// RUNEWIDTH_EASTASIAN=1 cannot double it either.

// spinFrames is one turn of the spinner.
var spinFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// spinEvery is how long a frame stays up: a turn a second.
const spinEvery = 100 * time.Millisecond

// spinnerFrame is the frame at now. A function of the clock rather than a
// counter on the model, so every card turns in step, a test can pass the
// time it wants, and a frame drawn twice in one tick is the same frame.
func spinnerFrame(now time.Time) string { return spinFrames[spinIndex(now)] }

// spinIndex is the frame's place in spinFrames. Floored, not truncated: a
// clock before 1970 has a negative UnixMilli and Go's % keeps its sign.
func spinIndex(now time.Time) int64 {
	n := int64(len(spinFrames))
	return (now.UnixMilli()/spinEvery.Milliseconds()%n + n) % n
}

// renderedSpin is every frame already coloured, the working states' colour,
// built once for the reason statusCells is (M13).
var renderedSpin = sync.OnceValue(func() [len(spinFrames)]string {
	var out [len(spinFrames)]string
	style := glyphStyle(watcher.StatusThinking)
	for i, f := range spinFrames {
		out[i] = style.Render(f)
	}
	return out
})

// working is whether a status means claude is busy with a turn. Thinking and
// tool are one spinner: both say "leave it", and telling them apart would
// flicker several times a turn for nothing the operator acts on.
func working(s watcher.Status) bool {
	return s == watcher.StatusThinking || s == watcher.StatusTool
}

// spins is whether a session's glyph turns: it is working and has a process.
// A card keeps its status after ctrl+o s (#318), so a session stopped
// mid-turn still reads "thinking"; spinning it would claim work nothing is
// doing, so it keeps the still ◐ or ◆.
func (m *Model) spins(id string, s watcher.Status) bool {
	return working(s) && m.terms[id] != nil
}

// glyphCell is a session's coloured status glyph at now: a spinner frame
// while it spins, its still glyph otherwise.
func (m *Model) glyphCell(id string, s watcher.Status, now time.Time) string {
	if m.spins(id, s) {
		return renderedSpin()[spinIndex(now)]
	}
	return statusCell(s)
}

// tickInterval is how long the heartbeat waits next: a spinner frame while
// any session spins, the age column's second otherwise. Idle, omatty ticks as
// it did before the spinner (M13); only work it is showing costs frames.
func (m *Model) tickInterval() time.Duration {
	for id, st := range m.status {
		if m.spins(id, st.Status) {
			return spinEvery
		}
	}
	return tickEvery
}
