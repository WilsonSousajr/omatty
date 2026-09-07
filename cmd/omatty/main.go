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

	"github.com/WilsonSousajr/omatty/internal/hooks"
	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
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

// readLine reads the operator's answer. An unreadable stdin means no answer,
// which is the same as choosing nothing - and so does a blank line, so the
// error needs no branch of its own: TrimSpace gives "" for both.
func readLine(in io.Reader) string {
	line, _ := bufio.NewReader(in).ReadString('\n')
	return strings.TrimSpace(line)
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
