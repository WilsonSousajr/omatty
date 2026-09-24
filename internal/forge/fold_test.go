package forge_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// pr builds one element of `gh pr list --json` with the given rollup.
func pr(state, merge, rollup string) string {
	return `{"number":349,"headRefName":"feat-x","state":"` + state +
		`","mergeStateStatus":"` + merge + `","statusCheckRollup":[` + rollup + `]}`
}

func run(status, conclusion string) string {
	return `{"__typename":"CheckRun","status":"` + status + `","conclusion":"` + conclusion + `"}`
}

func ctx(state string) string { return `{"__typename":"StatusContext","state":"` + state + `"}` }

func foldOne(t *testing.T, elem string) forge.PR {
	t.Helper()
	prs, err := forge.Fold([]byte("[" + elem + "]"))
	if err != nil {
		t.Fatalf("Fold() error = %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("Fold() = %d PRs, want 1", len(prs))
	}
	return prs[0]
}

// The card has one cell for CI, so the rollup is a precedence: anything
// failing wins, then anything still running, then passing (#310).
func TestFold_RollsChecksUpByPrecedence_issue310(t *testing.T) {
	for _, tt := range []struct {
		name   string
		rollup string
		want   forge.CIState
	}{
		{"one passing run", run("COMPLETED", "SUCCESS"), forge.CIPassing},
		{"a failing run", run("COMPLETED", "SUCCESS") + "," + run("COMPLETED", "FAILURE"), forge.CIFailing},
		{"a run in progress", run("COMPLETED", "SUCCESS") + "," + run("IN_PROGRESS", ""), forge.CIRunning},
		{"failure outranks running", run("IN_PROGRESS", "") + "," + run("COMPLETED", "TIMED_OUT"), forge.CIFailing},
		{"a pending status", ctx("PENDING"), forge.CIRunning},
		{"an expected status", ctx("EXPECTED"), forge.CIRunning},
		{"a failing status", ctx("FAILURE"), forge.CIFailing},
		{"an erroring status", ctx("ERROR"), forge.CIFailing},
		{"a passing status", ctx("SUCCESS"), forge.CIPassing},
		{"skipped and neutral pass", run("COMPLETED", "SKIPPED") + "," + run("COMPLETED", "NEUTRAL"), forge.CIPassing},
		{"cancelled fails", run("COMPLETED", "CANCELLED"), forge.CIFailing},
		{"action required fails", run("COMPLETED", "ACTION_REQUIRED"), forge.CIFailing},
		{"no checks at all", "", forge.CINone},
	} {
		if got := foldOne(t, pr("OPEN", "CLEAN", tt.rollup)).CI; got != tt.want {
			t.Errorf("%s: CI = %v, want %v", tt.name, got, tt.want)
		}
	}
}

// DIRTY is a conflict and BEHIND needs an update; both need the operator.
// UNKNOWN is GitHub still computing, and must not read as either.
func TestFold_ConflictIsDirtyOrBehindOnly_issue310(t *testing.T) {
	for merge, want := range map[string]bool{
		"DIRTY": true, "BEHIND": true, "UNKNOWN": false, "CLEAN": false, "BLOCKED": false, "": false,
	} {
		if got := foldOne(t, pr("OPEN", merge, "")).Conflict; got != want {
			t.Errorf("mergeStateStatus %q: Conflict = %v, want %v", merge, got, want)
		}
	}
}

func TestFold_CarriesNumberBranchAndState_issue310(t *testing.T) {
	for state, want := range map[string]forge.PRState{
		"OPEN": forge.Open, "MERGED": forge.Merged, "CLOSED": forge.Closed,
	} {
		got := foldOne(t, pr(state, "UNKNOWN", ""))
		if got.State != want || got.Number != 349 || got.Branch != "feat-x" {
			t.Errorf("state %q: %+v, want #349 on feat-x in state %v", state, got, want)
		}
	}
}

func TestFold_MalformedJSONIsAnError_issue310(t *testing.T) {
	_, err := forge.Fold([]byte("not json"))
	if err == nil || !strings.Contains(err.Error(), "forge") {
		t.Errorf("error = %v, want one naming forge", err)
	}
}
