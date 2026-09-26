package ui

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// The forge maps are keyed by project, so archive leaves them alone and
// forgetting the project clears them (#310, #394).
func TestForgetProject_clearsItsForgeState_issue310(t *testing.T) {
	m := filledModel()
	m.prs["omatty"] = []forge.PR{{Number: 7}}
	m.issues["omatty"] = []forge.Issue{{Number: 399}}
	m.prPending["omatty"], m.prFailed["omatty"], m.notGitHub["omatty"] = true, true, true
	m.issuePending["omatty"], m.issueFailed["omatty"] = true, true
	m.prAsked["omatty"], m.issueAsked["omatty"] = m.clock(), m.clock()

	m.forgetProject("omatty")

	for name, held := range map[string]bool{
		"prs": m.prs["omatty"] != nil, "prPending": m.prPending["omatty"],
		"prFailed": m.prFailed["omatty"], "notGitHub": m.notGitHub["omatty"],
		"prAsked": !m.prAsked["omatty"].IsZero(),
		"issues":  m.issues["omatty"] != nil, "issuePending": m.issuePending["omatty"],
		"issueFailed": m.issueFailed["omatty"], "issueAsked": !m.issueAsked["omatty"].IsZero(),
	} {
		if held {
			t.Errorf("%s still holds the forgotten project", name)
		}
	}
}

// The two polls share one period only by accident of reading; the difference is
// the reason there are two ticks. A CI verdict changes in minutes, an issue
// list in days (#394).
func TestIssueTick_isSlowerThanThePullRequestTick_issue394(t *testing.T) {
	if issueEvery <= prEvery {
		t.Errorf("issueEvery = %v, prEvery = %v; want the issue poll slower", issueEvery, prEvery)
	}
}
