package app_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
)

// A token the forge refused is a note naming the host and the variable - the
// operator's fix - and not an outage shown as a stale "?" (#453).
func TestTracker_ARefusedTokenSaysWhichHostAndVariable_issue453(t *testing.T) {
	m, fi, fp := modelWithBothLists(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	refused := &forge.AuthError{Host: "gitlab.com", TokenEnv: "GITLAB_TOKEN", Status: 401}
	for _, root := range []string{"/p/omatty", "/p/api-svc", "/p/empty"} {
		fi.Errs[root], fp.Errs[root] = refused, refused
	}

	openTracker(m)

	if got := columnText(m); !strings.Contains(got, "gitlab.com refused GITLAB_TOKEN (401)") {
		t.Errorf("the tracker reads %q, want the host and the variable named", got)
	}
}

// The environment cannot change under a running omatty, so a refused token is
// not asked again every minute: the project stops, as a missing tool does.
func TestModel_ARefusedTokenStopsItsProject_issue453(t *testing.T) {
	m, f := modelWithPRs(t)
	f.Errs["/p/omatty"] = &forge.AuthError{Host: "gitlab.com", TokenEnv: "GITLAB_TOKEN", Status: 401}
	deliver(m, m.PollPRs())
	f.Asked = nil
	f.later()

	deliver(m, m.PollPRs())

	if got := f.asked(); got != "/p/api-svc" {
		t.Errorf("asked %q, want only the project whose forge answered", got)
	}
}

// A CLI on PATH with no login for the host is not a missing CLI: the note
// says what the operator can fix, not that tea is absent (#586).
func TestTracker_ACLIWithNoLoginSaysSo_issue586(t *testing.T) {
	m, fi, fp := modelWithBothLists(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	noLogin := &forge.MissingToolError{Tool: "tea", NoLoginFor: "codeberg.org"}
	for _, root := range []string{"/p/omatty", "/p/api-svc", "/p/empty"} {
		fi.Errs[root], fp.Errs[root] = noLogin, noLogin
	}

	openTracker(m)

	got := columnText(m)
	if !strings.Contains(got, "tea has no login for codeberg.org") || strings.Contains(got, "not installed") {
		t.Errorf("the tracker reads %q, want tea's missing login named", got)
	}
}

// A token omatty will not send over plain http is a note naming the fix, and
// the project stops: no poll changes a remote's scheme (#584).
func TestTracker_APlainHTTPRemoteSaysHTTPSOnly_issue584(t *testing.T) {
	m, fi, fp := modelWithBothLists(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 32})
	plain := &forge.PlainHTTPError{Host: "git.corp.example", TokenEnv: "BITBUCKET_TOKEN"}
	for _, root := range []string{"/p/omatty", "/p/api-svc", "/p/empty"} {
		fi.Errs[root], fp.Errs[root] = plain, plain
	}

	openTracker(m)

	if got := columnText(m); !strings.Contains(got, "omatty sends BITBUCKET_TOKEN only over https, and git.corp.example is http") {
		t.Errorf("the tracker reads %q, want the https-only fix named", got)
	}
}

func TestModel_APlainHTTPRemoteStopsItsProject_issue584(t *testing.T) {
	m, f := modelWithPRs(t)
	f.Errs["/p/omatty"] = &forge.PlainHTTPError{Host: "git.corp.example", TokenEnv: "BITBUCKET_TOKEN"}
	deliver(m, m.PollPRs())
	f.Asked = nil
	f.later()

	deliver(m, m.PollPRs())

	if got := f.asked(); got != "/p/api-svc" {
		t.Errorf("the second poll asked %q, want only /p/api-svc", got)
	}
}
