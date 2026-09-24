// Each project's pull requests (#310): read through internal/forge's gh, one
// call per project, off the render path and kept in memory only. The shape is
// repostat.go's, keyed by project instead of session.

package ui

import (
	"errors"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// PRListFunc lists a repository's pull requests. Injected so ui never runs gh
// itself; the argument is the project's root.
type PRListFunc func(projectRoot string) ([]forge.PR, error)

// noPRs is the Deps.PRs default: with nothing wired there is no gh to ask,
// which is exactly what forge says when gh is missing.
func noPRs(string) ([]forge.PR, error) { return nil, forge.ErrNoGH }

// PRsLoadedMsg carries one project's answer into Update. Exported so tests
// can send one.
type PRsLoadedMsg struct {
	Project string
	PRs     []forge.PR
	Err     error
}

// PRTickMsg is the heartbeat that polls every project. Exported so tests can
// send one.
type PRTickMsg time.Time

// prEvery is the poll's period. A minute: CI takes minutes, a turn ending
// polls at once (refreshPRs), and every tick is a request against the
// operator's own GitHub rate limit.
const prEvery = time.Minute

// prMinGap is the least time between two calls for one project, whatever
// asks - a turn ending, focus returning, the tick. It holds the cost to what
// the README promises (#310 final review).
const prMinGap = 30 * time.Second

func schedulePRTick() tea.Cmd {
	return tea.Tick(prEvery, func(t time.Time) tea.Msg { return PRTickMsg(t) })
}

// onPRTick polls every project and re-arms the tick.
func (m *Model) onPRTick() tea.Cmd { return tea.Batch(m.pollPRs(), schedulePRTick()) }

// pollPRs is one call per project holding a session. Nothing is asked while
// omatty is blurred, the diffstat poll's rule (#314) - onWindowFocus polls on
// the way back in - and nothing at all once gh has been found missing.
func (m *Model) pollPRs() tea.Cmd {
	if m.ghMissing || !m.hasFocus {
		return nil
	}
	var cmds []tea.Cmd
	for _, p := range m.state.Projects {
		if m.holdsSessionsIn(p.Name) {
			cmds = append(cmds, m.pollProjectPRs(p.Name))
		}
	}
	return tea.Batch(cmds...)
}

func (m *Model) holdsSessionsIn(project string) bool {
	for _, sess := range m.state.Sessions {
		if sess.Project == project {
			return true
		}
	}
	return false
}

// pollProjectPRs asks for one project's pull requests unless a call is in
// flight or the project is not on GitHub.
func (m *Model) pollProjectPRs(project string) tea.Cmd {
	if m.ghMissing || m.prPending[project] || m.prOff[project] {
		return nil
	}
	now := m.clock()
	if last, asked := m.prAsked[project]; asked && now.Sub(last) < prMinGap {
		return nil
	}
	m.prPending[project], m.prAsked[project] = true, now
	root, list := m.projectRoot(project), m.prList
	return func() tea.Msg {
		prs, err := list(root)
		return PRsLoadedMsg{Project: project, PRs: prs, Err: err}
	}
}

// refreshPRs polls a session's project when a turn finishes, the moment a
// push - and so a new CI run - is likeliest. Not on waiting: a permission
// prompt changes nothing on GitHub, and there can be many (final review).
func (m *Model) refreshPRs(id string, before, after watcher.Status) tea.Cmd {
	if !m.hasFocus || before == after || after != watcher.StatusDone {
		return nil
	}
	sess, ok := m.session(id)
	if !ok {
		return nil
	}
	return m.pollProjectPRs(sess.Project)
}

// onPRs stores an answer. gh missing stops every poll and a project gh cannot
// map to GitHub stops its own, each said once in the log and nowhere else:
// the card simply keeps its branch. Any other failure keeps the last list and
// marks the project failed, so the card says it does not know rather than
// showing the last verdict as current.
func (m *Model) onPRs(msg PRsLoadedMsg) tea.Cmd {
	delete(m.prPending, msg.Project)
	if msg.Err != nil {
		m.prFailure(msg.Project, msg.Err)
		return nil
	}
	delete(m.prFailed, msg.Project)
	m.prs[msg.Project] = msg.PRs
	return nil
}

// prFailure sorts a failed call into gh missing, not GitHub, or an outage.
func (m *Model) prFailure(project string, err error) {
	switch {
	case errors.Is(err, forge.ErrNoGH):
		m.loseGH()
	case errors.Is(err, forge.ErrNotGitHub):
		m.prOff[project] = true
		delete(m.prs, project)
		delete(m.prFailed, project)
		slog.Info("project is not on GitHub; its cards will not show pull requests", "project", project)
	default:
		if !m.prFailed[project] {
			slog.Warn("reading pull requests", "project", project, "err", err)
		}
		m.prFailed[project] = true
	}
}

// loseGH stops every poll and sends every card back to its branch: a verdict
// nothing will refresh must not stand as current for the rest of the run.
func (m *Model) loseGH() {
	if !m.ghMissing {
		slog.Info("gh is not on PATH; cards will not show pull requests")
	}
	m.ghMissing = true
	clear(m.prs)
	clear(m.prFailed)
}

// withPRMaps allocates the pull request state (#310). Keyed by project, so
// archive leaves it alone (skipSessionMaps) and forgetProject clears it.
func (m *Model) withPRMaps() *Model {
	m.prs = map[string][]forge.PR{}
	m.prPending = map[string]bool{}
	m.prFailed = map[string]bool{}
	m.prOff = map[string]bool{}
	m.prAsked = map[string]time.Time{}
	return m
}
