// The TUI's dependency wiring: the launcher, the terminal factory and the
// typed functions that reach git and the registry on ui's behalf, because ui
// may do neither itself (invariants 4 and 10).

package main

import (
	"fmt"

	"github.com/google/uuid"

	"github.com/WilsonSousajr/omatty/internal/detach"
	"github.com/WilsonSousajr/omatty/internal/discover"
	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/vcs"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func runTUI(home string, store *registry.Store) error {
	state, err := store.Load()
	if err != nil {
		return err
	}
	hooksFile, err := supervisor.InstallHooks(home, watcher.HookEventNames())
	if err != nil {
		return err
	}
	w, h := windowSize()
	return ui.Run(tuiDeps(tuiEnv{Home: home, HooksFile: hooksFile, Width: w, Height: h}, store, state))
}

// tuiEnv is what the wiring needs before it can build ui.RunDeps: where
// things live, the hooks file claude is given, and the window to start at. A
// struct because the parameter list reached seven, five of them strings, and
// M7's config, naming and agent seams each add one (#136).
type tuiEnv struct {
	Home      string
	HooksFile string
	Width     int
	Height    int
}

// tuiDeps wires the TUI's dependencies: the launcher, the terminal factory,
// and the typed functions that reach git and the registry on ui's behalf,
// because ui may do neither itself (invariants 4 and 10).
func tuiDeps(env tuiEnv, store *registry.Store, state registry.State) ui.RunDeps {
	home, hooksFile, w, h := env.Home, env.HooksFile, env.Width, env.Height
	git := vcs.NewCLI()
	// One holder, used twice: it wraps each launch and it ends an archived
	// session's claude. Two would mean two PATH lookups that could disagree.
	holder := detach.New(home)
	deps := ui.RunDeps{
		Home: home, State: state, Width: w, Height: h,
		Stop:    holder.Stop,
		Notice:  holder.Notice(),
		Launch:  supervisor.NewLauncher("claude", hooksFile, home, holder),
		Factory: termwrap.Start,
		Create:  sessionCreator(home, store),
		Diff:    review.NewSource(git).Load,
		Files:   git.ListFiles,
	}
	return withStoreDeps(deps, store, home, git)
}

// wiringGit is the slice of vcs.CLI the dependency wiring below needs.
//
// Declared narrow so these adapters can be built with a fake. While they
// demanded the concrete *vcs.CLI not one of them could be called from a test -
// the defect registry.RepoRooter's own doc records for #91, and the reason
// main_test.go concedes this wiring is covered only by the milestone's PTY
// smoke test (#122).
type wiringGit interface {
	registry.RepoRooter
	registry.SessionBrancher
	discover.Git
	RemoveWorktree(repoRoot, dir string) error
}

// withStoreDeps adds the dependencies that close over the registry store: the
// lifecycle commands (#40, #41) and the two pickers that propose from claude's
// own transcript store (#91, #122).
//
// Split from tuiDeps because the one list ran past the twenty-line limit when
// adoption arrived. The seam is where it is because these all share the store,
// and the fields above share nothing but the window.
func withStoreDeps(
	deps ui.RunDeps, store *registry.Store, home string, git wiringGit,
) ui.RunDeps {
	return withPickerDeps(withLifecycleDeps(deps, store, git), store, home, git)
}

// withLifecycleDeps adds rename, archive and worktree removal (#40, #41).
func withLifecycleDeps(deps ui.RunDeps, store *registry.Store, git wiringGit) ui.RunDeps {
	deps.Rename = sessionRenamer(store)
	deps.Archive = sessionArchiver(store)
	deps.RemoveWorktree = git.RemoveWorktree
	return deps
}

// withPickerDeps adds the project picker (#91) and the adoption picker (#122).
func withPickerDeps(
	deps ui.RunDeps, store *registry.Store, home string, git wiringGit,
) ui.RunDeps {
	deps.Discover = projectProposer(store, home, git)
	deps.AddProject = projectRegistrar(store, git)
	deps.AdoptPropose = sessionProposer(store, home, git)
	deps.AdoptCommit = sessionAdopter(store, git)
	return deps
}

// projectProposer adapts discover.Propose to ui.DiscoverFunc.
//
// LastUsed is carried across rather than flattened away: it is what orders the
// list, so dropping it left the picker showing rows in an order it could not
// explain (#91).
func projectProposer(store *registry.Store, home string, git discover.Git) ui.DiscoverFunc {
	return func() ([]ui.Proposal, error) {
		roots, err := registeredRoots(store)
		if err != nil {
			return nil, err
		}
		cands, err := discover.Propose(paths.TranscriptsDir(home), git, roots)
		if err != nil {
			return nil, err
		}
		proposals := make([]ui.Proposal, 0, len(cands))
		for _, c := range cands {
			proposals = append(proposals, ui.Proposal{Name: c.Name, Root: c.Root, LastUsed: c.LastUsed})
		}
		return proposals, nil
	}
}

// sessionProposer adapts discover.ProposeSessions to ui.AdoptFunc.
//
// LastUsed and Dir are carried across rather than flattened away: one orders
// the list and the other is where the adopted session must actually start, and
// they differ for a session that ran in a linked worktree (#122).
func sessionProposer(store *registry.Store, home string, git discover.Git) ui.AdoptFunc {
	return func(projectRoot string) ([]ui.SessionProposal, error) {
		ids, err := registry.KnownSessionIDs(store)
		if err != nil {
			return nil, err
		}
		cands, err := discover.ProposeSessions(paths.TranscriptsDir(home), git, projectRoot, ids)
		if err != nil {
			return nil, err
		}
		proposals := make([]ui.SessionProposal, 0, len(cands))
		for _, c := range cands {
			proposals = append(proposals, ui.SessionProposal{
				ID: c.ID, Title: c.Title, Dir: c.Dir, LastUsed: c.LastUsed,
			})
		}
		return proposals, nil
	}
}

// sessionAdopter adapts registry.AdoptAll to ui.AdoptCommitFunc, reporting one
// result per pick in the order given so the picker can name the row that failed
// rather than the batch - and can start the row the registry actually wrote.
func sessionAdopter(store *registry.Store, git registry.SessionBrancher) ui.AdoptCommitFunc {
	return func(project string, picks []ui.SessionProposal) []registry.Adoption {
		out := make([]registry.SessionPick, 0, len(picks))
		for _, p := range picks {
			out = append(out, registry.SessionPick{ID: p.ID, Title: p.Title, Dir: p.Dir})
		}
		return registry.AdoptAll(store, git, project, out)
	}
}

// projectRegistrar adapts registry.RegisterAll to ui.AddProjectFunc.
func projectRegistrar(store *registry.Store, git registry.RepoRooter) ui.AddProjectFunc {
	return func(roots []string) []registry.Registration {
		return registry.RegisterAll(store, git, roots)
	}
}

// sessionRenamer adapts registry.RenameSession to ui.RenameFunc, so the model
// can retitle a session without holding the store (#41).
func sessionRenamer(store *registry.Store) ui.RenameFunc {
	return func(sessionID, title string) error {
		return registry.RenameSession(store, sessionID, title)
	}
}

// sessionArchiver adapts registry.RemoveSession to ui.ArchiveFunc, returning
// the row that was actually removed.
//
// RemoveSession re-reads state.json, so its copy is the authoritative one and
// the model's may be stale - a second omatty instance, or a hand-edited
// state.json. Deciding a worktree's fate from the stale copy is how omatty
// would run `git worktree remove --force` on a directory the registry no
// longer marks as a worktree, which is the case this return value exists to
// prevent (#40).
func sessionArchiver(store *registry.Store) ui.ArchiveFunc {
	return func(sessionID string) (registry.Session, error) {
		return registry.RemoveSession(store, sessionID)
	}
}

// sessionCreator adapts registry.AddSession to ui.CreateFunc. The project
// comes from the cursor, so a session created while looking at one repository
// never lands in another.
//
// The session is registered but not started: starting it needs a terminal
// factory inside the running program, which M2 wires up along with status.
func sessionCreator(home string, store *registry.Store) ui.CreateFunc {
	c := registry.NewCreator(vcs.NewCLI(), registry.CreatorOpts{WorktreeRoot: paths.DefaultWorktreeRoot(home)}, uuid.NewString)
	return func(project, title, branch string) (registry.Session, error) {
		if project == "" {
			return registry.Session{}, fmt.Errorf("no project selected; run `omatty add <dir>` first")
		}
		return registry.AddSession(store, c, project, title, branch)
	}
}
