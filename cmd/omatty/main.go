// Command omatty runs the terminal ADE: multiple projects and multiple
// parallel Claude Code sessions in one window.
//
// Usage:
//
//	omatty                            run the TUI
//	omatty add [dir]                  register the repository containing dir
//	omatty rm <project>               forget a registered project (the repository stays)
//	omatty discover                   register from the repositories claude knows
//	omatty adopt <project>            register claude sessions already in that project
//	omatty new <project> <title> [branch]  create a session
//	omatty gate <project>             show the gate that verifies it, or propose one
//	omatty gate <project> --detect    print the proposal without writing it
//	omatty gate <project> --set       write the proposal without asking
//	omatty gate <project> --clear     forget the gate
//	omatty carry <project> [path...]  files every new worktree of it carries
//	omatty hook                       forward a claude hook event (internal)
//	omatty --version                  print the release this binary was built from
//
// OMATTY_HEAP_PROFILE names a file to write a heap profile to when the TUI
// exits, for diagnosing what a long-running window is holding.
//
// A branch argument puts the session in a fresh worktree; without one it runs
// in the project's main checkout.
package main

import (
	"fmt"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/WilsonSousajr/omatty/internal/infra/config"
	"github.com/WilsonSousajr/omatty/internal/infra/hooks"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	statestore "github.com/WilsonSousajr/omatty/internal/infra/store"
	"github.com/WilsonSousajr/omatty/internal/tui/app"
	"github.com/WilsonSousajr/omatty/internal/tui/terminal"
)

func main() {
	// Invariant 11: the hook runs before anything that can fail or print. A
	// missing HOME or an unwritable log directory must not reach claude as a
	// non-zero exit or a byte of output (issue #54).
	if len(os.Args) > 1 && os.Args[1] == "hook" {
		runHook(os.Args[2:])
		return
	}
	// Ahead of run() because run() loads the config, and a config omatty
	// refuses to start on is exactly when an operator needs to say which
	// build they are reporting against (#134).
	if len(os.Args) > 1 && isVersionFlag(os.Args[1]) {
		report(versionLine(resolveVersion(version, moduleVersion())))
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
// It was agent-blind until M17 (#46), so that no config read or error path
// reached it. `--agent <name>` adds neither: the parser comes from the
// in-memory catalog, and an agent it cannot resolve is a hook that sends
// nothing (invariant 11, #522).
func runHook(args []string) {
	defer func() { _ = recover() }()
	parse, ok := hookParser(args)
	if !ok {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	// The launcher set SessionEnv on the agent and the hook inherited it; it
	// is what ties a /clear's new conversation to its pane (#316).
	_ = hooks.ReportAs(os.Stdin, parse, paths.HookSocket(home), time.Second, os.Getenv(session.SessionEnv))
}

func run() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if err := openLog(home); err != nil {
		return err
	}
	// After openLog so a malformed file is logged, before runTUI so the error
	// reaches stderr rather than the alt screen (invariant 5). runHook never
	// reads it: that path may depend on nothing that can fail (invariant 11).
	cfg, err := config.Load(paths.ConfigFile(home), home)
	if err != nil {
		return err
	}
	slog.Info("config", "leader", cfg.Leader, "claude_bin", cfg.ClaudeBin, "worktree_root", cfg.WorktreeRoot)
	store := statestore.NewStore(paths.StateFile(home))
	if len(os.Args) < 2 {
		// After the TUI has stopped, so the profile describes a settled
		// heap rather than one mid-frame.
		defer writeHeapProfile()
		return runTUI(home, cfg, store)
	}
	return dispatch(os.Args[1], os.Args[2:], home, cfg, store)
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
	w, h, err := terminal.WindowSize(os.Stdout)
	if err != nil {
		slog.Warn("terminal size unavailable; sessions start at the default",
			"err", err, "width", app.DefaultWidth, "height", app.DefaultHeight)
		return app.DefaultWidth, app.DefaultHeight
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
