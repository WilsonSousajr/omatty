// Package forge is omatty's interface over the gh CLI (#310): a repository's
// pull requests and their CI, its open issues, one item in full - and, since
// #331, the three writes a ship key makes.
//
// Read-only *by default*, which is a weaker claim than the one this package
// carried until #331 and the honest one now. It reads on a timer and writes only
// when somebody presses a key: opening a pull request, merging one that is
// already green on both sides, and reading whether a branch is protected in
// order to refuse. Everything else about the forge is still refused - no
// comment, no close, no label, no review.
//
// It is to gh what internal/infra/vcs is to git (invariant 4 in spirit): the one
// package that runs the binary, so the rest of omatty sees typed values and
// never parses gh's output itself.
package forge

import (
	"encoding/json"
	"fmt"
	dforge "github.com/WilsonSousajr/omatty/internal/domain/forge"
	"time"
)

// reviewOf is gh's reviewDecision as a Review.
func reviewOf(s string) dforge.Review {
	switch s {
	case "REVIEW_REQUIRED":
		return dforge.ReviewRequired
	case "APPROVED":
		return dforge.ReviewApproved
	case "CHANGES_REQUESTED":
		return dforge.ReviewChanges
	}
	return dforge.ReviewNone
}

// ghPR is one element of `gh pr list --json` with the fields ListPRs asks for.
type ghPR struct {
	Number            int       `json:"number"`
	Title             string    `json:"title"`
	HeadRefName       string    `json:"headRefName"`
	BaseRefName       string    `json:"baseRefName"`
	HeadRefOid        string    `json:"headRefOid"`
	IsCrossRepository bool      `json:"isCrossRepository"`
	State             string    `json:"state"`
	MergeStateStatus  string    `json:"mergeStateStatus"`
	StatusCheckRollup []check   `json:"statusCheckRollup"`
	IsDraft           bool      `json:"isDraft"`
	UpdatedAt         time.Time `json:"updatedAt"`
	MergedAt          time.Time `json:"mergedAt"`
	ReviewDecision    string    `json:"reviewDecision"`
}

// check is a CheckRun (status, conclusion) or a StatusContext (state); the
// rollup mixes the two.
type check struct {
	Typename   string `json:"__typename"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
	// Name (a CheckRun) or Context (a StatusContext) and the two times are
	// read only by an item's view, which lists each check (#433).
	Name        string    `json:"name"`
	Context     string    `json:"context"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
}

// Fold turns `gh pr list --json` output into PRs.
//
//	prs, err := forge.Fold(out)
func Fold(raw []byte) ([]dforge.PR, error) {
	var in []ghPR
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("forge: reading gh's pull request list: %w", err)
	}
	return foldPRs(in), nil
}

// foldPRs is Fold past the decoding, shared with the HTTP path, which decodes
// GitHub's GraphQL answer into the same ghPR gh itself answers with (#462).
func foldPRs(in []ghPR) []dforge.PR {
	out := make([]dforge.PR, len(in))
	for i, p := range in {
		out[i] = foldOne(p)
	}
	return out
}

// foldOne is one element of gh's list as omatty's own type. Split from Fold when
// #332's mergedAt took the loop past the length limit.
func foldOne(p ghPR) dforge.PR {
	return dforge.PR{
		Number:   p.Number,
		Title:    cleanLine(p.Title), // #483
		Branch:   cleanLine(p.HeadRefName),
		Base:     cleanLine(p.BaseRefName),
		State:    stateOf(p.State),
		CI:       rollup(p.StatusCheckRollup),
		Conflict: p.MergeStateStatus == "DIRTY" || p.MergeStateStatus == "BEHIND",
		Fork:     p.IsCrossRepository,
		Head:     p.HeadRefOid,
		Draft:    p.IsDraft,
		Updated:  p.UpdatedAt,
		MergedAt: p.MergedAt,
		Review:   reviewOf(p.ReviewDecision),
	}
}

func stateOf(s string) dforge.PRState {
	switch s {
	case "MERGED":
		return dforge.Merged
	case "CLOSED":
		return dforge.Closed
	}
	return dforge.Open
}

// rollup is a precedence, because the card has one cell: anything failing
// wins, then anything still running, then passing. UNKNOWN is never passing:
// a check that has not said is running, not green (Orca #18484).
func rollup(checks []check) dforge.CIState {
	if len(checks) == 0 {
		return dforge.CINone
	}
	for _, c := range checks {
		if failed(c) {
			return dforge.CIFailing
		}
	}
	for _, c := range checks {
		if running(c) {
			return dforge.CIRunning
		}
	}
	return dforge.CIPassing
}

var failingConclusion = map[string]bool{
	"FAILURE": true, "ERROR": true, "CANCELLED": true, "TIMED_OUT": true,
	"ACTION_REQUIRED": true, "STARTUP_FAILURE": true,
}

func failed(c check) bool {
	if c.Typename == "StatusContext" {
		return c.State == "FAILURE" || c.State == "ERROR"
	}
	return failingConclusion[c.Conclusion]
}

func running(c check) bool {
	if c.Typename == "StatusContext" {
		return c.State == "PENDING" || c.State == "EXPECTED"
	}
	// STALE is GitHub closing a check that never finished (final review).
	return c.Status != "COMPLETED" || c.Conclusion == "STALE"
}
