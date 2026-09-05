// The mouse wheel over the session pane. Split from routing.go because a
// pointer event is answered by geometry - which pane is under it - rather
// than by the modal key router (#107).

package ui

import (
	tea "charm.land/bubbletea/v2"
)

// WheelNotchesPerPage is how many notches make one page of scrollback.
// Exported so the regression tests spin a real flick rather than hard-coding 3.
//
// A terminal reports one notch per physical click and Claude's pgup moves a
// whole page, so translating each notch into a page sent a momentum flick -
// tens of notches - tens of pages into the transcript, overshooting whatever
// the operator was trying to read (#107).
const WheelNotchesPerPage = 3

// wheelAccumulator turns a burst of notches into pages.
type wheelAccumulator struct{ notches int }

// page counts one notch in direction (+1 down, -1 up) and reports whether it
// completes a page. Reversing the wheel drops the part-page behind it, so a
// correction scrolls back immediately rather than spending notches undoing a
// count the operator cannot see.
func (a *wheelAccumulator) page(direction int) bool {
	if a.notches != 0 && (a.notches > 0) != (direction > 0) {
		a.notches = 0
	}
	a.notches += direction
	if a.notches >= WheelNotchesPerPage || a.notches <= -WheelNotchesPerPage {
		a.notches = 0
		return true
	}
	return false
}

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

// scrollPane sends a wheel notch to whatever is under the pointer.
func (m *Model) scrollPane(msg tea.MouseWheelMsg) tea.Cmd {
	direction, ok := wheelDirection(msg.Button)
	if !ok {
		return nil
	}
	if m.overReview(msg.X) {
		return m.scrollReview(direction)
	}
	return m.scrollTerminal(direction, msg.X, msg.Y)
}

// wheelDirection is +1 for a notch down the page, -1 for one up.
func wheelDirection(b tea.MouseButton) (int, bool) {
	switch b {
	case tea.MouseWheelUp:
		return -1, true
	case tea.MouseWheelDown:
		return 1, true
	}
	return 0, false
}

// scrollTerminal hands the focused session Claude's own scroll key.
func (m *Model) scrollTerminal(direction, winX, winY int) tea.Cmd {
	if !m.paneOwnsKeys() || !m.inPane(winX, winY) {
		return nil
	}
	if !m.wheel.page(direction) {
		return nil
	}
	// The key message, not hand-written bytes: bubbleterm owns the
	// key-to-escape translation, which is the rule routing.go's dispatch states
	// for every other key. Literal "\x1b[5~" would keep sending the old
	// encoding the day that translation changes, and diverge in silence.
	code := tea.KeyPgUp
	if direction > 0 {
		code = tea.KeyPgDown
	}
	return m.focusedTerminal().Update(tea.KeyPressMsg{Code: code})
}

// paneOwnsKeys reports whether a keystroke would reach the PTY right now.
//
// Keyboard focus, not merely a running terminal: with the review column or the
// note editor focused the pane's border is dimmed to say a keystroke will not
// land there (#21), and a wheel notch must not contradict it by writing
// PageDown into a live session while the operator types a comment (#107).
func (m *Model) paneOwnsKeys() bool {
	target, focused := m.focus()
	return focused && target == focusTerminal && m.focusedTerminal() != nil
}

// scrollReview drives the review column's own keymap, so each of its three
// views scrolls the offset it owns.
//
// Turning mouse reporting on took away the host terminal's alternate-scroll
// fallback, which had been the only wheel the diff, tree and preview ever had;
// without this arm they are the one surface in omatty where it does nothing.
func (m *Model) scrollReview(direction int) tea.Cmd {
	if direction > 0 {
		return m.onPaneKey("j")
	}
	return m.onPaneKey("k")
}

// overReview reports whether a window column falls inside the review column,
// which is drawn flush to the window's right edge.
func (m *Model) overReview(winX int) bool {
	w := ReviewWidth(m.width, m.review.Open)
	return w > 0 && winX >= m.width-w
}

// inPane reports whether a window cell is one the embedded terminal draws. It
// is the exact inverse of PaneOrigin.
func (m *Model) inPane(winX, winY int) bool {
	ox, oy := PaneOrigin()
	return m.inPaneGrid(winX-ox, winY-oy)
}
