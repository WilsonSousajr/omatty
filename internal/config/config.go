// Package config reads ~/.omatty/config.toml. Every key is optional; a missing
// file is every default and not an error, which is what a first run must see.
// It is the only package that names a TOML library (AGENTS.md, Dependencies).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/WilsonSousajr/omatty/internal/paths"
)

// Naming is the [naming] table: whether omatty spends a headless agent call
// improving a session's auto-derived title (#127). Off by default, because a
// call the operator did not ask for is money and rate limit they did not
// budget.
type Naming struct {
	Model bool `toml:"model"`
}

// Config is the whole file, already defaulted. Every field is a value the
// caller can use directly: past Load there is no "unset" state, so no caller
// needs a second default of its own (#44).
type Config struct {
	Leader       string `toml:"leader"`
	ClaudeBin    string `toml:"claude_bin"`
	WorktreeRoot string `toml:"worktree_root"`
	BaseBranch   string `toml:"base_branch"`
	Naming       Naming `toml:"naming"`
}

// Defaults is the configuration of a machine with no config file.
//
//	cfg := config.Defaults(home)
func Defaults(home string) Config {
	return Config{Leader: "ctrl+o", ClaudeBin: "claude", WorktreeRoot: paths.DefaultWorktreeRoot(home)}
}

// Load reads path, filling every key the file omits from Defaults(home).
//
//	cfg, err := config.Load(paths.ConfigFile(home), home)
//
// A missing file is Defaults and no error. Anything else - a syntax error, a
// wrong type, a key omatty does not know - is an error naming the file and
// the key, because a silently ignored key is a leader the operator set and
// omatty did not use, with nothing on screen to say so (#44).
func Load(path, home string) (Config, error) {
	cfg := Defaults(home)
	md, err := toml.DecodeFile(path, &cfg)
	if errors.Is(err, fs.ErrNotExist) {
		return Defaults(home), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("config %s: %w", path, err)
	}
	if err := refuseUnknownKeys(path, md); err != nil {
		return Config{}, err
	}
	if err := refuseBlankLeader(path, cfg.Leader); err != nil {
		return Config{}, err
	}
	cfg.WorktreeRoot = expandHome(cfg.WorktreeRoot, home)
	return cfg, nil
}

// knownKeys is the list an unknown-key error offers, so a typo is answered
// with the spelling that would have worked.
const knownKeys = "leader, claude_bin, worktree_root, base_branch, naming.model"

// refuseUnknownKeys turns a typo into an error that names it.
func refuseUnknownKeys(path string, md toml.MetaData) error {
	if keys := md.Undecoded(); len(keys) > 0 {
		return fmt.Errorf("config %s: unknown key %q, want one of %s", path, keys[0].String(), knownKeys)
	}
	return nil
}

// refuseBlankLeader rejects a leader nobody can press. With a session
// focused ctrl+c belongs to claude (invariant 1), so a leader that never
// arrives leaves no way to quit at all. The spelling is bubbletea's
// ("ctrl+a", not "C-a") and cannot be validated here: only ui may import
// bubbletea.
func refuseBlankLeader(path, leader string) error {
	if strings.TrimSpace(leader) == "" {
		return fmt.Errorf("config %s: leader %q is blank, want a key such as \"ctrl+o\"", path, leader)
	}
	return nil
}

// expandHome resolves a leading "~/" against home. A config file is written
// by hand and "~" is what a person types; nothing else in the path is touched.
func expandHome(p, home string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}
