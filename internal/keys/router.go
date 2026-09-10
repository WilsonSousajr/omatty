// Package keys decides where a keystroke goes.
//
// It is a pure state machine with no bubbletea dependency so every path can
// be tested exhaustively. Invariant 1: when the terminal pane has focus,
// every key reaches the PTY except the leader. Never inspect a key and guess
// whether Claude wants it - guessing is what broke the earlier LazyVim
// attempt, because Claude Code binds esc, shift+tab, ctrl+r and ctrl+c.
package keys

// Route is what omatty does with a keystroke.
type Route int

const (
	// ToTerminal forwards the key verbatim to the PTY.
	ToTerminal Route = iota
	// ToOmatty handles the key as an omatty command.
	ToOmatty
	// Swallow consumes the key and forwards nothing. Only the leader.
	Swallow
)

// String makes test failures readable.
func (r Route) String() string {
	switch r {
	case ToTerminal:
		return "ToTerminal"
	case ToOmatty:
		return "ToOmatty"
	default:
		return "Swallow"
	}
}

// Router routes keystrokes.
//
//	r := keys.NewRouter("ctrl+o")
//	route := r.Next("esc", true) // ToTerminal
type Router struct {
	leader  string
	pending bool
}

// NewRouter returns a Router using leader as its escape hatch.
func NewRouter(leader string) *Router { return &Router{leader: leader} }

// Pending reports whether the leader was the previous key.
func (r *Router) Pending() bool { return r.pending }

// Arm makes the next key a command, as though the leader had just been pressed.
//
// Next cannot arm the leader while the terminal is unfocused, and an open modal
// surface is exactly that state - so `ctrl+o q` typed a literal q into a rename
// box and did nothing at all in the help box. The modal layer closes itself on
// the leader and calls this, so the pair completes wherever it is pressed
// (issues #41, #103).
//
//	m.router.Arm() // the next key is a command
func (r *Router) Arm() { r.pending = true }

// Next returns the route for key and advances the router's state.
//
// Losing focus disarms a pending leader: otherwise a leader pressed in the
// terminal would silently eat the first key typed in another pane.
func (r *Router) Next(key string, terminalFocused bool) Route {
	if !terminalFocused {
		r.pending = false
		return ToOmatty
	}
	if r.pending {
		r.pending = false
		return ToOmatty
	}
	if key == r.leader {
		r.pending = true
		return Swallow
	}
	return ToTerminal
}
