// `omatty adopt` and `omatty rm`, moved from cmd/omatty in migration step 7.2
// (#653) with their output unchanged: cmd keeps the flags, cli the flow.

package cli

import (
	"bufio"
	"context"
	"fmt"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"io"
	"strings"
	"time"

	"github.com/WilsonSousajr/omatty/internal/service/discovery"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

// AdoptPorts is what adopt reads and writes through.
type AdoptPorts struct {
	Store sessions.StateStore
	// Git says which repository a transcript's directory belongs to.
	Git discovery.Git
	// Branches says which branch an adopted session is on.
	Branches sessions.SessionBrancher
	// TranscriptsDir is claude's transcript store, where the sessions are found.
	TranscriptsDir string
	// Prompts is the agent's reading of a typed prompt, which titles each row.
	Prompts discovery.PromptText
}

// Adopt lists the claude sessions in one registered project that omatty
// does not yet hold, and registers the ones the operator picks. The CLI twin of
// the ctrl+o A picker, and the sibling of discover: one finds repositories,
// this finds sessions inside one (#122).
//
// git is a port rather than built here so the flow is testable without a
// real repository; everything else follows discover exactly.
//
//	err := cli.Adopt(ctx, os.Stdout, os.Stdin, ports, []string{"my-app"})
func Adopt(ctx context.Context, w io.Writer, in io.Reader, ports AdoptPorts, args []string) error {
	p, err := namedProject(ctx, ports.Store, args)
	if err != nil {
		return err
	}
	cands, err := proposeSessions(ctx, ports, p)
	if err != nil {
		return err
	}
	if len(cands) == 0 {
		say(w, "no unregistered claude sessions found in "+p.Root)
		return nil
	}
	return chooseAndAdopt(ctx, w, in, ports, p, cands)
}

// chooseAndAdopt prints the list, reads the answer, and registers each pick.
func chooseAndAdopt(
	ctx context.Context, w io.Writer, in io.Reader, ports AdoptPorts,
	p session.Project, cands []discovery.SessionCandidate,
) error {
	for _, line := range discovery.ListSessions(cands, time.Now()) {
		say(w, line)
	}
	say(w, "")
	say(w, "adopt which? (numbers, or `all`, or enter for none)")
	picked, err := discovery.ChooseSessions(cands, ReadLine(in))
	if err != nil {
		return err
	}
	return adoptAll(ctx, w, ports, p.Name, picked)
}

// adoptAll reports what sessions.AdoptAll did with each pick. The loop is the
// registry's; this one only says so on stdout (invariant 10).
func adoptAll(
	ctx context.Context, w io.Writer, ports AdoptPorts,
	project string, picked []discovery.SessionCandidate,
) error {
	for _, a := range sessions.AdoptAll(ctx, ports.Store, ports.Branches, project, sessionPicks(picked)) {
		if a.Err != nil {
			say(w, "skipped: "+a.Err.Error())
			continue
		}
		say(w, "adopted "+a.Session.ID+" ("+a.Session.Title+") in "+a.Session.Dir)
	}
	return nil
}

// sessionPicks narrows candidates to what the registry writes a row from.
func sessionPicks(picked []discovery.SessionCandidate) []sessions.SessionPick {
	out := make([]sessions.SessionPick, 0, len(picked))
	for _, c := range picked {
		out = append(out, sessions.SessionPick{ID: c.ID, Title: c.Title, Dir: c.Dir})
	}
	return out
}

// namedProject resolves the project argument, which adopt requires: it acts on
// one project, so a missing name is a usage error rather than a scan of
// everything the operator has ever registered.
func namedProject(ctx context.Context, store sessions.StateStore, args []string) (session.Project, error) {
	if len(args) == 0 {
		return session.Project{}, fmt.Errorf("adopt: want <project>, got no argument")
	}
	p, err := sessions.NamedProject(ctx, store, args[0])
	if err != nil {
		return session.Project{}, fmt.Errorf("adopt: %w", err)
	}
	return p, nil
}

// proposeSessions is the scan: the project's sessions, minus the ones state.json
// already holds.
func proposeSessions(ctx context.Context, ports AdoptPorts, p session.Project) ([]discovery.SessionCandidate, error) {
	ids, err := sessions.KnownSessionIDs(ctx, ports.Store)
	if err != nil {
		return nil, err
	}
	return discovery.ProposeSessions(ports.TranscriptsDir, ports.Git, p.Root, ids, ports.Prompts)
}

// Remove is `omatty rm <project>`: the CLI twin of ctrl+o x on an
// empty project's header, and the one surface that needs no cursor (#159).
//
//	err := cli.Remove(ctx, os.Stdout, store, []string{"my-app"})
func Remove(ctx context.Context, w io.Writer, store sessions.StateStore, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("rm: want <project>, got %v", args)
	}
	p, err := sessions.RemoveProject(ctx, store, args[0])
	if err != nil {
		return err
	}
	say(w, "removed "+p.Name+" (the repository at "+p.Root+" is untouched)")
	return nil
}

// ReadLine reads the operator's answer. An unreadable stdin means no answer,
// which is the same as choosing nothing - and so does a blank line, so the
// error needs no branch of its own: TrimSpace gives "" for both.
//
//	picked := cli.ReadLine(os.Stdin)
func ReadLine(in io.Reader) string {
	line, _ := bufio.NewReader(in).ReadString('\n')
	return strings.TrimSpace(line)
}

// say writes one line of plain-text output. Subcommands run before the TUI
// starts or after it stops, so stdout is theirs (invariant 5).
func say(w io.Writer, line string) {
	_, _ = fmt.Fprintln(w, line)
}
