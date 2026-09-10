package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	path := writeConfig(t, home, "leader = \"ctrl+a\"\nclaude_bin = \"/opt/claude\"\nworktree_root = \"/vol/wt\"\nbase_branch = \"develop\"\n[naming]\nmodel = true\n")
	got, err := config.Load(path, home)
	if err != nil {
		t.Fatal(err)
	}
	want := config.Config{Leader: "ctrl+a", ClaudeBin: "/opt/claude", WorktreeRoot: "/vol/wt", BaseBranch: "develop", Naming: config.Naming{Model: true}}
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
