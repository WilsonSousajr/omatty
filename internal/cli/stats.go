// `omatty gate <project> --stats`: the two numbers #332 asks for. Moved from
// cmd/omatty in migration step 7.2 (#653), output unchanged.

package cli

import (
	"context"
	"fmt"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"io"
	"strings"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/forge"
	"github.com/WilsonSousajr/omatty/internal/domain/tally"
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
func gateStats(w io.Writer, project session.Project, sessions []session.Session, prs PRLister) error {
	merged, err := mergedPRs(w, project, prs)
	if err != nil {
		return err
	}
	say(w, "the numbers for "+project.Name+":")
	for _, line := range statsLines(tally.Of(project, sessions, merged)) {
		say(w, line)
	}
	return nil
}

// PRLister is what Stats needs of the forge, so the command can be tested
// without one. cmd passes the forge Router's ListPRs.
type PRLister func(repoRoot string) ([]forge.PR, error)

// mergedPRs reads the project's pull requests, or none when there is no gh and
// nothing to ask.
//
// gh missing is not a failure of this command: half the measurement still works,
// and saying so is better than refusing to print the gate rate because the
// forge could not be reached.
func mergedPRs(w io.Writer, project session.Project, prs PRLister) ([]forge.PR, error) {
	if prs == nil {
		return nil, nil
	}
	list, err := prs(project.Root)
	if err != nil {
		say(w, "(no pull requests: "+err.Error()+")")
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

// Stats is the --stats branch of `omatty gate`: project's lead time and
// first-pass gate rate, written to w. Kept out of cmd's gateCommand so that
// command stays a list of flag arms (migration step 7.2, #653).
//
// prs is the forge reader, the TUI's own Router's ListPRs, so a project named
// in [forge.hosts] gets its lead time here too (#452). A machine without the
// forge's tool gets no lead time and the gate rate still prints.
//
//	err := cli.Stats(ctx, os.Stdout, store, project, router.ListPRs)
func Stats(ctx context.Context, w io.Writer, store Store, project session.Project, prs PRLister) error {
	st, err := store.Load(ctx)
	if err != nil {
		return err
	}
	return gateStats(w, project, st.Sessions, prs)
}
