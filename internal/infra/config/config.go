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
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/WilsonSousajr/omatty/internal/infra/forge"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
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
	Leader       string           `toml:"leader"`
	ClaudeBin    string           `toml:"claude_bin"`
	DefaultAgent string           `toml:"default_agent"`
	Agents       map[string]Agent `toml:"agents"`
	WorktreeRoot string           `toml:"worktree_root"`
	BaseBranch   string           `toml:"base_branch"`
	Naming       Naming           `toml:"naming"`
	Gate         Gate             `toml:"gate"`
	Sessions     Sessions         `toml:"sessions"`
	UI           UI               `toml:"ui"`
	Forge        Forge            `toml:"forge"`
}

// Forge is the [forge] table. Its one key, hosts, names a self-hosted forge
// omatty cannot recognise by its hostname (#451):
//
//	[forge.hosts]
//	"git.corp.example" = "gitlab"
//
// It amends M14's "no [forge] section": the poll stays zero-config for every
// host in forge's built-in table, and this exists only for the rest.
type Forge struct {
	Hosts forge.Hosts `toml:"hosts"`
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

// UI is the [ui] table: how omatty draws, as opposed to what it does (#425).
type UI struct {
	// Icons is the glyph set every state is drawn with: IconsPlain, Unicode
	// any terminal font has, or IconsNerd, a Nerd Font's icons. Opt-in,
	// because a Nerd Font glyph without the font is a tofu box - the reason
	// M5 and M8 cut icons, which M15 takes back only behind this key.
	Icons string `toml:"icons"`
}

// The values UI.Icons takes.
const (
	IconsPlain = "plain"
	IconsNerd  = "nerd"
)

// Defaults is the configuration of a machine with no config file.
//
//	cfg := config.Defaults(home)
func Defaults(home string) Config {
	return Config{
		Leader: "ctrl+o", ClaudeBin: "claude", DefaultAgent: "claude", WorktreeRoot: paths.DefaultWorktreeRoot(home),
		Gate: Gate{MaxParallel: 2}, Sessions: Sessions{LazyStart: true}, UI: UI{Icons: IconsPlain},
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
	if err := refuseUntabledHosts(path, md, cfg.Forge.Hosts); err != nil {
		return Config{}, err
	}
	if err := refuseBadValues(path, cfg); err != nil {
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

// refuseBadValues is every check on a value the decoder accepted by type but
// omatty cannot use.
func refuseBadValues(path string, cfg Config) error {
	if err := refuseBlankLeader(path, cfg.Leader); err != nil {
		return err
	}
	if err := refuseUnknownIcons(path, cfg.UI.Icons); err != nil {
		return err
	}
	if err := refuseBlankDefaultAgent(path, cfg.DefaultAgent); err != nil {
		return err
	}
	return refuseBadForgeHosts(path, cfg.Forge.Hosts)
}

// hostName is a host as DNS writes one, which is all a remote's host ever is:
// letters, digits, dots and hyphens, starting and ending with a letter or
// digit. A scheme, a port, a path, an "@" or a trailing dot never matches.
var hostName = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$`)

// refuseBadForgeHosts rejects a key that is not a host name, and two keys that
// name one host - DNS ignores case, so "Git.Corp" and "git.corp" are the same
// host, and which kind won would be map order, changing between polls. A line
// that silently matches nothing, or matches differently each time, is the
// failure #44 exists to prevent.
func refuseBadForgeHosts(path string, hosts forge.Hosts) error {
	seen := map[string]string{}
	for host := range hosts {
		if !hostName.MatchString(host) {
			return fmt.Errorf("config %s: forge.hosts key %q is not a host name, want one such as \"git.corp.example\"", path, host)
		}
		if other, dup := seen[strings.ToLower(host)]; dup {
			return fmt.Errorf("config %s: forge.hosts names %q twice, as %q and %q, want one line per host", path, strings.ToLower(host), other, host)
		}
		seen[strings.ToLower(host)] = host
	}
	return nil
}

// refuseUntabledHosts rejects forge.hosts written as anything but a table. The
// decoder reads an array or a string there as no hosts and no error, which
// would leave the operator's line doing nothing with nothing said. A map that
// did decode was a table, whatever md.Type makes of an odd key inside it.
func refuseUntabledHosts(path string, md toml.MetaData, hosts forge.Hosts) error {
	if hosts != nil || !md.IsDefined("forge", "hosts") || md.Type("forge", "hosts") == "Hash" {
		return nil
	}
	return fmt.Errorf("config %s: forge.hosts is a %s, want a table such as [forge.hosts] \"git.corp.example\" = \"gitlab\"",
		path, strings.ToLower(md.Type("forge", "hosts")))
}

// refuseUnknownIcons rejects a glyph set omatty does not have, rather than
// drawing plain and leaving the operator to wonder why the key did nothing.
func refuseUnknownIcons(path, icons string) error {
	if icons != IconsPlain && icons != IconsNerd {
		return fmt.Errorf("config %s: ui.icons %q is not a glyph set, want %q or %q", path, icons, IconsPlain, IconsNerd)
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
