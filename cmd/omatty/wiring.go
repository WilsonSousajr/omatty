// The TUI's dependency wiring: the launcher, the terminal factory and the
// typed functions that reach git and the registry on ui's behalf, because ui
// may do neither itself (invariants 4 and 10).

package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/WilsonSousajr/omatty/internal/agent"
	"github.com/WilsonSousajr/omatty/internal/config"
	"github.com/WilsonSousajr/omatty/internal/detach"
	"github.com/WilsonSousajr/omatty/internal/discover"
	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/vcs"
)

func runTUI(home string, cfg config.Config, store *registry.Store) error {
	state, err := store.Load()
	if err != nil {
		return err
	}
	// Claude is the only profile today; an empty name resolves to it (#46).
	profile, err := agent.Lookup("")
	if err != nil {
		return err
	}
	hooksFile, err := supervisor.InstallHooks(profile, home)
	if err != nil {
		return err
	}
	w, h := windowSize()
	env := tuiEnv{Home: home, Cfg: cfg, Agent: profile, HooksFile: hooksFile, Holder: detach.New(home), Width: w, Height: h}
	return runWithNamer(tuiDeps(env, store, state), cfg)
}

// runWithNamer runs the TUI with the opt-in model namer attached, closing
// the namer's working directory on the way out (#127).
func runWithNamer(deps ui.RunDeps, cfg config.Config) error {
	namer, closeNamer := modelNamer(cfg)
	deps.ModelName = namer
	defer closeNamer()
	return ui.Run(deps)
}

// tuiEnv is what the wiring needs before it can build ui.RunDeps: where
// things live, the hooks file claude is given, and the window to start at. A
// struct because the parameter list reached seven, five of them strings, and
// M7's config, naming and agent seams each add one (#136).
type tuiEnv struct {
	Home      string
	Cfg       config.Config
	Agent     agent.Profile
	HooksFile string
	// Holder keeps sessions alive across quit. One holder, used twice: it
	// wraps each launch and it ends an archived session's claude. Two would
	// mean two PATH lookups that could disagree (#43).
	Holder detach.Holder
	Width  int
	Height int
}

// turnFuncs is everything the review column does with a turn baseline: take
// one, diff against it, count what would be discarded, put it back, and drop it
// when the session is archived (#311, #334). Grouped here so tuiDeps stays a
// list of assignments rather than a nested literal.
func turnFuncs(src *review.Source) ui.TurnFuncs {
	return ui.TurnFuncs{
		Snap:   src.SnapTurn,
		Diff:   src.LoadTurn,
		Drop:   src.DropTurn,
		Revert: src.RevertTurn,
		Count:  src.TurnFileCount,
	}
}

// tuiDeps wires the TUI's dependencies: the launcher, the terminal factory,
// and the typed functions that reach git and the registry on ui's behalf,
// because ui may do neither itself (invariants 4 and 10).
func tuiDeps(env tuiEnv, store *registry.Store, state registry.State) ui.RunDeps {
	home, hooksFile, w, h := env.Home, env.HooksFile, env.Width, env.Height
	git, holder := vcs.NewCLI(), env.Holder
	src := review.NewSource(git)
	deps := ui.RunDeps{
		Home: home, State: state, Width: w, Height: h,
		Stop:      holder.Stop,
		Notice:    holder.Notice(),
		Launch:    supervisor.NewLauncher(env.Agent, env.Cfg.ClaudeBin, hooksFile, home, holder),
		Agent:     env.Agent,
		Factory:   termwrap.Start,
		Create:    sessionCreator(env.Cfg, store),
		Leader:    env.Cfg.Leader,
		Name:      sessionNamer(home),
		Diff:      src.Load,
		Stat:      src.Stat,
		Turn:      turnFuncs(src),
		Files:     git.ListFiles,
		Generated: src.Generated,
	}
	return withStoreDeps(withTableDeps(withForgeDeps(deps), env.Cfg), store, home, git)
}

// withForgeDeps points both gh-backed lists at one forge CLI: the pull requests
// a card shows (#310) and the open issues the tracker lists (#394) share a
// binary, a bound and the operator's own authentication.
func withForgeDeps(deps ui.RunDeps) ui.RunDeps {
	gh := forge.NewCLI()
	deps.PRs, deps.Issues = gh.ListPRs, gh.ListIssues
	deps.Item = ui.ForgeItemFuncs{Issue: gh.ViewIssue, PR: gh.ViewPR}
	deps.Browse = gh.Browse
	return deps
}

// withTableDeps copies the config's [gate] and [sessions] tables onto the run.
// Split from tuiDeps when [sessions] pushed it past the length limit (#317,
// #319); the four are all plain values read from one file.
func withTableDeps(deps ui.RunDeps, cfg config.Config) ui.RunDeps {
	// The gate's bound comes from the config; the Runner raises a zero to
	// one, so an old config file without a [gate] section still works.
	deps.GateParallel, deps.GateAuto = cfg.Gate.MaxParallel, cfg.Gate.Auto
	deps.LazyStart = cfg.Sessions.LazyStart
	deps.IdleStop = time.Duration(cfg.Sessions.IdleStop)
	return deps
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

// withLifecycleDeps adds rename, rebind, archive, worktree removal and project
// removal (#40, #41, #159, #316).
func withLifecycleDeps(deps ui.RunDeps, store *registry.Store, git wiringGit) ui.RunDeps {
	deps.Rename = sessionRenamer(store)
	deps.Rebind = sessionRebinder(store)
	deps.RenameBranch = branchRenamer(store, vcs.NewCLI())
	deps.Archive = sessionArchiver(store)
	deps.RemoveWorktree = git.RemoveWorktree
	deps.RemoveProject = projectRemover(store)
	return deps
}

// projectRemover adapts registry.RemoveProject to ui.RemoveProjectFunc (#159).
func projectRemover(store *registry.Store) ui.RemoveProjectFunc {
	return func(name string) (registry.Project, error) {
		return registry.RemoveProject(store, name)
	}
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

// sessionRebinder adapts registry.RebindSession to ui.RebindFunc, so the model
// can follow a /clear onto its new conversation without holding the store
// (#316).
func sessionRebinder(store *registry.Store) ui.RebindFunc {
	return func(sessionID, conversation string) error {
		return registry.RebindSession(store, sessionID, conversation)
	}
}

// branchRenamer adapts registry.RenameSessionBranch to ui.BranchRenameFunc, so
// the model can name a worktree's branch without holding the store or git
// (#151) - the shape sessionRenamer already has.
func branchRenamer(store *registry.Store, git vcs.Git) ui.BranchRenameFunc {
	return func(sess registry.Session, branch string, unstartedOnly bool) (bool, error) {
		return registry.RenameSessionBranch(store, git, sess, branch, unstartedOnly)
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

// namingTimeout bounds one headless naming call: long enough for a small
// model round trip, short enough that a stalled one is invisible.
const namingTimeout = 10 * time.Second

// modelNamer adapts supervisor.Namer to ui.ModelNameFunc, or returns nil when
// the operator has not opted in (#44, #127). The closer removes the namer's
// working directory; ui.Run cannot, because it receives a func, not the
// Namer.
func modelNamer(cfg config.Config) (ui.ModelNameFunc, func()) {
	if !cfg.Naming.Model {
		return nil, func() {}
	}
	n := supervisor.NewNamer(supervisor.NamerOpts{Bin: cfg.ClaudeBin, Timeout: namingTimeout})
	name := func(prompt string) (string, error) { return n.Name(context.Background(), prompt) }
	return name, func() {
		if err := n.Close(); err != nil {
			slog.Warn("removing the naming directory", "err", err)
		}
	}
}

// sessionNamer adapts discover.FirstPromptTitle to ui.NameFunc, so the model
// can name a session from its transcript without reading one itself (#127).
func sessionNamer(home string) ui.NameFunc {
	return func(sess registry.Session) (string, error) {
		// The conversation, not the ID: after /clear the row's first
		// transcript is the one it left behind (#316).
		return discover.FirstPromptTitle(paths.Transcript(home, sess.Dir, sess.ConversationID()))
	}
}

// creatorOpts is the one place the config's worktree keys become creator
// options, so the TUI and `omatty new` cannot disagree about where a
// worktree goes or what it forks from (#44).
func creatorOpts(cfg config.Config) registry.CreatorOpts {
	return registry.CreatorOpts{WorktreeRoot: cfg.WorktreeRoot, BaseBranch: cfg.BaseBranch}
}

// sessionCreator adapts registry.AddSession to ui.CreateFunc. The project
// comes from the cursor, so a session created while looking at one repository
// never lands in another.
//
// The session is registered but not started: starting it needs a terminal
// factory inside the running program, which M2 wires up along with status.
func sessionCreator(cfg config.Config, store *registry.Store) ui.CreateFunc {
	c := registry.NewCreator(vcs.NewCLI(), creatorOpts(cfg), uuid.NewString)
	return func(project, title, branch string, worktree bool) (registry.Session, error) {
		if project == "" {
			return registry.Session{}, fmt.Errorf("no project selected; run `omatty add <dir>` first")
		}
		if worktree {
			return registry.AddWorktreeSession(store, c, project, title, branch)
		}
		return registry.AddSession(store, c, project, title, branch)
	}
}
