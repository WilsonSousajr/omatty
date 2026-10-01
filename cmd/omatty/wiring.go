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

	"github.com/WilsonSousajr/omatty/internal/discover"
	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/agentcli"
	"github.com/WilsonSousajr/omatty/internal/infra/config"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/infra/forge"
	"github.com/WilsonSousajr/omatty/internal/infra/fsread"
	"github.com/WilsonSousajr/omatty/internal/infra/gateexec"
	"github.com/WilsonSousajr/omatty/internal/infra/hooks"
	"github.com/WilsonSousajr/omatty/internal/infra/hookserver"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	statestore "github.com/WilsonSousajr/omatty/internal/infra/store"
	"github.com/WilsonSousajr/omatty/internal/infra/transcript"
	"github.com/WilsonSousajr/omatty/internal/infra/vcs"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/service/status"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"io"
)

func runTUI(home string, cfg config.Config, store sessions.StateStore) error {
	state, err := store.Load(context.Background())
	if err != nil {
		return err
	}
	// Claude is the only profile today; an empty name resolves to it (#46).
	profile, err := lookupAgent("")
	if err != nil {
		return err
	}
	hooksFile, err := hooks.Install(profile, home)
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

// shipFuncs is #331's push, open and merge: git for the worktree and the push,
// the project's forge for the pull request. omatty's first write to the forge,
// and it happens only on a keypress, on one session, after a person has read
// the verdict.
func shipFuncs(src *review.Source, git *vcs.CLI, fg *forge.Router) ui.ShipFuncs {
	return ui.ShipFuncs{
		Shippable:       src.Shippable,
		Push:            git.Push,
		CreatePR:        fg.CreatePR,
		MergePR:         fg.MergePR,
		BranchProtected: fg.BranchProtected,
	}
}

// newRouter is the one forge reader a run uses (#452): each project's forge is
// read off its origin by git, with the operator's [forge.hosts] before the
// built-in table.
func newRouter(cfg config.Config, git *vcs.CLI) *forge.Router {
	return forge.NewRouter(forge.Options{Remote: git.RemoteURL, Hosts: cfg.Forge.Hosts})
}

// tuiDeps wires the TUI's dependencies: the launcher, the terminal factory,
// and the typed functions that reach git and the registry on ui's behalf,
// because ui may do neither itself (invariants 4 and 10).
func tuiDeps(env tuiEnv, store sessions.StateStore, state sessions.State) ui.RunDeps {
	home, hooksFile, w, h := env.Home, env.HooksFile, env.Width, env.Height
	git, holder := vcs.NewCLI(), env.Holder
	src, fg := review.NewSource(git).WithHeads(fsread.Head), newRouter(env.Cfg, git)
	deps := ui.RunDeps{
		Home: home, State: state, Width: w, Height: h, OpenTranscript: openTranscript, ListenHooks: listenHooks,
		RunGate: gateexec.Run, Profiles: fsread.CoverageProfiles{}, Preview: fsread.ReadPreview, Stop: holder.Stop,
		Notice:    holder.Notice(),
		Launch:    sessions.NewLauncher(env.Agent, env.Cfg.ClaudeBin, hooksFile, home, holder),
		Agent:     env.Agent,
		Factory:   termwrap.Start,
		Create:    sessionCreator(env.Cfg, store),
		Leader:    env.Cfg.Leader,
		Name:      sessionNamer(home, env.Agent),
		Diff:      src.Load,
		Stat:      src.Stat,
		Turn:      turnFuncs(src),
		Files:     git.ListFiles,
		Generated: src.Generated, Ship: shipFuncs(src, git, fg),
	}
	return withStoreDeps(withTableDeps(withForgeDeps(deps, fg), env.Cfg), store, home, git, git.Contextual())
}

// withForgeDeps points every forge-backed call at one Router: the pull requests
// a card shows (#310), the open issues the tracker lists (#394), an item, the
// browser and the words the copy uses share a resolved forge per project, a
// bound and the operator's own authentication (#452).
func withForgeDeps(deps ui.RunDeps, fg *forge.Router) ui.RunDeps {
	deps.PRs, deps.Issues, deps.Label = fg.ListPRs, fg.ListIssues, fg.Label
	deps.Item = ui.ForgeItemFuncs{Issue: fg.ViewIssue, PR: fg.ViewPR}
	deps.Browse = ui.ForgeBrowseFuncs{Issue: fg.BrowseIssue, PR: fg.BrowsePR}
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
	deps.NerdIcons = cfg.UI.Icons == config.IconsNerd
	return deps
}

// wiringGit is the slice of vcs.CLI the dependency wiring below needs.
//
// Declared narrow so these adapters can be built with a fake. While they
// demanded the concrete *vcs.CLI not one of them could be called from a test -
// the defect sessions.RepoRooter's own doc records for #91, and the reason
// main_test.go concedes this wiring is covered only by the milestone's PTY
// smoke test (#122).
type wiringGit interface {
	discover.Git
	RemoveWorktree(repoRoot, dir string) error
}

// sessionsGit is the slice of git the session service's ports need. Its
// methods take a context (ADR 0001, migration step 5.4, #653), so it is
// vcs.CLI's Contextual, a second value beside wiringGit rather than a
// widening of it: one type cannot hold RepoRoot both with and without one.
type sessionsGit interface {
	sessions.RepoRooter
	sessions.SessionBrancher
}

// withStoreDeps adds the dependencies that close over the registry store: the
// lifecycle commands (#40, #41) and the two pickers that propose from claude's
// own transcript store (#91, #122).
//
// Split from tuiDeps because the one list ran past the twenty-line limit when
// adoption arrived. The seam is where it is because these all share the store,
// and the fields above share nothing but the window.
func withStoreDeps(
	deps ui.RunDeps, store sessions.StateStore, home string, git wiringGit, sgit sessionsGit,
) ui.RunDeps {
	return withPickerDeps(withLifecycleDeps(deps, store, git), store, home, git, sgit)
}

// withLifecycleDeps adds rename, rebind, archive, worktree removal, project
// removal and the sidebar fold (#40, #41, #159, #316, #505).
func withLifecycleDeps(deps ui.RunDeps, store sessions.StateStore, git wiringGit) ui.RunDeps {
	deps.Rename = sessionRenamer(store)
	deps.Rebind = sessionRebinder(store)
	deps.RenameBranch = branchRenamer(store, vcs.NewCLI().Contextual())
	deps.Archive = sessionArchiver(store)
	deps.RemoveWorktree = git.RemoveWorktree
	deps.RemoveProject = projectRemover(store)
	deps.Fold = projectFolder(store)
	deps.Tally = gateTallier(store)
	return deps
}

// gateTallier adapts sessions.TallyGateRun to ui.TallyFunc (#332).
func gateTallier(store sessions.StateStore) ui.TallyFunc {
	return func(project string, passed bool) error {
		return sessions.TallyGateRun(context.Background(), store, project, passed)
	}
}

// projectRemover adapts sessions.RemoveProject to ui.RemoveProjectFunc (#159).
func projectRemover(store sessions.StateStore) ui.RemoveProjectFunc {
	return func(name string) (sessions.Project, error) {
		return sessions.RemoveProject(context.Background(), store, name)
	}
}

// projectFolder adapts sessions.SetCollapsed to ui.FoldFunc (#505).
func projectFolder(store sessions.StateStore) ui.FoldFunc {
	return func(project string, collapsed bool) error {
		return sessions.SetCollapsed(context.Background(), store, project, collapsed)
	}
}

// withPickerDeps adds the project picker (#91) and the adoption picker (#122).
func withPickerDeps(
	deps ui.RunDeps, store sessions.StateStore, home string, git wiringGit, sgit sessionsGit,
) ui.RunDeps {
	deps.Discover = projectProposer(store, home, git)
	deps.AddProject = projectRegistrar(store, sgit)
	deps.AdoptPropose = sessionProposer(store, home, git)
	deps.AdoptCommit = sessionAdopter(store, sgit)
	return deps
}

// projectProposer adapts discover.Propose to ui.DiscoverFunc.
//
// LastUsed is carried across rather than flattened away: it is what orders the
// list, so dropping it left the picker showing rows in an order it could not
// explain (#91).
func projectProposer(store sessions.StateStore, home string, git discover.Git) ui.DiscoverFunc {
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
func sessionProposer(store sessions.StateStore, home string, git discover.Git) ui.AdoptFunc {
	return func(projectRoot string) ([]ui.SessionProposal, error) {
		ids, err := sessions.KnownSessionIDs(context.Background(), store)
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

// sessionAdopter adapts sessions.AdoptAll to ui.AdoptCommitFunc, reporting one
// result per pick in the order given so the picker can name the row that failed
// rather than the batch - and can start the row the registry actually wrote.
func sessionAdopter(store sessions.StateStore, git sessions.SessionBrancher) ui.AdoptCommitFunc {
	return func(project string, picks []ui.SessionProposal) []sessions.Adoption {
		out := make([]sessions.SessionPick, 0, len(picks))
		for _, p := range picks {
			out = append(out, sessions.SessionPick{ID: p.ID, Title: p.Title, Dir: p.Dir})
		}
		return sessions.AdoptAll(context.Background(), store, git, project, out)
	}
}

// projectRegistrar adapts sessions.RegisterAll to ui.AddProjectFunc.
func projectRegistrar(store sessions.StateStore, git sessions.RepoRooter) ui.AddProjectFunc {
	return func(roots []string) []sessions.Registration {
		return sessions.RegisterAll(context.Background(), store, git, roots)
	}
}

// sessionRenamer adapts sessions.RenameSession to ui.RenameFunc, so the model
// can retitle a session without holding the store (#41).
func sessionRenamer(store sessions.StateStore) ui.RenameFunc {
	return func(sessionID, title string) error {
		return sessions.RenameSession(context.Background(), store, sessionID, title)
	}
}

// sessionRebinder adapts sessions.RebindSession to ui.RebindFunc, so the model
// can follow a /clear onto its new conversation without holding the store
// (#316).
func sessionRebinder(store sessions.StateStore) ui.RebindFunc {
	return func(sessionID, conversation string) error {
		return sessions.RebindSession(context.Background(), store, sessionID, conversation)
	}
}

// branchRenamer adapts sessions.RenameSessionBranch to ui.BranchRenameFunc, so
// the model can name a worktree's branch without holding the store or git
// (#151) - the shape sessionRenamer already has.
func branchRenamer(store sessions.StateStore, git sessions.BranchRenamer) ui.BranchRenameFunc {
	return func(sess sessions.Session, branch string, unstartedOnly bool) (bool, error) {
		return sessions.RenameSessionBranch(context.Background(), store, git, sess, branch, unstartedOnly)
	}
}

// sessionArchiver adapts sessions.RemoveSession to ui.ArchiveFunc, returning
// the row that was actually removed.
//
// RemoveSession re-reads state.json, so its copy is the authoritative one and
// the model's may be stale - a second omatty instance, or a hand-edited
// state.json. Deciding a worktree's fate from the stale copy is how omatty
// would run `git worktree remove --force` on a directory the registry no
// longer marks as a worktree, which is the case this return value exists to
// prevent (#40).
func sessionArchiver(store sessions.StateStore) ui.ArchiveFunc {
	return func(sessionID string) (sessions.Session, error) {
		return sessions.RemoveSession(context.Background(), store, sessionID)
	}
}

// namingTimeout bounds one headless naming call: long enough for a small
// model round trip, short enough that a stalled one is invisible.
const namingTimeout = 10 * time.Second

// modelNamer adapts agentcli.Namer to ui.ModelNameFunc, or returns nil when
// the operator has not opted in (#44, #127). The closer removes the namer's
// working directory; ui.Run cannot, because it receives a func, not the
// Namer.
func modelNamer(cfg config.Config) (ui.ModelNameFunc, func()) {
	if !cfg.Naming.Model {
		return nil, func() {}
	}
	n := agentcli.NewNamer(agentcli.NamerOpts{Bin: cfg.ClaudeBin, Timeout: namingTimeout})
	name := func(prompt string) (string, error) { return n.Name(context.Background(), prompt) }
	return name, func() {
		if err := n.Close(); err != nil {
			slog.Warn("removing the naming directory", "err", err)
		}
	}
}

// sessionNamer adapts discover.FirstPromptTitle to ui.NameFunc, so the model
// can name a session from its transcript without reading one itself (#127).
// The path is the agent's, not paths.Transcript's: claude files a transcript
// under its resolved working directory, which differs behind a symlink (#564).
func sessionNamer(home string, profile agent.Profile) ui.NameFunc {
	return func(sess sessions.Session) (string, error) {
		// The conversation, not the ID: after /clear the row's first
		// transcript is the one it left behind (#316).
		return discover.FirstPromptTitle(profile.TranscriptPath(home, sess.Dir, sess.ConversationID()))
	}
}

// creatorOpts is the one place the config's worktree keys become creator
// options, so the TUI and `omatty new` cannot disagree about where a
// worktree goes or what it forks from (#44).
func creatorOpts(cfg config.Config) sessions.CreatorOpts {
	return sessions.CreatorOpts{WorktreeRoot: cfg.WorktreeRoot, WorktreeDir: paths.WorktreeDir, BaseBranch: cfg.BaseBranch, Carry: statestore.CarryInto}
}

// sessionCreator adapts sessions.AddSession to ui.CreateFunc. The project
// comes from the cursor, so a session created while looking at one repository
// never lands in another.
//
// The session is registered but not started: starting it needs a terminal
// factory inside the running program, which M2 wires up along with status.
func sessionCreator(cfg config.Config, store sessions.StateStore) ui.CreateFunc {
	c := sessions.NewCreator(vcs.NewCLI().Contextual(), creatorOpts(cfg), uuid.NewString)
	return func(project, title, branch string, worktree bool) (sessions.Session, error) {
		if project == "" {
			return sessions.Session{}, fmt.Errorf("no project selected; run `omatty add <dir>` first")
		}
		if worktree {
			return sessions.AddWorktreeSession(context.Background(), store, c, project, title, branch)
		}
		return sessions.AddSession(context.Background(), store, c, project, title, branch)
	}
}

// openTranscript is the watcher's reader: status comes from the transcript
// (invariant 2), and reading the file is infra's business, not the
// watcher's (ADR 0001, step 5.2c, #653).
func openTranscript(path string) status.Transcript { return transcript.NewReader(path) }

// listenHooks is the watcher's hook server. Running a socket server is
// infra's business (ADR 0001, step 5.2d, #653), and a nil *Listener must not
// reach the watcher wrapped in a non-nil io.Closer, so the error path returns
// a plain nil.
func listenHooks(path string, sink chan<- dstatus.HookPayload) (io.Closer, error) {
	l, err := hookserver.Listen(path, sink)
	if err != nil {
		return nil, err
	}
	return l, nil
}
