package app

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
)

// The forge maps are keyed by project, so archive leaves them alone and
// forgetting the project clears them (#310, #394).
func TestForgetProject_clearsItsForgeState_issue310(t *testing.T) {
	m := filledModel()
	m.prs["omatty"] = []forge.PR{{Number: 7}}
	m.issues["omatty"] = []forge.Issue{{Number: 399}}
	m.prPending["omatty"], m.prFailed["omatty"], m.forgeStopped["omatty"] = true, true, forge.ErrNoForge
	m.issuePending["omatty"], m.issueFailed["omatty"] = true, true
	m.prAsked["omatty"], m.issueAsked["omatty"] = m.clock(), m.clock()
	m.noTracker["omatty"] = true // and a forge that keeps no issues (#460)

	m.forgetProject("omatty")

	for name, held := range map[string]bool{
		"prs": m.prs["omatty"] != nil, "prPending": m.prPending["omatty"],
		"prFailed": m.prFailed["omatty"], "forgeStopped": m.forgeStopped["omatty"] != nil,
		"prAsked": !m.prAsked["omatty"].IsZero(),
		"issues":  m.issues["omatty"] != nil, "issuePending": m.issuePending["omatty"],
		"issueFailed": m.issueFailed["omatty"], "issueAsked": !m.issueAsked["omatty"].IsZero(),
		"noTracker": m.noTracker["omatty"],
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

// A project found to keep no issues is not also an outage: a failure marked
// before is cleared, so its header shows no "?" (#460's review).
func TestIssueFailure_noTrackerIsNotAnOutage_issue460(t *testing.T) {
	m := filledModel()
	m.issueFailed["omatty"] = true

	m.issueFailure("omatty", forge.ErrNoTracker)

	if m.issueFailed["omatty"] || !m.noTracker["omatty"] {
		t.Errorf("issueFailed = %v, noTracker = %v; want the outage cleared and no tracker kept", m.issueFailed["omatty"], m.noTracker["omatty"])
	}
}
