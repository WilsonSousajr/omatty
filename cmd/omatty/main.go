// Command omatty runs the terminal ADE: multiple projects and multiple
// parallel Claude Code sessions in one window.
package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// defaultSize is used until the terminal reports its real dimensions in the
// first WindowSizeMsg.
const (
	defaultWidth  = 80
	defaultHeight = 24
)

func main() {
	if err := run(); err != nil {
		slog.Error("omatty exited", "err", err)
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
	state, err := registry.NewStore(paths.StateFile(home)).Load()
	if err != nil {
		return err
	}
	launcher := supervisor.NewLauncher("claude", paths.HooksFile(home))
	return ui.Run(state, launcher, termwrap.Start, defaultWidth, defaultHeight)
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
