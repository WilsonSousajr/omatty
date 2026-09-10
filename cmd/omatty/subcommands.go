// The subcommands: everything omatty does before the TUI starts, each one
// a thin verb over a typed library call (invariant 10). registeredRoots lives
// here although wiring.go's projectProposer also calls it: the CLI is its
// first caller.

package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/WilsonSousajr/omatty/internal/config"
	"github.com/WilsonSousajr/omatty/internal/discover"
	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// dispatch runs a subcommand. `add` registers a repository and `rm` forgets
// one; `new` creates a session, with a branch argument meaning "in a fresh
// worktree".
func dispatch(cmd string, args []string, home string, cfg config.Config, store *registry.Store) error {
	switch cmd {
	case "add":
		return addProject(store, args)
	case "rm":
		return removeProject(store, args)
	case "new":
		return newSession(store, cfg, args)
	case "discover":
		return discoverProjects(store, home, os.Stdin)
	case "adopt":
		return adoptSessions(store, home, vcs.NewCLI(), args, os.Stdin)
	default:
		return fmt.Errorf("unknown command %q (want add, rm, new, discover, adopt, --version, or no argument)", cmd)
	}
}

// adoptSessions lists the claude sessions in one registered project that omatty
// does not yet hold, and registers the ones the operator picks. The CLI twin of
// the ctrl+o A picker, and the sibling of discoverProjects: one finds
// repositories, this finds sessions inside one (#122).
//
// git is a parameter rather than built here so the flow is testable without a
// real repository; everything else follows discoverProjects exactly.
func adoptSessions(
	store *registry.Store, home string, git discover.Git, args []string, in io.Reader,
) error {
	p, err := namedProject(store, args)
	if err != nil {
		return err
	}
	cands, err := proposeSessions(store, home, git, p)
	if err != nil {
		return err
	}
	if len(cands) == 0 {
		report("no unregistered claude sessions found in " + p.Root)
		return nil
	}
	return chooseAndAdopt(store, p, cands, in)
}

// chooseAndAdopt prints the list, reads the answer, and registers each pick.
func chooseAndAdopt(
	store *registry.Store, p registry.Project, cands []discover.SessionCandidate, in io.Reader,
) error {
	for _, line := range discover.ListSessions(cands, time.Now()) {
		report(line)
	}
	report("")
	report("adopt which? (numbers, or `all`, or enter for none)")
	picked, err := discover.ChooseSessions(cands, readLine(in))
	if err != nil {
		return err
	}
	return adoptAll(store, vcs.NewCLI(), p.Name, picked)
}

// adoptAll reports what registry.AdoptAll did with each pick. The loop is the
// registry's; this one only says so on stdout (invariant 10).
func adoptAll(
	store *registry.Store, git registry.SessionBrancher,
	project string, picked []discover.SessionCandidate,
) error {
	for _, a := range registry.AdoptAll(store, git, project, sessionPicks(picked)) {
		if a.Err != nil {
			report("skipped: " + a.Err.Error())
			continue
		}
		report("adopted " + a.Session.ID + " (" + a.Session.Title + ") in " + a.Session.Dir)
	}
	return nil
}

// sessionPicks narrows candidates to what the registry writes a row from.
func sessionPicks(picked []discover.SessionCandidate) []registry.SessionPick {
	out := make([]registry.SessionPick, 0, len(picked))
	for _, c := range picked {
		out = append(out, registry.SessionPick{ID: c.ID, Title: c.Title, Dir: c.Dir})
	}
	return out
}

// namedProject resolves the project argument, which adopt requires: it acts on
// one project, so a missing name is a usage error rather than a scan of
// everything the operator has ever registered.
func namedProject(store *registry.Store, args []string) (registry.Project, error) {
	if len(args) == 0 {
		return registry.Project{}, fmt.Errorf("adopt: want <project>, got no argument")
	}
	p, err := registry.NamedProject(store, args[0])
	if err != nil {
		return registry.Project{}, fmt.Errorf("adopt: %w", err)
	}
	return p, nil
}

// proposeSessions is the scan: the project's sessions, minus the ones state.json
// already holds.
func proposeSessions(
	store *registry.Store, home string, git discover.Git, p registry.Project,
) ([]discover.SessionCandidate, error) {
	ids, err := registry.KnownSessionIDs(store)
	if err != nil {
		return nil, err
	}
	return discover.ProposeSessions(paths.TranscriptsDir(home), git, p.Root, ids)
}

// discoverProjects lists the repositories claude has been used in and
// registers the ones the operator picks. stdout is free here: discover runs
// before the TUI starts, which is what report exists for (invariant 5).
func discoverProjects(store *registry.Store, home string, in io.Reader) error {
	cands, err := proposeProjects(store, home)
	if err != nil {
		return err
	}
	if len(cands) == 0 {
		report("no repositories found in " + paths.TranscriptsDir(home))
		return nil
	}
	for _, line := range discover.List(cands, time.Now()) {
		report(line)
	}
	report("")
	report("register which? (numbers, or `all`, or enter for none)")
	picked, err := discover.Choose(cands, readLine(in))
	if err != nil {
		return err
	}
	return registerAll(store, picked)
}

// proposeProjects is the scan: what claude has been used in, minus what
// state.json already holds.
func proposeProjects(store *registry.Store, home string) ([]discover.Candidate, error) {
	roots, err := registeredRoots(store)
	if err != nil {
		return nil, err
	}
	return discover.Propose(paths.TranscriptsDir(home), vcs.NewCLI(), roots)
}

// registeredRoots is what state.json already holds, so discovery does not
// offer a repository that can only fail on commit (#91).
func registeredRoots(store *registry.Store) ([]string, error) {
	st, err := store.Load()
	if err != nil {
		return nil, err
	}
	roots := make([]string, 0, len(st.Projects))
	for _, p := range st.Projects {
		roots = append(roots, p.Root)
	}
	return roots, nil
}

// registerAll reports what registry.RegisterAll did with each pick. The loop
// itself lives there, shared with the TUI picker: cmd/ holds no logic
// (invariant 10), and two copies of a collision policy drift (#91).
func registerAll(store *registry.Store, picked []discover.Candidate) error {
	roots := make([]string, 0, len(picked))
	for _, c := range picked {
		roots = append(roots, c.Root)
	}
	for _, r := range registry.RegisterAll(store, vcs.NewCLI(), roots) {
		if r.Err != nil {
			report("skipped " + r.Root + ": " + r.Err.Error())
			continue
		}
		report("registered " + r.Project.Name + " at " + r.Project.Root)
	}
	return nil
}

func addProject(store *registry.Store, args []string) error {
	dir, err := argOrCwd(args)
	if err != nil {
		return err
	}
	p, err := registry.AddProject(store, vcs.NewCLI(), dir)
	if err != nil {
		return err
	}
	report("registered " + p.Name + " at " + p.Root)
	return nil
}

// removeProject is `omatty rm <project>`: the CLI twin of ctrl+o x on an
// empty project's header, and the one surface that needs no cursor (#159).
func removeProject(store *registry.Store, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("rm: want <project>, got %v", args)
	}
	p, err := registry.RemoveProject(store, args[0])
	if err != nil {
		return err
	}
	report("removed " + p.Name + " (the repository at " + p.Root + " is untouched)")
	return nil
}

func newSession(store *registry.Store, cfg config.Config, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("new: want <project> <title> [branch], got %v", args)
	}
	branch := ""
	if len(args) > 2 {
		branch = args[2]
	}
	c := registry.NewCreator(vcs.NewCLI(), creatorOpts(cfg), uuid.NewString)
	sess, err := registry.AddSession(store, c, args[0], args[1], branch)
	if err != nil {
		return err
	}
	report("created session " + sess.ID + " in " + sess.Dir)
	return nil
}
