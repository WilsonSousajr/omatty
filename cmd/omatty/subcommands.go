// The subcommands: everything omatty does before the TUI starts, each one
// a thin verb over a typed library call (invariant 10). registeredRoots lives
// here although wiring.go's projectProposer also calls it: the CLI is its
// first caller.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/WilsonSousajr/omatty/internal/cli"
	"github.com/WilsonSousajr/omatty/internal/infra/config"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/infra/vcs"
	"github.com/WilsonSousajr/omatty/internal/service/discovery"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// dispatch runs a subcommand. `add` registers a repository and `rm` forgets
// one; `new` creates a session, with a branch argument meaning "in a fresh
// worktree"; `sessions --json` and `status --json` read them (#653).
func dispatch(cmd string, args []string, home string, cfg config.Config, store sessions.StateStore) error {
	switch cmd {
	case "add":
		return addProject(store, args)
	case "rm":
		return cli.Remove(context.Background(), os.Stdout, store, args)
	case "new":
		return newSession(store, cfg, args)
	case "discover":
		return discoverProjects(store, home, os.Stdin)
	case "adopt":
		return cli.Adopt(context.Background(), os.Stdout, os.Stdin, adoptPorts(store, home), args)
	case "sessions", "status":
		return readCommand(cmd, args, home, store, os.Stdout)
	default:
		return dispatchSettings(cmd, args, store, newRouter(cfg, vcs.NewCLI()).ListPRs)
	}
}

// dispatchSettings runs the per-project settings subcommands, and owns the
// unknown-command error. Split from dispatch to keep each inside funlen: they
// share a shape - resolve a project, then show, set or clear one field of it.
func dispatchSettings(cmd string, args []string, store sessions.StateStore, prs cli.PRLister) error {
	switch cmd {
	case "gate":
		return gateCommand(store, args, os.Stdin, prs)
	case "carry":
		return carryCommand(store, args)
	default:
		return fmt.Errorf(
			"unknown command %q (want add, rm, new, discover, adopt, gate, carry, sessions, status, --version, or no argument)", cmd)
	}
}

// adoptPorts is what `omatty adopt` reads and writes through: the real git
// and claude's transcript store under home.
func adoptPorts(store sessions.StateStore, home string) cli.AdoptPorts {
	git := vcs.NewCLI()
	return cli.AdoptPorts{Store: store, Git: git, Branches: git.Contextual(),
		TranscriptsDir: paths.TranscriptsDir(home), Prompts: status.PromptText}
}

// discoverProjects lists the repositories claude has been used in and
// registers the ones the operator picks. stdout is free here: discover runs
// before the TUI starts, which is what report exists for (invariant 5).
func discoverProjects(store sessions.StateStore, home string, in io.Reader) error {
	cands, err := proposeProjects(store, home)
	if err != nil {
		return err
	}
	if len(cands) == 0 {
		report("no repositories found in " + paths.TranscriptsDir(home))
		return nil
	}
	for _, line := range discovery.List(cands, time.Now()) {
		report(line)
	}
	report("")
	report("register which? (numbers, or `all`, or enter for none)")
	picked, err := discovery.Choose(cands, cli.ReadLine(in))
	if err != nil {
		return err
	}
	return registerAll(store, picked)
}

// proposeProjects is the scan: what claude has been used in, minus what
// state.json already holds.
func proposeProjects(store sessions.StateStore, home string) ([]discovery.Candidate, error) {
	roots, err := registeredRoots(store)
	if err != nil {
		return nil, err
	}
	return discovery.Propose(paths.TranscriptsDir(home), vcs.NewCLI(), roots)
}

// registeredRoots is what state.json already holds, so discovery does not
// offer a repository that can only fail on commit (#91).
func registeredRoots(store sessions.StateStore) ([]string, error) {
	st, err := store.Load(context.Background())
	if err != nil {
		return nil, err
	}
	roots := make([]string, 0, len(st.Projects))
	for _, p := range st.Projects {
		roots = append(roots, p.Root)
	}
	return roots, nil
}

// registerAll reports what sessions.RegisterAll did with each pick. The loop
// itself lives there, shared with the TUI picker: cmd/ holds no logic
// (invariant 10), and two copies of a collision policy drift (#91).
func registerAll(store sessions.StateStore, picked []discovery.Candidate) error {
	roots := make([]string, 0, len(picked))
	for _, c := range picked {
		roots = append(roots, c.Root)
	}
	for _, r := range sessions.RegisterAll(context.Background(), store, vcs.NewCLI().Contextual(), roots) {
		if r.Err != nil {
			report("skipped " + r.Root + ": " + r.Err.Error())
			continue
		}
		report("registered " + r.Project.Name + " at " + r.Project.Root)
	}
	return nil
}

func addProject(store sessions.StateStore, args []string) error {
	dir, err := argOrCwd(args)
	if err != nil {
		return err
	}
	p, err := sessions.AddProject(context.Background(), store, vcs.NewCLI().Contextual(), dir)
	if err != nil {
		return err
	}
	report("registered " + p.Name + " at " + p.Root)
	return nil
}

func newSession(store sessions.StateStore, cfg config.Config, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("new: want <project> <title> [branch], got %v", args)
	}
	branch := ""
	if len(args) > 2 {
		branch = args[2]
	}
	c := sessions.NewCreator(vcs.NewCLI().Contextual(), creatorOpts(cfg), uuid.NewString)
	sess, err := sessions.AddSession(context.Background(), store, c, args[0], args[1], branch)
	if err != nil {
		return err
	}
	report("created session " + sess.ID + " in " + sess.Dir)
	return nil
}
