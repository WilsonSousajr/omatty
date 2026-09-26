// A session's coverage overlay (#254): which of the lines in its tree are
// exercised, read out of the profile its own gate just wrote.
//
// Held per session and display-only, like repoStat and the gate report, and
// never persisted - state.json must suffice to relaunch a session on its own
// (invariant 9). An overlay is re-read the next time the gate runs.
//
// Stale by construction, deliberately. It describes the tree as the gate found
// it; when the session edits further the markers are about to be wrong, which
// is exactly the freshness the diff and the diffstat beside it already have.

package ui

import (
	"log/slog"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/coverage"
	"github.com/WilsonSousajr/omatty/internal/gate"
)

// coverageMsg carries one session's freshly read overlay back into Update.
type coverageMsg struct {
	id      string
	profile coverage.Profile
	err     error
}

// loadCoverage reads the profile a session's gate declares, or nil when
// nothing declares one - which is most gates.
//
// A command rather than a read in Update: a profile is a file, and the size of
// it is the project's business, not omatty's. The read happens in the session's
// own directory, which for a worktree session is that worktree, so two sessions
// never read each other's numbers.
func (m *Model) loadCoverage(id string) tea.Cmd {
	sess, found := m.sessionByID(id)
	if !found {
		return nil
	}
	declared := declaredProfile(m.gateFor(id))
	if declared == "" {
		return nil
	}
	dir, path := sess.Dir, filepath.Join(sess.Dir, declared)
	return func() tea.Msg {
		p, err := coverage.Load(path, dir, coverage.ModulePath(dir))
		return coverageMsg{id: id, profile: p, err: err}
	}
}

// declaredProfile is the profile path a gate's coverage step declares (#253).
// The first one: a gate with two coverage steps has no single overlay, and
// picking the first is at least the one the operator listed first.
func declaredProfile(steps []gate.Step) string {
	for _, s := range steps {
		if s.Kind == gate.KindCoverage && s.Profile != "" {
			return s.Profile
		}
	}
	return ""
}

// onCoverage stores an overlay, or keeps the one already held.
//
// A profile that is missing or will not parse leaves the previous overlay in
// place. Blanking it would quietly claim that nothing is uncovered, which is
// the one thing the operator has no way to notice; stale markers at least go
// stale in the direction the diff beside them already does.
//
// The warning is made once per session until a read succeeds, the way
// statFailed makes one warning per outage rather than one per poll (#180).
func (m *Model) onCoverage(msg coverageMsg) {
	if msg.err != nil {
		if !m.coverFailed[msg.id] {
			slog.Warn("reading a session's coverage profile", "session", msg.id, "err", msg.err)
			m.coverFailed[msg.id] = true
		}
		return
	}
	delete(m.coverFailed, msg.id)
	m.covers[msg.id] = msg.profile
}
