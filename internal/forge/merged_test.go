package forge_test

import (
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/forge"
)

// #332 measures lead time to the moment a pull request merged, and nothing was
// reading that moment: `finishedFields` asked for state and not for mergedAt,
// so every merged pull request came back with a zero time and no lead time
// could ever be computed. mergedAt rather than updatedAt because it is the
// thing itself rather than a proxy for it.
func TestFold_CarriesWhenAPullRequestMerged_issue332(t *testing.T) {
	at := time.Date(2026, 9, 26, 11, 30, 0, 0, time.UTC)
	raw := []byte(`[{"number":411,"headRefName":"feat/410","state":"MERGED",
		"mergedAt":"2026-09-26T11:30:00Z"}]`)

	prs, err := forge.Fold(raw)
	if err != nil {
		t.Fatal(err)
	}

	if len(prs) != 1 {
		t.Fatalf("folded %d pull requests, want 1", len(prs))
	}
	if !prs[0].MergedAt.Equal(at) {
		t.Errorf("MergedAt = %v, want %v", prs[0].MergedAt, at)
	}
}

// An open pull request has not merged, and a zero time is how that reads.
func TestFold_AnOpenPullRequestHasNoMergeTime_issue332(t *testing.T) {
	prs, err := forge.Fold([]byte(`[{"number":1,"headRefName":"feat/a","state":"OPEN"}]`))
	if err != nil {
		t.Fatal(err)
	}

	if !prs[0].MergedAt.IsZero() {
		t.Errorf("MergedAt = %v, want the zero time", prs[0].MergedAt)
	}
}

// The field has to be asked for, or the fold above has nothing to read. This
// asserts the request, which is the half a fold test cannot cover.
func TestListPRs_AsksForTheMergeTime_issue332(t *testing.T) {
	if !forge.FinishedFieldsInclude("mergedAt") {
		t.Error("the finished-pull-request field set does not ask for mergedAt, " +
			"so no merge time ever reaches the fold")
	}
}
