// Package forge is what omatty knows about a forge's work, independent of the
// forge: a pull request and its CI and review state, an issue, one item in
// full with its comments and checks, and how the forge names things (Label).
// Reading any of it from gh, glab, az, tea or a REST API is
// internal/infra/forge's business; this package is the vocabulary it returns.
//
//	pr := forge.PR{Number: 402, State: forge.Open, CI: forge.CIPassing}
//	ref := forge.GitHub.Ref(pr.Number) // "#402"
package forge

import "time"

// PRState is where a pull request stands.
type PRState int

// CIState is a pull request's checks rolled up into the one cell a card has.
type CIState int

// PR is one pull request as a session card needs it, and as the tracker's list
// draws it (#393).
type PR struct {
	Number   int
	Title    string // empty on a finished one: only the open set is asked for it
	Branch   string // the head branch, matched against a session's
	Base     string // the branch it would merge into: where protection is read (#598)
	State    PRState
	CI       CIState
	Conflict bool   // DIRTY or BEHIND: it cannot merge as it stands
	Fork     bool   // opened from another repository: its branch name is not ours
	Head     string // the head commit, which says whether a merged PR is this work
	Draft    bool   // opened as a draft: not ready to be read as work offered
	Updated  time.Time
	// MergedAt is when it merged, zero for one that has not (#332). The thing
	// itself rather than Updated as a proxy for it: a merged pull request can be
	// commented on afterwards, which moves Updated and not the merge.
	MergedAt time.Time
	Review   Review // what review asks of it, from reviewDecision (#432)
}

// Review is a pull request's review state as gh reports it. ReviewNone is a
// repository that asks for no review, or an answer that did not say - never
// read as approved.
type Review int

// The three states gh reports.
const (
	Open PRState = iota
	Merged
	Closed
)

// The rollup's four answers, in rising precedence.
const (
	CINone    CIState = iota // no checks reported at all
	CIPassing                // every check finished, none failed
	CIRunning                // nothing failed yet, something still running
	CIFailing                // at least one check failed
)

// The review states, in gh's reviewDecision.
const (
	ReviewNone     Review = iota
	ReviewRequired        // REVIEW_REQUIRED: nobody has approved yet
	ReviewApproved        // APPROVED
	ReviewChanges         // CHANGES_REQUESTED
)
