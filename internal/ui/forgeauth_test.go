package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
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
