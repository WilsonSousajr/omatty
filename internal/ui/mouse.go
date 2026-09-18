// Giving the mouse back to the host terminal (#217).
//
// #107 asked the host for mouse reporting deliberately: without it no wheel
// event arrives at all, and the terminal's alternate scroll turns every notch
// into an arrow key that lands in Claude's prompt. What it did not add was any
// way to stop asking. `?1002h` hands *drags* to the application, which is the
// whole point of the mode and also the whole cost of it: while omatty holds it
// the terminal will not make a selection of its own, so nothing on screen can
// be selected or copy-on-selected, for the life of the session.
//
// Every multiplexer has the answer and it is the same one - tmux's
// `set -g mouse off`. This is that key. Released, the host owns the pointer
// again: native selection, native copy-on-select, the native context menu and
// no crosshair. Held, the wheel (#107), the sidebar's clicks (#45) and the
// review column's (#168) work as they did.
//
// The state is not persisted. state.json carries what a session needs to be
// relaunched (invariant 9) and nothing about a window, and config.toml refuses
// keys it does not know; a preference that outlived the process would be new
// precedent, and the issue asks for a toggle rather than a setting.

package ui

import (
	tea "charm.land/bubbletea/v2"
)

// mouseOffMark is what the header says while the host owns the pointer. Only
// the released state is marked: reporting is the default, and a marker that is
// always on the screen says nothing when it matters.
const mouseOffMark = "mouse off"

// toggleMouse releases the mouse to the host terminal, or takes it back.
//
// The part-page of wheel notches goes with it. A flick interrupted by the
// toggle would otherwise finish itself on the far side of it, scrolling a pane
// the operator had already handed to the terminal.
func (m *Model) toggleMouse() tea.Cmd {
	m.mouseReleased = !m.mouseReleased
	m.wheel = wheelAccumulator{}
	return nil
}

// mouseMode is what View asks the host for. bubbletea writes the mode only
// when it differs from the last frame's, so one toggle emits exactly one
// `?1002l` and the next exactly one `?1002h`.
func (m *Model) mouseMode() tea.MouseMode {
	if m.mouseReleased {
		return tea.MouseModeNone
	}
	return tea.MouseModeCellMotion
}
