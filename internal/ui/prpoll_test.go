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
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// FakePRs is a named fake for ui.PRListFunc, keyed by project root (#310).
type FakePRs struct {
	Lists map[string][]forge.PR
	Errs  map[string]error
	Asked []string
	Now   time.Time // the model's clock, so a test can step past the poll gap
}

// later moves the clock past the least gap between two polls of a project.
func (f *FakePRs) later() { f.Now = f.Now.Add(time.Minute) }

func (f *FakePRs) List(root string) ([]forge.PR, error) {
	f.Asked = append(f.Asked, root)
	return f.Lists[root], f.Errs[root]
}

func (f *FakePRs) asked() string {
	a := append([]string(nil), f.Asked...)
	sort.Strings(a)
	return strings.Join(a, " ")
}

func modelWithPRs(t *testing.T) (*ui.Model, *FakePRs) {
	t.Helper()
	terms, _ := fakeTerms(t)
	f := &FakePRs{Lists: map[string][]forge.PR{}, Errs: map[string]error{}, Now: fixedNow}
	d := baseDeps(twoProjectState(), terms)
	d.PRs = f.List
	d.Clock = func() time.Time { return f.Now }
	return ui.NewModel(d), f
}

// One call per project, never per session or per pull request: the per-PR
// pattern is what tripped GitHub's secondary rate limit here before.
func TestModel_aPRPollAsksEachProjectOnceWithItsRoot_issue310(t *testing.T) {
	m, f := modelWithPRs(t)
	f.Lists["/p/omatty"] = []forge.PR{{Number: 7, Branch: "feat-x"}}

	deliver(m, m.PollPRs())

	if got := f.asked(); got != "/p/api-svc /p/omatty" {
		t.Errorf("asked %q, want each project root once", got)
	}
	if prs := m.PRsOf("omatty"); len(prs) != 1 || prs[0].Number != 7 {
		t.Errorf("PRsOf(omatty) = %+v, want the polled list", prs)
	}
}

func TestModel_aPRPollInFlightIsNotRepeated_issue310(t *testing.T) {
	m, f := modelWithPRs(t)
	first := m.PollPRs()

	deliver(m, m.PollPRs())
	if len(f.Asked) != 0 {
		t.Fatalf("asked %v while the first poll was in flight", f.Asked)
	}
	deliver(m, first)
	if len(f.Asked) != 2 {
		t.Errorf("after the first poll landed, asked %v, want both projects", f.Asked)
	}
}

func TestModel_aPRTickReArmsItself_issue310(t *testing.T) {
	m, _ := modelWithPRs(t)
	if _, cmd := m.Update(ui.PRTickMsg(fixedNow)); cmd == nil {
		t.Error("a PR tick returned no command; the poll would run once and never again")
	}
}

// Review Focus 4: nothing while nobody is looking, everything on the way back.
func TestModel_noPRPollWhileBlurredAndOneOnFocus_issue310(t *testing.T) {
	m, f := modelWithPRs(t)
	m.Update(tea.BlurMsg{})

	deliver(m, m.PollPRs())
	if len(f.Asked) != 0 {
		t.Fatalf("polled %v while blurred", f.Asked)
	}
	_, cmd := m.Update(tea.FocusMsg{})
	deliver(m, cmd)
	if got := f.asked(); got != "/p/api-svc /p/omatty" {
		t.Errorf("on focus asked %q, want every project", got)
	}
}

// A turn ending is when a push, and so a new CI run, is likeliest.
func TestModel_aSessionAtRestPollsItsOwnProject_issue310(t *testing.T) {
	m, f := modelWithPRs(t)

	_, cmd := m.Update(ui.StatusMsg{SessionID: "s3", Kind: watcher.TurnEnded, At: time.Now()})
	settle(m, cmd)

	if got := f.asked(); got != "/p/api-svc" {
		t.Errorf("asked %q, want only s3's project", got)
	}
}

// Review Focus 5: without gh there is nothing to ask, ever, this run.
func TestModel_withoutGhNothingIsAskedAgain_issue310(t *testing.T) {
	m, f := modelWithPRs(t)
	f.Errs["/p/omatty"], f.Errs["/p/api-svc"] = forge.ErrNoGH, forge.ErrNoGH
	deliver(m, m.PollPRs())
	f.Asked = nil
	f.later()

	deliver(m, m.PollPRs())
	_, cmd := m.Update(tea.FocusMsg{})
	deliver(m, cmd)
	_, cmd = m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.TurnEnded, At: time.Now()})
	settle(m, cmd)

	if len(f.Asked) != 0 {
		t.Errorf("asked %v after gh was found missing", f.Asked)
	}
}

// A project that is not on GitHub stops; the others carry on.
func TestModel_aProjectNotOnGitHubStopsAlone_issue310(t *testing.T) {
	m, f := modelWithPRs(t)
	f.Errs["/p/omatty"] = fmt.Errorf("forge: no git remotes found: %w", forge.ErrNotGitHub)
	deliver(m, m.PollPRs())
	f.Asked = nil
	f.later()

	deliver(m, m.PollPRs())

	if got := f.asked(); got != "/p/api-svc" {
		t.Errorf("asked %q, want only the GitHub project", got)
	}
}

// Any other failure keeps the last list, marks the project failed so the card
// can say it does not know, and logs once per outage.
func TestModel_aFailedPRPollKeepsTheLastListAndWarnsOnce_issue310(t *testing.T) {
	var log bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	m, f := modelWithPRs(t)
	f.Lists["/p/omatty"] = []forge.PR{{Number: 7, Branch: "feat-x"}}
	deliver(m, m.PollPRs())
	f.Errs["/p/omatty"] = errors.New("HTTP 401: Bad credentials")

	f.later()
	deliver(m, m.PollPRs())
	f.later()
	deliver(m, m.PollPRs())

	if prs := m.PRsOf("omatty"); len(prs) != 1 || !m.PRFailed("omatty") {
		t.Errorf("PRsOf = %+v, failed = %v; want the last list kept and the project marked failed", prs, m.PRFailed("omatty"))
	}
	if n := strings.Count(log.String(), "reading pull requests"); n != 1 {
		t.Errorf("logged %d warnings over two failed polls, want 1", n)
	}
	f.Errs["/p/omatty"] = nil
	f.later()
	deliver(m, m.PollPRs())
	if m.PRFailed("omatty") {
		t.Error("a successful poll did not clear the failure")
	}
}

// Final review, 3: the rate the README promises. A permission prompt is
// "waiting" and is not a moment CI changes; only a finished turn polls.
func TestModel_aPermissionPromptDoesNotPollPRs_issue310(t *testing.T) {
	m, f := modelWithPRs(t)

	_, cmd := m.Update(ui.StatusMsg{SessionID: "s3", Kind: watcher.PermissionRequested, At: time.Now()})
	settle(m, cmd)

	if len(f.Asked) != 0 {
		t.Errorf("a permission prompt polled %v", f.Asked)
	}
}

// Final review, 3: turns ending and focus returning in quick succession ask
// a project at most once in the gap; after it, they ask again.
func TestModel_aProjectIsAskedAtMostOnceInTheGap_issue310(t *testing.T) {
	m, f := modelWithPRs(t)
	deliver(m, m.PollPRs())
	f.Asked = nil

	_, cmd := m.Update(ui.StatusMsg{SessionID: "s3", Kind: watcher.TurnEnded, At: time.Now()})
	settle(m, cmd)
	_, cmd = m.Update(tea.FocusMsg{})
	deliver(m, cmd)
	if len(f.Asked) != 0 {
		t.Fatalf("asked %v within the gap after the last poll", f.Asked)
	}

	f.later()
	_, cmd = m.Update(tea.FocusMsg{})
	deliver(m, cmd)
	if got := f.asked(); got != "/p/api-svc /p/omatty" {
		t.Errorf("after the gap, focus asked %q, want every project", got)
	}
}

// Final review, 10: a turn ending while blurred spends no request either.
func TestModel_aTurnEndingWhileBlurredDoesNotPollPRs_issue310(t *testing.T) {
	m, f := modelWithPRs(t)
	m.Update(tea.BlurMsg{})

	_, cmd := m.Update(ui.StatusMsg{SessionID: "s3", Kind: watcher.TurnEnded, At: time.Now()})
	settle(m, cmd)

	if len(f.Asked) != 0 {
		t.Errorf("a turn ending while blurred polled %v", f.Asked)
	}
}

// Final review, 4: once gh is gone, or a project stops mapping to GitHub, the
// last verdict must not stay on the card as current for the rest of the run.
func TestModel_losingGhOrGitHubDropsTheLastVerdict_issue310(t *testing.T) {
	m, f := modelWithPRs(t)
	f.Lists["/p/omatty"] = []forge.PR{{Number: 7, Branch: "feat-x"}}
	f.Lists["/p/api-svc"] = []forge.PR{{Number: 8, Branch: "feat-y"}}
	deliver(m, m.PollPRs())

	m.Update(ui.PRsLoadedMsg{Project: "api-svc", Err: fmt.Errorf("gone: %w", forge.ErrNotGitHub)})
	if len(m.PRsOf("api-svc")) != 0 || m.PRFailed("api-svc") {
		t.Errorf("api-svc kept %+v (failed %v) after it stopped mapping to GitHub", m.PRsOf("api-svc"), m.PRFailed("api-svc"))
	}
	m.Update(ui.PRsLoadedMsg{Project: "omatty", Err: forge.ErrNoGH})
	if len(m.PRsOf("omatty")) != 0 {
		t.Errorf("omatty kept %+v after gh went missing", m.PRsOf("omatty"))
	}
}
