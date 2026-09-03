package supervisor_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/supervisor"
)

// The bootstrap used to live in cmd, outside the coverage gate, with no test
// (issue #79). It must name the running binary, shell-quoted (#56), and
// register the events it was given.
func TestInstallHooks_WritesTheRunningBinaryPath_issue79(t *testing.T) {
	home := t.TempDir()

	path, err := supervisor.InstallHooks(home, []string{"Stop"})
	if err != nil {
		t.Fatalf("InstallHooks() error = %v", err)
	}

	if path != paths.HooksFile(home) {
		t.Errorf("path = %q, want %q", path, paths.HooksFile(home))
	}
	exe, _ := os.Executable()
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "'"+exe+"' hook") || !strings.Contains(string(got), `"Stop"`) {
		t.Errorf("hooks.json does not name the running binary and the Stop event:\n%s", got)
	}
}

func TestWriteHooksFile_CreatesTheFileAndParentDir_issue17(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "hooks.json")

	if err := supervisor.WriteHooksFile(path, []byte(`{"hooks":{}}`)); err != nil {
		t.Fatalf("WriteHooksFile() error = %v, want nil", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != `{"hooks":{}}` {
		t.Errorf("file = %q, err = %v; want the content written", got, err)
	}
}

// Replaces TestEnsureHooksFile_DoesNotOverwriteAnExistingFile_issue31. That
// test asserted the file was never rewritten, which was correct only for the
// #31 stub. The file now names the omatty binary by absolute path, which moves
// with `go install`, so a stale path must be replaced (invariant 11 depends on
// the hook actually reaching a running omatty).
func TestWriteHooksFile_OverwritesEveryTime_issue17(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":"OLD BINARY PATH"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := supervisor.WriteHooksFile(path, []byte(`{"hooks":{}}`)); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "OLD BINARY PATH") {
		t.Errorf("the stale hooks file was not overwritten:\n%s", got)
	}
}

func TestWriteHooksFile_UnwritableDirectoryNamesThePath_issue17(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "hooks.json")

	err := supervisor.WriteHooksFile(path, []byte("{}"))

	if err == nil {
		t.Fatal("WriteHooksFile() into a read-only dir returned nil, want an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the offending path", err)
	}
}

// Regression, issue #58 (invariant 3): a symlink at the hooks path was
// followed, so a link planted at ~/.omatty/hooks.json pointing at the user's
// ~/.claude/settings.json made omatty overwrite that file on its next start.
func TestWriteHooksFile_RefusesASymlinkAndLeavesTheTargetAlone_issue58(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(target, []byte(`{"theirs":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "hooks.json")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	err := supervisor.WriteHooksFile(path, []byte(`{"hooks":{}}`))

	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("WriteHooksFile over a symlink = %v, want an error naming %s", err, path)
	}
	got, _ := os.ReadFile(target)
	if string(got) != `{"theirs":true}` {
		t.Errorf("the symlink target was rewritten to %q (invariant 3)", got)
	}
}

// The file is renamed into place, so a claude reading --settings at that
// instant never sees a truncated file (the #31 failure) and no temp file is
// left behind.
func TestWriteHooksFile_LeavesNoTempFileBehind_issue58(t *testing.T) {
	dir := t.TempDir()

	if err := supervisor.WriteHooksFile(filepath.Join(dir, "hooks.json"), []byte("{}")); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "hooks.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("dir holds %v, want only hooks.json", names)
	}
}
