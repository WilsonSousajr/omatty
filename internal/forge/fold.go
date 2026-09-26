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
// It is to gh what internal/vcs is to git (invariant 4 in spirit): the one
// package that runs the binary, so the rest of omatty sees typed values and
// never parses gh's output itself.
package forge

import (
	"encoding/json"
	"fmt"
	"time"
)

// PRState is where a pull request stands.
type PRState int

// The three states gh reports.
const (
	Open PRState = iota
	Merged
	Closed
)

// CIState is a pull request's checks rolled up into the one cell a card has.
type CIState int

// The rollup's four answers, in rising precedence.
const (
	CINone    CIState = iota // no checks reported at all
	CIPassing                // every check finished, none failed
	CIRunning                // nothing failed yet, something still running
	CIFailing                // at least one check failed
)

// PR is one pull request as a session card needs it, and as the tracker's list
// draws it (#393).
type PR struct {
	Number   int
	Title    string // empty on a finished one: only the open set is asked for it
	Branch   string // the head branch, matched against a session's
	State    PRState
	CI       CIState
	Conflict bool   // DIRTY or BEHIND: it cannot merge as it stands
	Fork     bool   // opened from another repository: its branch name is not ours
	Head     string // the head commit, which says whether a merged PR is this work
	Draft    bool   // opened as a draft: not ready to be read as work offered
	Updated  time.Time
}

// ghPR is one element of `gh pr list --json` with the fields ListPRs asks for.
type ghPR struct {
	Number            int       `json:"number"`
	Title             string    `json:"title"`
	HeadRefName       string    `json:"headRefName"`
	HeadRefOid        string    `json:"headRefOid"`
	IsCrossRepository bool      `json:"isCrossRepository"`
	State             string    `json:"state"`
	MergeStateStatus  string    `json:"mergeStateStatus"`
	StatusCheckRollup []check   `json:"statusCheckRollup"`
	IsDraft           bool      `json:"isDraft"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

// check is a CheckRun (status, conclusion) or a StatusContext (state); the
// rollup mixes the two.
type check struct {
	Typename   string `json:"__typename"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

// Fold turns `gh pr list --json` output into PRs.
//
//	prs, err := forge.Fold(out)
func Fold(raw []byte) ([]PR, error) {
	var in []ghPR
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("forge: reading gh's pull request list: %w", err)
	}
	out := make([]PR, len(in))
	for i, p := range in {
		out[i] = PR{
			Number:   p.Number,
			Title:    p.Title,
			Branch:   p.HeadRefName,
			State:    stateOf(p.State),
			CI:       rollup(p.StatusCheckRollup),
			Conflict: p.MergeStateStatus == "DIRTY" || p.MergeStateStatus == "BEHIND",
			Fork:     p.IsCrossRepository,
			Head:     p.HeadRefOid,
			Draft:    p.IsDraft,
			Updated:  p.UpdatedAt,
		}
	}
	return out, nil
}

func stateOf(s string) PRState {
	switch s {
	case "MERGED":
		return Merged
	case "CLOSED":
		return Closed
	}
	return Open
}

// rollup is a precedence, because the card has one cell: anything failing
// wins, then anything still running, then passing. UNKNOWN is never passing:
// a check that has not said is running, not green (Orca #18484).
func rollup(checks []check) CIState {
	if len(checks) == 0 {
		return CINone
	}
	for _, c := range checks {
		if failed(c) {
			return CIFailing
		}
	}
	for _, c := range checks {
		if running(c) {
			return CIRunning
		}
	}
	return CIPassing
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
