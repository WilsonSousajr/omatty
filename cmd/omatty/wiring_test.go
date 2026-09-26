package main

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/agent"
	"github.com/WilsonSousajr/omatty/internal/config"
	"github.com/WilsonSousajr/omatty/internal/detach"
	"github.com/WilsonSousajr/omatty/internal/registry"
)

// A wiring test of the kind main_test.go concedes is missing: the configured
// binary reaches the launcher (#44).
// The config's lazy_start reaches the boot, in both directions (#317).
func TestTuiDeps_PassesLazyStart_issue317(t *testing.T) {
	for _, lazy := range []bool{true, false} {
		env := tuiEnv{Home: "/h", Agent: agent.Claude(), HooksFile: "/h/hooks.json", Holder: &detach.Plain{}, Width: 80, Height: 24}
		env.Cfg = config.Defaults("/h")
		env.Cfg.Sessions.LazyStart = lazy

		if got := tuiDeps(env, nil, registry.State{}).LazyStart; got != lazy {
			t.Errorf("config lazy_start = %v reached the boot as %v", lazy, got)
		}
	}
}

// The config's idle_stop reaches the model's sweep (#319).
func TestTuiDeps_PassesIdleStop_issue319(t *testing.T) {
	env := tuiEnv{Home: "/h", Agent: agent.Claude(), HooksFile: "/h/hooks.json", Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults("/h")
	env.Cfg.Sessions.IdleStop = config.Duration(90 * time.Minute)

	if got := tuiDeps(env, nil, registry.State{}).IdleStop; got != 90*time.Minute {
		t.Errorf("config idle_stop = 90m reached the sweep as %v", got)
	}
}

func TestTuiDeps_PassesTheConfiguredClaudeBinToTheLauncher_issue44(t *testing.T) {
	// A short home: tuiDeps touches no file, but with dtach installed the
	// launcher derives a socket path from it and refuses one over 103 bytes,
	// which t.TempDir() exceeds on macOS (#43).
	home := "/h"
	env := tuiEnv{Home: home, Agent: agent.Claude(), HooksFile: filepath.Join(home, "hooks.json"), Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults(home)
	env.Cfg.ClaudeBin = "/opt/claude"
	env.Cfg.Leader = "ctrl+a"

	deps := tuiDeps(env, nil, registry.State{})

	cmd, err := deps.Launch.Command(registry.Session{ID: "id", Dir: home})
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

	got := creatorOpts(cfg)

	if got.WorktreeRoot != "/vol/wt" || got.BaseBranch != "develop" {
		t.Errorf("creatorOpts() = %+v, want /vol/wt forked from develop", got)
	}
}
