// The mouse wheel over the session pane. Split from routing.go because a
// pointer event is answered by geometry - which pane is under it - rather
// than by the modal key router (#107).

package ui

import (
	tea "charm.land/bubbletea/v2"
)

// scrollBack and scrollForward are the keys Claude Code documents for moving
// through its transcript; it prints a hint naming them when it receives the
// arrow keys a terminal's alternate scroll sends in their place.
//
// omatty forwards these rather than an SGR mouse report because vt.SendMouse
// drops a report unless the child has set a mouse mode, and claude's is off
// in exactly the state where you want to scroll (#107).
const (
	scrollBack    = "\x1b[5~" // page up
	scrollForward = "\x1b[6~" // page down
)

// onMouse answers a pointer event. Only the wheel does anything; clicks and
// drags are dropped until #45 adds sidebar hit-testing.
//
// Dropping them is the point. This runs before the broadcast in
// onWindowFocus, which would otherwise hand one untranslated event to every
// emulator at once - each of them reading coordinates measured from the
// window, not from its own pane.
func (m *Model) onMouse(msg tea.MouseMsg) tea.Cmd {
	wheel, ok := msg.(tea.MouseWheelMsg)
	if !ok {
		return nil
	}
	return m.scrollPane(wheel)
}

// scrollPane hands the focused session Claude's own scroll key, if the
// pointer is over its pane. A modal leaves no focused terminal, which is the
// right answer for free: there is no transcript under the pointer to scroll.
func (m *Model) scrollPane(msg tea.MouseWheelMsg) tea.Cmd {
	term := m.focusedTerminal()
	if _, _, inside := m.paneCell(msg.X, msg.Y); term == nil || !inside {
		return nil
	}
	switch msg.Button {
	case tea.MouseWheelUp:
		return term.SendInput(scrollBack)
	case tea.MouseWheelDown:
		return term.SendInput(scrollForward)
	}
	return nil
}

// paneCell maps a window cell to the embedded terminal's own grid, reporting
// false when the pointer is outside the pane. It is the exact inverse of
// PaneOrigin.
func (m *Model) paneCell(winX, winY int) (x, y int, inside bool) {
	ox, oy := PaneOrigin()
	x, y = winX-ox, winY-oy
	return x, y, m.inPaneGrid(x, y)
}
