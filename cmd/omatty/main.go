// Command omatty runs the terminal ADE: multiple projects and multiple
// parallel Claude Code sessions in one window.
//
// Usage:
//
//	omatty                            run the TUI
//	omatty add [dir]                  register the repository containing dir
//	omatty new <project> <title> [branch]  create a session
//
// A branch argument puts the session in a fresh worktree; without one it runs
// in the project's main checkout.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"

	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// defaultWidth and defaultHeight are used until the terminal reports its real
// dimensions in the first WindowSizeMsg.
const (
	defaultWidth  = 80
	defaultHeight = 24
)

func main() {
	if err := run(); err != nil {
		slog.Error("omatty exited", "err", err)
		// Subcommands report to the operator; the TUI owns stdout only while
		// it is running, and by here it has stopped.
		_, _ = fmt.Fprintln(os.Stderr, "omatty:", err)
		os.Exit(1)
	}
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
	default:
		return fmt.Errorf("unknown command %q (want add, new, or no argument)", cmd)
	}
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
	// claude refuses to start when --settings names a missing file, which
	// leaves every session with a dead PTY (issue #31).
	hooks := paths.HooksFile(home)
	if err := supervisor.EnsureHooksFile(hooks); err != nil {
		return err
	}
	launcher := supervisor.NewLauncher("claude", hooks, home)
	return ui.Run(state, launcher, termwrap.Start, defaultWidth, defaultHeight,
		sessionCreator(home, store))
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
