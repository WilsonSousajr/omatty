package ui

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// The PR maps are keyed by project, so archive leaves them alone and
// forgetting the project clears them (#310).
func TestForgetProject_clearsItsPullRequestState_issue310(t *testing.T) {
	m := filledModel()
	m.prs["omatty"] = []forge.PR{{Number: 7}}
	m.prPending["omatty"], m.prFailed["omatty"], m.prOff["omatty"] = true, true, true

	m.forgetProject("omatty")

	for name, held := range map[string]bool{
		"prs": m.prs["omatty"] != nil, "prPending": m.prPending["omatty"],
		"prFailed": m.prFailed["omatty"], "prOff": m.prOff["omatty"],
	} {
		if held {
			t.Errorf("%s still holds the forgotten project", name)
		}
	}
}
