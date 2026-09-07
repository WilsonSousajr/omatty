package main

import (
	"path/filepath"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/config"
	"github.com/WilsonSousajr/omatty/internal/detach"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// A wiring test of the kind main_test.go concedes is missing: the configured
// binary reaches the launcher (#44).
func TestTuiDeps_PassesTheConfiguredClaudeBinToTheLauncher_issue44(t *testing.T) {
	// A short home: tuiDeps touches no file, but with dtach installed the
	// launcher derives a socket path from it and refuses one over 103 bytes,
	// which t.TempDir() exceeds on macOS (#43).
	home := "/h"
	env := tuiEnv{Home: home, HooksFile: filepath.Join(home, "hooks.json"), Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults(home)
	env.Cfg.ClaudeBin = "/opt/claude"
	env.Cfg.Leader = "ctrl+a"

	deps := tuiDeps(env, nil, registry.State{})

	cmd, err := deps.Launch.Command("id", home)
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Args[0] != "/opt/claude" {
		t.Errorf("launcher runs %q, want /opt/claude", cmd.Args[0])
	}
	if deps.Leader != "ctrl+a" {
		t.Errorf("RunDeps.Leader = %q, want the configured ctrl+a", deps.Leader)
	}
}

// The creator forks from the configured base and places worktrees under the
// configured root, for the TUI and the CLI alike (#44).
func TestCreatorOpts_ComeFromTheConfig_issue44(t *testing.T) {
	cfg := config.Config{WorktreeRoot: "/vol/wt", BaseBranch: "develop"}
	if got := creatorOpts(cfg); got != (registry.CreatorOpts{WorktreeRoot: "/vol/wt", BaseBranch: "develop"}) {
		t.Errorf("creatorOpts() = %+v", got)
	}
}
