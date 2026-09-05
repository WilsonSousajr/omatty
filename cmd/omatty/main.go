// Command omatty runs the terminal ADE: multiple projects and multiple
// parallel Claude Code sessions in one window.
//
// Usage:
//
//	omatty                            run the TUI
//	omatty add [dir]                  register the repository containing dir
//	omatty discover                   register from the repositories claude knows
//	omatty adopt <project>            register claude sessions already in that project
//	omatty new <project> <title> [branch]  create a session
//	omatty hook                       forward a claude hook event (internal)
//
// A branch argument puts the session in a fresh worktree; without one it runs
// in the project's main checkout.
package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/WilsonSousajr/omatty/internal/detach"
	"github.com/WilsonSousajr/omatty/internal/discover"
	"github.com/WilsonSousajr/omatty/internal/hooks"
	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/vcs"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func main() {
	// Invariant 11: the hook runs before anything that can fail or print. A
	// missing HOME or an unwritable log directory must not reach claude as a
	// non-zero exit or a byte of output (issue #54).
	if len(os.Args) > 1 && os.Args[1] == "hook" {
		runHook()
		return
	}
	if err := run(); err != nil {
		slog.Error("omatty exited", "err", err)
		// Subcommands report to the operator; the TUI owns stdout only while
		// it is running, and by here it has stopped.
		_, _ = fmt.Fprintln(os.Stderr, "omatty:", err)
		os.Exit(1)
	}
}

// runHook is the whole of `omatty hook`. Every error and panic is swallowed
// here rather than logged: the log file is the one thing this path must not
// depend on.
func runHook() {
	defer func() { _ = recover() }()
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	_ = hooks.Report(os.Stdin, paths.HookSocket(home), time.Second)
}

func run() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if err := openLog(home); err != nil {
		return err
	}
	store := registry.NewStore(paths.StateFile(home))
	if len(os.Args) < 2 {
		return runTUI(home, store)
	}
	return dispatch(os.Args[1], os.Args[2:], home, store)
}

// dispatch runs a subcommand. `add` registers a repository; `new` creates a
// session, with a branch argument meaning "in a fresh worktree".
func dispatch(cmd string, args []string, home string, store *registry.Store) error {
	switch cmd {
	case "add":
		return addProject(store, args)
	case "new":
		return newSession(store, home, args)
	case "discover":
		return discoverProjects(store, home, os.Stdin)
	case "adopt":
		return adoptSessions(store, home, vcs.NewCLI(), args, os.Stdin)
	default:
		return fmt.Errorf("unknown command %q (want add, new, discover, adopt, or no argument)", cmd)
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
	return adoptAll(store, p.Name, picked)
}

// adoptAll registers each pick, carrying on past a failure so one bad row does
// not abandon the rest of a bulk pick - RegisterAll's rule, for sessions (#91).
func adoptAll(store *registry.Store, project string, picked []discover.SessionCandidate) error {
	for _, c := range picked {
		sess, err := registry.AdoptSession(store, c.ID, project, c.Title, c.Dir)
		if err != nil {
			report("skipped " + c.ID + ": " + err.Error())
			continue
		}
		report("adopted " + sess.ID + " (" + sess.Title + ") in " + sess.Dir)
	}
	return nil
}

// namedProject resolves the project argument, which adopt requires: it acts on
// one project, so a missing name is a usage error rather than a scan of
// everything the operator has ever registered.
func namedProject(store *registry.Store, args []string) (registry.Project, error) {
	if len(args) == 0 {
		return registry.Project{}, fmt.Errorf("adopt: want <project>, got no argument")
	}
	st, err := store.Load()
	if err != nil {
		return registry.Project{}, err
	}
	for _, p := range st.Projects {
		if p.Name == args[0] {
			return p, nil
		}
	}
	return registry.Project{}, fmt.Errorf("adopt: no project named %q is registered", args[0])
}

// proposeSessions is the scan: the project's sessions, minus the ones state.json
// already holds.
func proposeSessions(
	store *registry.Store, home string, git discover.Git, p registry.Project,
) ([]discover.SessionCandidate, error) {
	ids, err := registeredSessionIDs(store)
	if err != nil {
		return nil, err
	}
	return discover.ProposeSessions(paths.TranscriptsDir(home), git, p.Root, ids)
}

// registeredSessionIDs is what state.json already holds, so adoption does not
// offer a session that can only fail on commit (#91, #122).
func registeredSessionIDs(store *registry.Store) ([]string, error) {
	st, err := store.Load()
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(st.Sessions))
	for _, s := range st.Sessions {
		ids = append(ids, s.ID)
	}
	return ids, nil
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

// readLine reads the operator's answer. An unreadable stdin means no answer,
// which is the same as choosing nothing - and so does a blank line, so the
// error needs no branch of its own: TrimSpace gives "" for both.
func readLine(in io.Reader) string {
	line, _ := bufio.NewReader(in).ReadString('\n')
	return strings.TrimSpace(line)
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

func newSession(store *registry.Store, home string, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("new: want <project> <title> [branch], got %v", args)
	}
	branch := ""
	if len(args) > 2 {
		branch = args[2]
	}
	c := registry.NewCreator(vcs.NewCLI(), home, uuid.NewString)
	sess, err := registry.AddSession(store, c, args[0], args[1], branch)
	if err != nil {
		return err
	}
	report("created session " + sess.ID + " in " + sess.Dir)
	return nil
}

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
	return ui.Run(tuiDeps(home, store, state, hooksFile, w, h))
}

// tuiDeps wires the TUI's dependencies: the launcher, the terminal factory,
// and the typed functions that reach git and the registry on ui's behalf,
// because ui may do neither itself (invariants 4 and 10).
func tuiDeps(
	home string, store *registry.Store, state registry.State, hooksFile string, w, h int,
) ui.RunDeps {
	git := vcs.NewCLI()
	// One holder, used twice: it wraps each launch and it ends an archived
	// session's claude. Two would mean two PATH lookups that could disagree.
	holder := detach.New(home)
	deps := ui.RunDeps{
		Home: home, State: state, Width: w, Height: h,
		Stop:    holder.Stop,
		Notice:  persistNotice(holder.Persists()),
		Launch:  supervisor.NewLauncher("claude", hooksFile, home, holder),
		Factory: termwrap.Start,
		Create:  sessionCreator(home, store),
		Diff:    review.NewSource(git).Load,
		Files:   git.ListFiles,
	}
	return withStoreDeps(deps, store, home, git)
}

// withStoreDeps adds the dependencies that close over the registry store: the
// lifecycle commands (#40, #41) and the two pickers that propose from claude's
// own transcript store (#91, #122).
//
// Split from tuiDeps because the one list ran past the twenty-line limit when
// adoption arrived. The seam is where it is because these all share the store,
// and the fields above share nothing but the window.
func withStoreDeps(
	deps ui.RunDeps, store *registry.Store, home string, git *vcs.CLI,
) ui.RunDeps {
	return withPickerDeps(withLifecycleDeps(deps, store, git), store, home, git)
}

// withLifecycleDeps adds rename, archive and worktree removal (#40, #41).
func withLifecycleDeps(deps ui.RunDeps, store *registry.Store, git *vcs.CLI) ui.RunDeps {
	deps.Rename = sessionRenamer(store)
	deps.Archive = sessionArchiver(store)
	deps.RemoveWorktree = git.RemoveWorktree
	return deps
}

// withPickerDeps adds the project picker (#91) and the adoption picker (#122).
func withPickerDeps(
	deps ui.RunDeps, store *registry.Store, home string, git *vcs.CLI,
) ui.RunDeps {
	deps.Discover = projectProposer(store, home, git)
	deps.AddProject = projectRegistrar(store, git)
	deps.AdoptPropose = sessionProposer(store, home, git)
	deps.AdoptCommit = sessionAdopter(store)
	return deps
}

// persistNotice is what the footer says at startup when nothing is holding the
// sessions, and nothing at all when something is.
//
// It names the command that fixes it because the alternative is a warning an
// operator cannot act on: dtach is an unusual enough dependency that "sessions
// will not survive quit" alone does not imply what to install (#43).
func persistNotice(persists bool) string {
	if persists {
		return ""
	}
	return "dtach not found: sessions will not survive quit (brew install dtach)"
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
		ids, err := registeredSessionIDs(store)
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

// sessionAdopter adapts registry.AdoptSession to ui.AdoptCommitFunc, reporting
// one result per pick in the order given so the picker can name the row that
// failed rather than the batch.
func sessionAdopter(store *registry.Store) ui.AdoptCommitFunc {
	return func(project string, picks []ui.SessionProposal) []error {
		errs := make([]error, 0, len(picks))
		for _, p := range picks {
			_, err := registry.AdoptSession(store, p.ID, project, p.Title, p.Dir)
			errs = append(errs, err)
		}
		return errs
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
	c := registry.NewCreator(vcs.NewCLI(), home, uuid.NewString)
	return func(project, title, branch string) (registry.Session, error) {
		if project == "" {
			return registry.Session{}, fmt.Errorf("no project selected; run `omatty add <dir>` first")
		}
		return registry.AddSession(store, c, project, title, branch)
	}
}

func argOrCwd(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	return os.Getwd()
}

// report writes plain-text CLI output. Subcommands exit before the TUI
// starts, so stdout is theirs; forbidigo bans fmt.Print* to keep invariant 5
// enforceable, hence the explicit writer.
func report(line string) {
	_, _ = fmt.Fprintln(os.Stdout, line)
}

// windowSize is the real terminal size, so sessions are born at the right
// width (issue #51). Off a tty there is nothing to measure; the default is
// logged and used, and onResize ignores the 0x0 bubbletea then reports
// (issue #74).
func windowSize() (int, int) {
	w, h, err := termwrap.WindowSize(os.Stdout)
	if err != nil {
		slog.Warn("terminal size unavailable; sessions start at the default",
			"err", err, "width", ui.DefaultWidth, "height", ui.DefaultHeight)
		return ui.DefaultWidth, ui.DefaultHeight
	}
	return w, h
}

// openLog points slog at a file. Invariant 5: stdout belongs to the TUI, so
// a stray write there would corrupt the screen.
func openLog(home string) error {
	dir := paths.LogDir(home)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "omatty.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(f, nil)))
	return nil
}
