// Package config reads ~/.omatty/config.toml. Every key is optional; a missing
// file is every default and not an error, which is what a first run must see.
// It is the only package that names a TOML library (AGENTS.md, Dependencies).
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"reflect"
	"strings"
	"time"

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
	Leader       string   `toml:"leader"`
	ClaudeBin    string   `toml:"claude_bin"`
	WorktreeRoot string   `toml:"worktree_root"`
	BaseBranch   string   `toml:"base_branch"`
	Naming       Naming   `toml:"naming"`
	Gate         Gate     `toml:"gate"`
	Sessions     Sessions `toml:"sessions"`
}

// Sessions is the [sessions] table: when omatty spends a claude process on a
// session (#317).
type Sessions struct {
	// LazyStart boots only the sessions dtach is already holding, and starts
	// the rest when the operator presses enter in their pane. On by default:
	// a claude costs ~225 MB before its first turn, and eleven of them at
	// boot were 2.99 GB on the machine this was measured on, ten idle for
	// days. False starts every session at boot, as omatty did before.
	LazyStart bool `toml:"lazy_start"`
	// IdleStop stops a session that has been quiet this long, exactly as
	// ctrl+o s does: process ended, row kept, enter resumes it. Zero is off,
	// and the default: ending a process the operator did not ask to end costs
	// a turn if omatty is wrong about "quiet", so it is opted into, the
	// argument [gate] auto makes (#233, #319).
	IdleStop Duration `toml:"idle_stop"`
}

// Duration is a config value a person writes as "90m" or "2h", parsed by
// time.ParseDuration. Its own type so the TOML decoder reads a string, and so
// a nonsense value fails in Load, where the error names the file and the key,
// rather than wherever the value is first used (#319).
type Duration time.Duration

// UnmarshalText parses text as a non-negative duration. Zero means off; a
// negative threshold has no meaning and would sweep every session at once.
//
//	var d config.Duration
//	err := d.UnmarshalText([]byte("90m"))
func (d *Duration) UnmarshalText(text []byte) error {
	v, err := time.ParseDuration(string(text))
	if err != nil {
		return fmt.Errorf("duration %q: want one such as \"90m\" or \"2h\": %w", text, err)
	}
	if v < 0 {
		return fmt.Errorf("duration %q is negative, want \"0\" (off) or a positive one such as \"90m\"", text)
	}
	*d = Duration(v)
	return nil
}

// Gate is the [gate] section: how omatty runs a project's own checks (#229).
type Gate struct {
	// MaxParallel bounds how many gates run at once. Small on purpose - four
	// concurrent `go test ./... -race` make a laptop unusable, and a laggy TUI
	// would make the gate worse than running it by hand.
	MaxParallel int `toml:"max_parallel"`
	// Auto runs a session's gate when its turn ends. False by default: a test
	// suite on every idle costs real time and a real fan, so it is opted into
	// rather than out of (#233).
	Auto bool `toml:"auto"`
}

// Defaults is the configuration of a machine with no config file.
//
//	cfg := config.Defaults(home)
func Defaults(home string) Config {
	return Config{
		Leader: "ctrl+o", ClaudeBin: "claude", WorktreeRoot: paths.DefaultWorktreeRoot(home),
		Gate: Gate{MaxParallel: 2}, Sessions: Sessions{LazyStart: true},
	}
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
// with the spelling that would have worked. Derived from Config's toml tags
// rather than written down: the written list never learned M9's [gate]
// table, so a typo there was answered with five keys, none of them the one
// meant (#321).
//
//	knownKeys() // "leader, claude_bin, ..., gate.max_parallel, gate.auto"
func knownKeys() string {
	return strings.Join(tagPaths(reflect.TypeOf(Config{}), ""), ", ")
}

// tagPaths is every key t decodes, in field order, a nested table's keys
// prefixed with its own name the way the decoder spells them.
func tagPaths(t reflect.Type, prefix string) []string {
	var paths []string
	for i := range t.NumField() {
		f := t.Field(i)
		name := prefix + f.Tag.Get("toml")
		if f.Type.Kind() == reflect.Struct {
			paths = append(paths, tagPaths(f.Type, name+".")...)
			continue
		}
		paths = append(paths, name)
	}
	return paths
}

// refuseUnknownKeys turns a typo into an error that names it.
func refuseUnknownKeys(path string, md toml.MetaData) error {
	if keys := md.Undecoded(); len(keys) > 0 {
		return fmt.Errorf("config %s: unknown key %q, want one of %s", path, keys[0].String(), knownKeys())
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
