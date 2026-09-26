// Each project's open issues (#394): read through internal/forge's gh, one
// call per project, off the render path and kept in memory only. prpoll.go's
// shape, with two differences that are the whole point of a second poll.
//
//   - **Every project, not only those holding a session.** A card needs its own
//     session's pull request; a header needs its project's counts, and a project
//     you registered and have not started yet is exactly when its open issues
//     are worth reading (#158).
//   - **A slower period.** CI changes in minutes, which is why prEvery is one.
//     An issue list changes in days.
//
// Everything else is shared on purpose, through mayAsk: a fact about the
// machine (no gh) or about a checkout (not on GitHub) belongs to both lists,
// and the thirty-second floor is the cost promise the README makes.

package ui

import (
	"errors"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// IssueListFunc lists a repository's open issues. Injected so ui never runs gh
// itself; the argument is the project's root.
type IssueListFunc func(projectRoot string) ([]forge.Issue, error)

// noIssues is the Deps.Issues default: with nothing wired there is no gh to
// ask, which is exactly what forge says when gh is missing.
func noIssues(string) ([]forge.Issue, error) { return nil, forge.ErrNoGH }

// IssuesLoadedMsg carries one project's answer into Update. Exported so tests
// can send one.
type IssuesLoadedMsg struct {
	Project string
	Issues  []forge.Issue
	Err     error
}

// IssueTickMsg is the heartbeat that polls every project. Exported so tests
// can send one.
type IssueTickMsg time.Time

// issueEvery is the poll's period: five minutes, where prEvery is one. The
// difference is deliberate and is the reason this is a second tick rather than
// more work on the first - an issue list is not a CI verdict.
const issueEvery = 5 * time.Minute

func scheduleIssueTick() tea.Cmd {
	return tea.Tick(issueEvery, func(t time.Time) tea.Msg { return IssueTickMsg(t) })
}

// onIssueTick polls every project and re-arms the tick.
func (m *Model) onIssueTick() tea.Cmd { return tea.Batch(m.pollIssues(), scheduleIssueTick()) }

// pollIssues is one call per registered project. Nothing while omatty is
// blurred (#314) - onWindowFocus polls on the way back in - and nothing at all
// once gh has been found missing.
func (m *Model) pollIssues() tea.Cmd {
	if m.ghMissing || !m.hasFocus {
		return nil
	}
	var cmds []tea.Cmd
	for _, p := range m.state.Projects {
		cmds = append(cmds, m.pollProjectIssues(p.Name))
	}
	return tea.Batch(cmds...)
}

// pollProjectIssues asks for one project's open issues unless a call is in
// flight, the project is not on GitHub, or it was asked a moment ago.
func (m *Model) pollProjectIssues(project string) tea.Cmd {
	if !m.mayAsk(m.issuePending, m.issueAsked, project) {
		return nil
	}
	root, list := m.projectRoot(project), m.issueList
	return func() tea.Msg {
		issues, err := list(root)
		return IssuesLoadedMsg{Project: project, Issues: issues, Err: err}
	}
}

// onIssues stores an answer. The failure paths are prpoll's, for its reasons:
// gh missing stops every poll, a project gh cannot map to GitHub stops its own,
// and any other failure keeps the last list so a count is stale rather than
// reading as zero.
func (m *Model) onIssues(msg IssuesLoadedMsg) tea.Cmd {
	delete(m.issuePending, msg.Project)
	if msg.Err != nil {
		m.issueFailure(msg.Project, msg.Err)
		return nil
	}
	delete(m.issueFailed, msg.Project)
	m.issues[msg.Project] = msg.Issues
	return nil
}

// issueFailure sorts a failed call into gh missing, not GitHub, or an outage.
func (m *Model) issueFailure(project string, err error) {
	switch {
	case errors.Is(err, forge.ErrNoGH):
		m.loseGH()
	case errors.Is(err, forge.ErrNotGitHub):
		m.loseGitHub(project)
	default:
		if !m.issueFailed[project] {
			slog.Warn("reading issues", "project", project, "err", err)
		}
		m.issueFailed[project] = true
	}
}

// withIssueMaps allocates the issue state (#394). Keyed by project, like the
// pull request maps, so archive leaves it alone (skipSessionMaps) and
// forgetProject clears it.
func (m *Model) withIssueMaps() *Model {
	m.issues = map[string][]forge.Issue{}
	m.issuePending = map[string]bool{}
	m.issueFailed = map[string]bool{}
	m.issueAsked = map[string]time.Time{}
	return m
}

// forgetProjectIssues is forgetProject's half of this file.
func (m *Model) forgetProjectIssues(name string) {
	delete(m.issues, name)
	delete(m.issuePending, name)
	delete(m.issueFailed, name)
	delete(m.issueAsked, name)
}
