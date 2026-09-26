// Package tally measures the two numbers #332 asks for: how long a session
// takes to reach a merged pull request, and how often a project's gate passes
// the first time.
//
// omatty's thesis is that the scarce thing is how fast a person can tell whether
// the work is any good (M9). It had never measured that, while holding every
// timestamp needed to - when a session started, when each gate run ended and
// what it returned, and, since #310, when the branch's pull request merged.
//
// Local only. Nothing here leaves the machine, there is no account and no
// telemetry, and the refusal of "cloud, accounts, sync" is untouched.
//
// Two numbers, and a third needs an argument. R9 in `2026-landscape.md` says not
// to widen the surface, so a proposal for another one has to say what decision
// it would change.
//
// It lives in its own package rather than in registry, which would have to
// import forge and lose the instability margin that keeps `watcher -> registry`
// legal, and rather than in gate, which is a stable leaf on purpose. As a leaf
// importing both it is I=1.00 and depends only downwards.
package tally

import (
	"time"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// Numbers is one project's measurement.
type Numbers struct {
	// LeadTime is the mean time from a session being registered to its pull
	// request merging, over the sessions where both are known.
	LeadTime time.Duration
	// Merged is how many sessions that mean is over, so a reader can tell one
	// measurement from a hundred.
	Merged int
	// GateRuns is how many gate runs followed a turn, and FirstPass the share
	// of those that passed, in 0..1.
	GateRuns  int
	FirstPass float64
}

// Measured reports whether anything was measured at all. A rate over no runs is
// not zero percent - that would read as "the gate never passes" - and a mean
// over no sessions is not an instant lead time.
func (n Numbers) Measured() bool { return n.Merged > 0 || n.GateRuns > 0 }

// Of measures project against its sessions and the repository's pull requests.
//
//	n := tally.Of(project, state.Sessions, prs)
//
// The pull requests come from the caller because reading them is `gh`'s job and
// `internal/forge` owns that (invariant 4); passing them in also means this
// package does no I/O and needs none faked.
func Of(project registry.Project, sessions []registry.Session, prs []forge.PR) Numbers {
	n := Numbers{GateRuns: project.GateRuns}
	if project.GateRuns > 0 {
		n.FirstPass = float64(project.GateFirstPass) / float64(project.GateRuns)
	}
	total := time.Duration(0)
	for _, sess := range sessions {
		lead, ok := leadTime(project, sess, prs)
		if !ok {
			continue
		}
		total += lead
		n.Merged++
	}
	if n.Merged > 0 {
		n.LeadTime = total / time.Duration(n.Merged)
	}
	return n
}

// leadTime is how long sess took to reach a merged pull request, and whether
// that is knowable at all.
//
// Three ways it is not: the session belongs to another project, it was written
// before #332 and has no start time, or its branch has no merged pull request.
// Each of those is "not measured" rather than zero, because a zero would pull
// the mean down as though the work had been instant.
func leadTime(project registry.Project, sess registry.Session, prs []forge.PR) (time.Duration, bool) {
	if sess.Project != project.Name || sess.Started.IsZero() || sess.Branch == "" {
		return 0, false
	}
	merged, ok := mergedFor(sess.Branch, prs)
	if !ok {
		return 0, false
	}
	lead := merged.Sub(sess.Started)
	// A merge recorded before the session started is a clock that went
	// backwards or a branch name used twice. Neither is a lead time.
	if lead < 0 {
		return 0, false
	}
	return lead, true
}

// mergedFor is when branch's pull request merged.
//
// `MergedAt`, which `internal/forge` had to start asking gh for: the finished
// field set carried state and not the time, so every merged pull request arrived
// with a zero time and no lead time could ever be computed. Found by running
// --stats against this repository rather than by a test - the fold was right
// about what it was given, and what it was given was missing a field.
//
// Not Updated as a proxy: a merged pull request can be commented on afterwards,
// which moves Updated and not the merge.
func mergedFor(branch string, prs []forge.PR) (time.Time, bool) {
	for _, pr := range prs {
		if pr.Branch == branch && pr.State == forge.Merged && !pr.MergedAt.IsZero() {
			return pr.MergedAt, true
		}
	}
	return time.Time{}, false
}
