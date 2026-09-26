package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/config"
)

func writeConfig(t *testing.T, home, body string) string {
	t.Helper()
	path := filepath.Join(home, "config.toml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_MissingFileIsEveryDefault_issue44(t *testing.T) {
	home := t.TempDir()
	got, err := config.Load(filepath.Join(home, "none.toml"), home)
	if err != nil {
		t.Fatalf("Load() on a missing file: %v, want nil", err)
	}
	if got != config.Defaults(home) {
		t.Errorf("Load() = %+v, want Defaults %+v", got, config.Defaults(home))
	}
}

func TestLoad_ReadsEveryKey_issue44(t *testing.T) {
	home := t.TempDir()
	path := writeConfig(t, home, "leader = \"ctrl+a\"\nclaude_bin = \"/opt/claude\"\nworktree_root = \"/vol/wt\"\nbase_branch = \"develop\"\n[naming]\nmodel = true\n[gate]\nmax_parallel = 3\nauto = true\n[sessions]\nlazy_start = false\n[ui]\nicons = \"nerd\"\n")
	got, err := config.Load(path, home)
	if err != nil {
		t.Fatal(err)
	}
	want := config.Config{
		Leader: "ctrl+a", ClaudeBin: "/opt/claude", WorktreeRoot: "/vol/wt", BaseBranch: "develop",
		Naming: config.Naming{Model: true}, Gate: config.Gate{MaxParallel: 3, Auto: true},
		Sessions: config.Sessions{LazyStart: false}, UI: config.UI{Icons: config.IconsNerd},
	}
	if got != want {
		t.Errorf("Load() = %+v, want %+v", got, want)
	}
}

func TestLoad_FillsOmittedKeysFromDefaults_issue44(t *testing.T) {
	home := t.TempDir()
	got, err := config.Load(writeConfig(t, home, "leader = \"ctrl+a\"\n"), home)
	if err != nil {
		t.Fatal(err)
	}
	if got.ClaudeBin != "claude" || got.WorktreeRoot != filepath.Join(home, ".omatty", "wt") || got.Naming.Model {
		t.Errorf("omitted keys were not defaulted: %+v", got)
	}
}

func TestLoad_MalformedFileNamesTheFileAndTheKey_issue44(t *testing.T) {
	home := t.TempDir()
	path := writeConfig(t, home, "leader = 3\n")
	_, err := config.Load(path, home)
	if err == nil || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "leader") {
		t.Fatalf("Load() error = %v, want it to name %s and the key leader", err, path)
	}
}

func TestLoad_UnknownKeyIsAnErrorNamingIt_issue44(t *testing.T) {
	home := t.TempDir()
	_, err := config.Load(writeConfig(t, home, "leaderr = \"ctrl+a\"\n"), home)
	if err == nil || !strings.Contains(err.Error(), "leaderr") {
		t.Fatalf("Load() error = %v, want it to name the unknown key leaderr", err)
	}
}

func TestLoad_RefusesABlankLeader_issue44(t *testing.T) {
	home := t.TempDir()
	_, err := config.Load(writeConfig(t, home, "leader = \"  \"\n"), home)
	if err == nil || !strings.Contains(err.Error(), "leader") {
		t.Fatalf("Load() error = %v, want a refusal naming leader", err)
	}
}

func TestLoad_ExpandsATildeInTheWorktreeRoot_issue44(t *testing.T) {
	home := t.TempDir()
	got, err := config.Load(writeConfig(t, home, "worktree_root = \"~/wt\"\n"), home)
	if err != nil {
		t.Fatal(err)
	}
	if got.WorktreeRoot != filepath.Join(home, "wt") {
		t.Errorf("WorktreeRoot = %q, want ~ expanded against %s", got.WorktreeRoot, home)
	}
}

func TestLoad_ADirectoryAtTheConfigPathIsAnError_issue44(t *testing.T) {
	home := t.TempDir()
	dir := filepath.Join(home, "config.toml")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(dir, home); err == nil {
		t.Fatal("Load() on a directory = nil error, want one: a directory is not a missing file")
	}
}

// The opt-in-spend rule from #127: nothing calls a model unless the operator
// wrote it down.
func TestLoad_NamingModelDefaultsOff_issue44(t *testing.T) {
	home := t.TempDir()
	got, err := config.Load(writeConfig(t, home, "[naming]\n"), home)
	if err != nil || got.Naming.Model {
		t.Errorf("Naming.Model = %v err = %v, want false and nil", got.Naming.Model, err)
	}
}

// The gate's parallelism is configurable because the right number depends on
// the machine, and the default is deliberately small: four concurrent
// `go test ./... -race` make a laptop unusable, which would make the gate
// worse than running it by hand (#229).
func TestLoad_gateParallelDefaultsToTwo_issue231(t *testing.T) {
	home := t.TempDir()
	got, err := config.Load(filepath.Join(home, "none.toml"), home)
	if err != nil {
		t.Fatal(err)
	}
	if got.Gate.MaxParallel != 2 {
		t.Errorf("Gate.MaxParallel = %d, want 2 by default", got.Gate.MaxParallel)
	}
}

func TestLoad_readsGateMaxParallel_issue231(t *testing.T) {
	home := t.TempDir()
	path := writeConfig(t, home, "[gate]\nmax_parallel = 4\n")

	got, err := config.Load(path, home)
	if err != nil {
		t.Fatal(err)
	}
	if got.Gate.MaxParallel != 4 {
		t.Errorf("Gate.MaxParallel = %d, want 4", got.Gate.MaxParallel)
	}
}

// Auto is off unless asked for, which is the whole reason it is a key: a test
// suite on every idle costs real time (#233).
func TestLoad_gateAutoIsOffByDefault_issue233(t *testing.T) {
	home := t.TempDir()
	got, err := config.Load(filepath.Join(home, "none.toml"), home)
	if err != nil {
		t.Fatal(err)
	}
	if got.Gate.Auto {
		t.Error("Gate.Auto is on by default, want off")
	}
}

// Lazy start is on unless the operator turns it off: a fleet of idle claudes
// was 2.99 GB on the machine #317 was measured on.
func TestDefaults_LazyStartIsOn_issue317(t *testing.T) {
	if !config.Defaults(t.TempDir()).Sessions.LazyStart {
		t.Error("Defaults().Sessions.LazyStart = false, want true")
	}
}

func TestLoad_LazyStartFalseIsHonoured_issue317(t *testing.T) {
	home := t.TempDir()
	got, err := config.Load(writeConfig(t, home, "[sessions]\nlazy_start = false\n"), home)
	if err != nil {
		t.Fatal(err)
	}
	if got.Sessions.LazyStart {
		t.Error("lazy_start = false was read as true")
	}
}

// idle_stop is off unless asked for: ending a process the operator did not
// ask to end costs a turn if omatty is wrong about "quiet" (#319).
func TestLoad_IdleStopIsOffByDefault_issue319(t *testing.T) {
	home := t.TempDir()
	got, err := config.Load(filepath.Join(home, "none.toml"), home)
	if err != nil || got.Sessions.IdleStop != 0 {
		t.Errorf("Load() with no file = (idle_stop %v, %v), want 0 and no error", got.Sessions.IdleStop, err)
	}
}

// A duration is what a person writes: "90m", not a count of nanoseconds.
func TestLoad_ReadsIdleStopAsADuration_issue319(t *testing.T) {
	home := t.TempDir()
	got, err := config.Load(writeConfig(t, home, "[sessions]\nidle_stop = \"90m\"\n"), home)
	if err != nil {
		t.Fatal(err)
	}
	if time.Duration(got.Sessions.IdleStop) != 90*time.Minute {
		t.Errorf("idle_stop = %v, want 90m", time.Duration(got.Sessions.IdleStop))
	}
}

// A negative threshold or a typo is refused, naming the file, the key and the
// value, rather than silently sweeping everything or nothing (#319).
func TestLoad_RefusesABadIdleStop_issue319(t *testing.T) {
	for _, value := range []string{"-5m", "soon"} {
		home := t.TempDir()
		path := writeConfig(t, home, "[sessions]\nidle_stop = \""+value+"\"\n")

		_, err := config.Load(path, home)

		if err == nil {
			t.Errorf("idle_stop = %q was accepted, want an error", value)
			continue
		}
		for _, want := range []string{path, "idle_stop", value} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("idle_stop = %q: error %q does not name %q", value, err, want)
			}
		}
	}
}

// Icons are plain Unicode unless asked for: a Nerd Font glyph in a terminal
// without one is a tofu box, worse than no icon (#425).
func TestLoad_IconsArePlainByDefault_issue425(t *testing.T) {
	home := t.TempDir()
	got, err := config.Load(filepath.Join(home, "none.toml"), home)
	if err != nil || got.UI.Icons != config.IconsPlain {
		t.Errorf("Load() with no file = (icons %q, %v), want %q and no error", got.UI.Icons, err, config.IconsPlain)
	}
}

func TestLoad_ReadsNerdIcons_issue425(t *testing.T) {
	home := t.TempDir()
	got, err := config.Load(writeConfig(t, home, "[ui]\nicons = \"nerd\"\n"), home)
	if err != nil {
		t.Fatal(err)
	}
	if got.UI.Icons != config.IconsNerd {
		t.Errorf("icons = %q, want %q", got.UI.Icons, config.IconsNerd)
	}
}

// A value omatty has no glyphs for is refused, naming the file, the key, the
// value and what would have worked, rather than silently drawing plain (#44).
func TestLoad_RefusesUnknownIcons_issue425(t *testing.T) {
	home := t.TempDir()
	path := writeConfig(t, home, "[ui]\nicons = \"emoji\"\n")

	_, err := config.Load(path, home)

	if err == nil {
		t.Fatal("icons = \"emoji\" was accepted, want an error")
	}
	for _, want := range []string{path, "ui.icons", "emoji", `"plain"`, `"nerd"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not name %q", err, want)
		}
	}
}
