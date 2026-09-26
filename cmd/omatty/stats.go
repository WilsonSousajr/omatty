// `omatty gate <project> --stats`: the two numbers #332 asks for.

package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/tally"
)

// gateStats prints the project's lead time and first-pass gate rate.
//
// A CLI line rather than a card, deliberately. The sidebar is 27 columns and
// the branch name already loses cells to the diffstat (#180); a number read
// occasionally does not belong on a surface watched continuously. It sits under
// `omatty gate` because the gate is what it measures.
//
// The pull requests come from the operator's own `gh`, read-only, and a machine
// without gh simply gets no lead time - the same quiet degradation the cards
// have (#310).
func gateStats(project registry.Project, sessions []registry.Session, prs prLister) error {
	merged, err := mergedPRs(project, prs)
	if err != nil {
		return err
	}
	report("the numbers for " + project.Name + ":")
	for _, line := range statsLines(tally.Of(project, sessions, merged)) {
		report(line)
	}
	return nil
}

// prLister is what gateStats needs of the forge, so the command can be tested
// without one.
type prLister func(repoRoot string) ([]forge.PR, error)

// mergedPRs reads the project's pull requests, or none when there is no gh and
// nothing to ask.
//
// gh missing is not a failure of this command: half the measurement still works,
// and saying so is better than refusing to print the gate rate because the
// forge could not be reached.
func mergedPRs(project registry.Project, prs prLister) ([]forge.PR, error) {
	if prs == nil {
		return nil, nil
	}
	list, err := prs(project.Root)
	if err != nil {
		report("(no pull requests: " + err.Error() + ")")
		return nil, nil
	}
	return list, nil
}

// statsLines renders the numbers the way a person reads them.
//
// Nothing measured says so rather than printing zeroes: a rate of 0% over no
// runs reads as "the gate never passes", and a lead time of 0s as "everything
// ships instantly". Each number also stands without the other, because the
// counters fill up long before the first pull request merges.
func statsLines(n tally.Numbers) []string {
	if !n.Measured() {
		return []string{"  nothing measured yet: no gate has run after a turn, and no session's pull request has merged"}
	}
	return []string{leadTimeLine(n), firstPassLine(n)}
}

// leadTimeLine is session start to pull request merged, averaged.
func leadTimeLine(n tally.Numbers) string {
	if n.Merged == 0 {
		return "  lead time        no merged pull request from a session yet"
	}
	return fmt.Sprintf("  lead time        %s  (mean over %d merged session(s))",
		roundLead(n.LeadTime), n.Merged)
}

// firstPassLine is the share of turn-following gate runs that passed.
func firstPassLine(n tally.Numbers) string {
	if n.GateRuns == 0 {
		return "  first-pass gate  no gate has run after a turn yet"
	}
	return fmt.Sprintf("  first-pass gate  %.0f%%  (%d run(s) after a turn)",
		n.FirstPass*100, n.GateRuns)
}

// roundLead drops the seconds: a lead time is hours and minutes, and the "0s"
// Duration.String leaves on a rounded value spends two cells saying nothing.
func roundLead(d time.Duration) string {
	return strings.TrimSuffix(d.Round(time.Minute).String(), "0s")
}

// reportStats is the --stats branch of `omatty gate`, kept out of gateCommand
// so that command stays a list of flag arms.
func reportStats(store *registry.Store, project registry.Project) error {
	st, err := store.Load()
	if err != nil {
		return err
	}
	return gateStats(project, st.Sessions, gatePRs())
}

// gatePRs is the forge reader --stats uses: the operator's own gh, read-only.
//
// Built here rather than passed through gateCommand's signature, because every
// other form of the command needs no forge at all and `cmd/` stays thin
// (invariant 10). A machine without gh gets no lead time and the gate rate
// still prints.
func gatePRs() prLister { return forge.NewCLI().ListPRs }
