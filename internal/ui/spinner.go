package ui

import (
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"

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

// SpinTickMsg redraws the frame for the spinner's next step (#412).
// Exported so tests can send one.
type SpinTickMsg time.Time

// anySpins is whether any session's glyph is turning.
func (m *Model) anySpins() bool {
	for id, st := range m.status {
		if m.spins(id, st.Status) {
			return true
		}
	}
	return false
}

// armSpin starts the spin tick if something spins and none is pending. Update
// calls it after every message, so the spinner starts on whichever message
// made a session spin - a status, or a stopped session's process starting -
// rather than on the next heartbeat, which left the glyph still for up to a
// second (#412). The heartbeat stays a plain second: idle, omatty ticks as it
// did before the spinner (M13), and only work it is showing costs frames.
func (m *Model) armSpin() tea.Cmd {
	if m.spinArmed || !m.anySpins() {
		return nil
	}
	m.spinArmed = true
	return m.spinTick(spinEvery, func(t time.Time) tea.Msg { return SpinTickMsg(t) })
}

// onSpinTick ends the pending tick. Update's armSpin re-arms it while
// something still spins; when nothing does, the chain stops here.
func (m *Model) onSpinTick() tea.Cmd {
	m.spinArmed = false
	return nil
}
