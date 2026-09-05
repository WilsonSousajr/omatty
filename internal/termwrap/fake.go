package termwrap

import (
	tea "charm.land/bubbletea/v2"
)

// Fake is a Terminal that records what it was told, for tests in other
// packages. It lives in the production package because supervisor and ui
// both need it.
//
//	f := termwrap.NewFake("session one")
//	model := ui.NewModel(state, map[string]termwrap.Terminal{"s1": f})
type Fake struct {
	// Sent holds every string passed to SendInput, in order. Review
	// submission uses SendInput; keystrokes do not.
	Sent []string
	// Msgs holds every message passed to Update, in order. Keystrokes reach
	// a terminal this way, because bubbleterm does its own key-to-escape
	// translation.
	Msgs []tea.Msg
	// Width and Height record the last Resize.
	Width, Height int
	// Closed reports whether Close has been called.
	Closed bool
	// Inited reports whether Init has been called. A terminal that is never
	// initialised never reads from its PTY (issue #33).
	Inited bool
	// Caret is what Cursor reports, so a test can place the emulated cursor
	// without driving a real emulator (issue #106).
	Caret Caret

	view    string
	focused bool
}

// NewFake returns a Fake whose View always renders view.
func NewFake(view string) *Fake { return &Fake{view: view} }

// Init records the call. A Fake has no process, but the model must still
// initialise every terminal it owns.
func (f *Fake) Init() tea.Cmd {
	f.Inited = true
	return func() tea.Msg { return nil }
}

// Update records msg instead of driving an emulator.
func (f *Fake) Update(msg tea.Msg) tea.Cmd {
	f.Msgs = append(f.Msgs, msg)
	return nil
}

// View returns the fixed frame given to NewFake.
func (f *Fake) View() string { return f.view }

// Cursor returns the Caret the test set.
func (f *Fake) Cursor() Caret { return f.Caret }

// Focus marks the Fake focused.
func (f *Fake) Focus() { f.focused = true }

// Blur marks the Fake unfocused.
func (f *Fake) Blur() { f.focused = false }

// Focused reports the focus state.
func (f *Fake) Focused() bool { return f.focused }

// Close records that the terminal was closed.
func (f *Fake) Close() error {
	f.Closed = true
	return nil
}

// SendInput records s instead of writing it to a PTY.
func (f *Fake) SendInput(s string) tea.Cmd {
	f.Sent = append(f.Sent, s)
	return nil
}

// Resize records the requested dimensions.
func (f *Fake) Resize(w, h int) tea.Cmd {
	f.Width, f.Height = w, h
	return nil
}
