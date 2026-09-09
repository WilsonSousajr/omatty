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

// onMouse answers a pointer event: the wheel scrolls whatever is under it
// (#107) and a left click on a sidebar row selects it (#45).
//
// Everything else is dropped, and dropping it is still the point. This runs
// before the broadcast in onWindowFocus, which would otherwise hand one
// untranslated event to every emulator at once - each of them reading
// coordinates measured from the window, not from its own pane. Motion, drag
// and release are dropped for that reason and for a second one: a release
// arrives with every click, and answering both would run moveCursor twice
// per click.
func (m *Model) onMouse(msg tea.MouseMsg) tea.Cmd {
	switch typed := msg.(type) {
	case tea.MouseWheelMsg:
		return m.scrollPane(typed)
	case tea.MouseClickMsg:
		return m.clickSidebar(typed)
	}
	return nil
}

// scrollPane sends a wheel notch to whatever is under the pointer.
//
// The review column is offered the whole message rather than a direction,
// because it is the only surface with two axes: the button and the modifier
// together pick which one a notch drives (#125).
//
// Everywhere else keeps taking a direction, which is what drops a horizontal
// notch over the session pane: wheelDirection reports false for buttons 6 and
// 7, so this returns before scrollTerminal. Claude has no horizontal scroll,
// and inventing one out of arrow keys is the corruption #107 fixed.
func (m *Model) scrollPane(msg tea.MouseWheelMsg) tea.Cmd {
	if m.overReview(msg.X) {
		return m.wheelReview(msg)
	}
	direction, ok := wheelDirection(msg.Button)
	if !ok {
		return nil
	}
	return m.scrollTerminal(direction, msg.X, msg.Y)
}

// wheelReview drives one of the review column's two axes with a notch:
// sideways for a horizontal button or a shifted vertical one, down the view
// for a plain vertical one. It is reached only from scrollPane's overReview
// arm, so the pointer is over the column by the time it runs.
func (m *Model) wheelReview(msg tea.MouseWheelMsg) tea.Cmd {
	if delta, ok := panDirection(msg); ok {
		m.panReview(delta * panStep)
		return nil
	}
	direction, ok := wheelDirection(msg.Button)
	if !ok {
		return nil
	}
	return m.scrollReview(direction)
}

// panDirection is +1 for a notch that pans right, -1 for one that pans left,
// and false for a notch that is not on the horizontal axis at all.
//
// Two gestures answer to it because neither is sufficient alone. Buttons 6 and
// 7 - what a trackpad's two-finger swipe reports - always arrive, but a
// wheel-only mouse cannot send them; shift+wheel covers that mouse, and yet
// Ghostty, kitty, xterm and Alacritty bypass mouse reporting on shift so the
// operator can select text, so there it never arrives at all (#125).
func panDirection(msg tea.MouseWheelMsg) (int, bool) {
	switch msg.Button {
	case tea.MouseWheelRight:
		return 1, true
	case tea.MouseWheelLeft:
		return -1, true
	}
	if msg.Mod&tea.ModShift == 0 {
		return 0, false
	}
	// A shifted vertical notch takes the vertical mapping onto this axis:
	// down is right and up is left, which is how every pager reads it.
	return wheelDirection(msg.Button)
}

// wheelDirection is +1 for a notch down the page, -1 for one up, and false
// for the horizontal buttons - which is what keeps them off the session pane
// (#125).
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

// overSidebar reports whether a window column falls inside the sidebar's
// content. The hairline on its right belongs to no one: a click there does
// nothing (#45, #174).
func (m *Model) overSidebar(winX int) bool { return winX >= 0 && winX < sidebarContentCols }

// inPane reports whether a window cell is one the embedded terminal draws. It
// is the exact inverse of PaneOrigin.
func (m *Model) inPane(winX, winY int) bool {
	ox, oy := PaneOrigin()
	return m.inPaneGrid(winX-ox, winY-oy)
}
