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
	// Sent holds every string passed to SendInput, in order.
	Sent []string
	// Width and Height record the last Resize.
	Width, Height int
	// Closed reports whether Close has been called.
	Closed bool

	view    string
	focused bool
}

// NewFake returns a Fake whose View always renders view.
func NewFake(view string) *Fake { return &Fake{view: view} }

// Init does nothing; a Fake has no process to start.
func (f *Fake) Init() tea.Cmd { return nil }

// Update ignores every message.
func (f *Fake) Update(tea.Msg) tea.Cmd { return nil }

// View returns the fixed frame given to NewFake.
func (f *Fake) View() string { return f.view }

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
