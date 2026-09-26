package ui_test

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// FakeIssues is a named fake for ui.IssueListFunc, keyed by project root
// (#394), the shape FakePRs has.
type FakeIssues struct {
	Lists map[string][]forge.Issue
	Errs  map[string]error
	Asked []string
	Now   time.Time // the model's clock, so a test can step past the poll gap
}

func (f *FakeIssues) later() { f.Now = f.Now.Add(time.Minute) }

func (f *FakeIssues) List(root string) ([]forge.Issue, error) {
	f.Asked = append(f.Asked, root)
	return f.Lists[root], f.Errs[root]
}

func (f *FakeIssues) asked() string {
	a := append([]string(nil), f.Asked...)
	sort.Strings(a)
	return strings.Join(a, " ")
}

// withEmptyProject is twoProjectState plus a project nobody has started a
// session in: the case the issue poll must cover and the PR poll must not.
func withEmptyProject() registry.State {
	st := twoProjectState()
	st.Projects = append(st.Projects, registry.Project{Name: "empty", Root: "/p/empty"})
	return st
}

func modelWithIssues(t *testing.T) (*ui.Model, *FakeIssues) {
	t.Helper()
	terms, _ := fakeTerms(t)
	f := &FakeIssues{Lists: map[string][]forge.Issue{}, Errs: map[string]error{}, Now: fixedNow}
	d := baseDeps(withEmptyProject(), terms)
	d.Issues = f.List
	d.Clock = func() time.Time { return f.Now }
	return ui.NewModel(d), f
}

// One call per project, and unlike the PR poll that includes a project holding
// no sessions: a project you registered and have not started yet is exactly
// when its open issues are worth reading (#158's lesson, #310's rule).
func TestModel_anIssuePollAsksEveryProjectIncludingAnEmptyOne_issue394(t *testing.T) {
	m, f := modelWithIssues(t)
	f.Lists["/p/empty"] = []forge.Issue{{Number: 399, Title: "filter the tracker"}}

	deliver(m, m.PollIssues())

	if got := f.asked(); got != "/p/api-svc /p/empty /p/omatty" {
		t.Errorf("asked %q, want every project root once", got)
	}
	if got := m.IssuesOf("empty"); len(got) != 1 || got[0].Number != 399 {
		t.Errorf("IssuesOf(empty) = %+v, want the polled list", got)
	}
}

func TestModel_anIssuePollInFlightIsNotRepeated_issue394(t *testing.T) {
	m, f := modelWithIssues(t)
	first := m.PollIssues()

	deliver(m, m.PollIssues())
	if len(f.Asked) != 0 {
		t.Fatalf("asked %v while the first poll was in flight", f.Asked)
	}
	deliver(m, first)
	if len(f.Asked) != 3 {
		t.Errorf("after the first poll landed, asked %v, want every project", f.Asked)
	}
}

func TestModel_anIssueTickReArmsItself_issue394(t *testing.T) {
	m, _ := modelWithIssues(t)
	if _, cmd := m.Update(ui.IssueTickMsg(fixedNow)); cmd == nil {
		t.Error("an issue tick returned no command; the poll would run once and never again")
	}
}

// The PR poll's focus rule, for the same reason (#314): nothing while nobody
// is looking, everything on the way back.
func TestModel_noIssuePollWhileBlurredAndOneOnFocus_issue394(t *testing.T) {
	m, f := modelWithIssues(t)
	m.Update(tea.BlurMsg{})

	deliver(m, m.PollIssues())
	if len(f.Asked) != 0 {
		t.Fatalf("polled %v while blurred", f.Asked)
	}
	_, cmd := m.Update(tea.FocusMsg{})
	deliver(m, cmd)
	if got := f.asked(); got != "/p/api-svc /p/empty /p/omatty" {
		t.Errorf("on focus asked %q, want every project", got)
	}
}

// gh missing is a fact about the machine, not about which list was being read,
// so either poll finding it stops both.
func TestModel_withoutGhNeitherListIsAskedAgain_issue394(t *testing.T) {
	m, f := modelWithIssues(t)
	for _, root := range []string{"/p/omatty", "/p/api-svc", "/p/empty"} {
		f.Errs[root] = forge.ErrNoGH
	}
	deliver(m, m.PollIssues())
	f.Asked = nil
	f.later()

	deliver(m, m.PollIssues())
	_, cmd := m.Update(tea.FocusMsg{})
	deliver(m, cmd)

	if len(f.Asked) != 0 {
		t.Errorf("asked %v after gh was found missing", f.Asked)
	}
}

// A checkout that is not on GitHub is a fact about that checkout, so one list
// learning it stops the other asking too.
func TestModel_aProjectNotOnGitHubStopsBothLists_issue394(t *testing.T) {
	m, f := modelWithIssues(t)
	f.Errs["/p/omatty"] = fmt.Errorf("forge: no git remotes found: %w", forge.ErrNotGitHub)
	deliver(m, m.PollIssues())
	f.Asked = nil
	f.later()

	deliver(m, m.PollIssues())

	if got := f.asked(); got != "/p/api-svc /p/empty" {
		t.Errorf("asked %q, want only the GitHub projects", got)
	}
}

// Any other failure keeps the last list - a count that is only stale is better
// than one that reads as zero - marks the project, and logs once per outage.
func TestModel_aFailedIssuePollKeepsTheLastListAndWarnsOnce_issue394(t *testing.T) {
	var log bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	m, f := modelWithIssues(t)
	f.Lists["/p/omatty"] = []forge.Issue{{Number: 399}}
	deliver(m, m.PollIssues())
	f.Errs["/p/omatty"] = errors.New("HTTP 401: Bad credentials")

	f.later()
	deliver(m, m.PollIssues())
	f.later()
	deliver(m, m.PollIssues())

	if got := m.IssuesOf("omatty"); len(got) != 1 || !m.IssuesFailed("omatty") {
		t.Errorf("IssuesOf = %+v, failed = %v; want the last list kept and the project marked", got, m.IssuesFailed("omatty"))
	}
	if n := strings.Count(log.String(), "reading issues"); n != 1 {
		t.Errorf("logged %d warnings over two failed polls, want 1", n)
	}
	f.Errs["/p/omatty"] = nil
	f.later()
	deliver(m, m.PollIssues())
	if m.IssuesFailed("omatty") {
		t.Error("a successful poll did not clear the failure")
	}
}

// The floor the README promises, which the two polls share: whatever asks - a
// tick, focus returning, opening the tracker - one project is asked at most
// once in thirty seconds.
func TestModel_aProjectIsAskedForIssuesAtMostOnceInTheGap_issue394(t *testing.T) {
	m, f := modelWithIssues(t)
	deliver(m, m.PollIssues())
	f.Asked = nil

	deliver(m, m.PollIssues())
	if len(f.Asked) != 0 {
		t.Fatalf("asked %v inside the gap", f.Asked)
	}
	f.later()
	deliver(m, m.PollIssues())
	if got := f.asked(); got != "/p/api-svc /p/empty /p/omatty" {
		t.Errorf("after the gap asked %q, want every project again", got)
	}
}

// Without wiring the default says gh is missing rather than answering with an
// empty list - an empty list reads as "this project has no open issues", which
// is noDiff's argument (#21) - so nothing is held and nothing is asked again.
func TestModel_withoutAnIssueSourceNothingIsListedOrAskedAgain_issue394(t *testing.T) {
	terms, _ := fakeTerms(t)
	m := ui.NewModel(baseDeps(twoProjectState(), terms))

	deliver(m, m.PollIssues())

	if got := m.IssuesOf("omatty"); got != nil {
		t.Errorf("IssuesOf(omatty) = %+v, want nothing held", got)
	}
	if cmd := m.PollIssues(); cmd != nil {
		t.Error("polled again after the default reported gh missing")
	}
}
