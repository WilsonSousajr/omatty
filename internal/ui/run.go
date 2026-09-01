package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// StartTerminals launches one embedded terminal per registered session,
// keyed by session id.
func StartTerminals(
	st registry.State, l *supervisor.Launcher, f termwrap.Factory, w, h int,
) (map[string]termwrap.Terminal, error) {
	terms := make(map[string]termwrap.Terminal, len(st.Sessions))
	for _, sess := range st.Sessions {
		term, err := l.Start(f, sess, w, h)
		if err != nil {
			return nil, fmt.Errorf("ui: starting terminal for session %s: %w", sess.ID, err)
		}
		terms[sess.ID] = term
	}
	return terms, nil
}

// Run starts every session's terminal and runs the TUI until the user quits.
// create is called when the operator finishes a new-session prompt.
func Run(
	st registry.State, l *supervisor.Launcher, f termwrap.Factory, w, h int,
	create CreateFunc,
) error {
	terms, err := StartTerminals(st, l, f, w, h)
	if err != nil {
		return err
	}
	if _, err := tea.NewProgram(NewModel(st, terms, create)).Run(); err != nil {
		return fmt.Errorf("ui: running the program with %d sessions: %w", len(terms), err)
	}
	return nil
}
