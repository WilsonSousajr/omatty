package hooks_test

import (
	"github.com/WilsonSousajr/omatty/internal/domain/status"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/infra/hooks"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
)

// The bootstrap used to live in cmd, outside the coverage gate, with no test
// (issue #79). It must name the running binary, shell-quoted (#56), and
// register the events it was given.
func TestInstall_WritesTheRunningBinaryPath_issue79(t *testing.T) {
	home := t.TempDir()

	path, err := hooks.Install(stopProfile(), home)
	if err != nil {
		t.Fatalf("InstallHooks() error = %v", err)
	}

	if path != paths.HooksFile(home, "") {
		t.Errorf("path = %q, want %q", path, paths.HooksFile(home, ""))
	}
	exe, _ := os.Executable()
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "'"+exe+"' hook") || !strings.Contains(string(got), `"Stop"`) {
		t.Errorf("hooks.json does not name the running binary and the Stop event:\n%s", got)
	}
}

func TestWriteSettings_CreatesTheFileAndParentDir_issue17(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "hooks.json")

	if err := hooks.WriteSettings(path, []byte(`{"hooks":{}}`)); err != nil {
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
func TestWriteSettings_OverwritesEveryTime_issue17(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, []byte(`{"hooks":{"Stop":"OLD BINARY PATH"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := hooks.WriteSettings(path, []byte(`{"hooks":{}}`)); err != nil {
		t.Fatal(err)
	}

	got, _ := os.ReadFile(path)
	if strings.Contains(string(got), "OLD BINARY PATH") {
		t.Errorf("the stale hooks file was not overwritten:\n%s", got)
	}
}

func TestWriteSettings_UnwritableDirectoryNamesThePath_issue17(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "hooks.json")

	err := hooks.WriteSettings(path, []byte("{}"))

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
func TestWriteSettings_RefusesASymlinkAndLeavesTheTargetAlone_issue58(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(target, []byte(`{"theirs":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "hooks.json")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	err := hooks.WriteSettings(path, []byte(`{"hooks":{}}`))

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
func TestWriteSettings_LeavesNoTempFileBehind_issue58(t *testing.T) {
	dir := t.TempDir()

	if err := hooks.WriteSettings(filepath.Join(dir, "hooks.json"), []byte("{}")); err != nil {
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

// stopProfile is an agent whose settings subscribe to one event, rendered by
// this package's own Render: all Install reads from a profile.
func stopProfile() agent.Profile {
	return agent.Profile{HookEvents: func() []string { return []string{"Stop"} }, RenderSettings: hooks.Render}
}

// One settings file per agent that takes hooks, and none for one that does
// not: claude's at hooks.json, which live detached sessions already name,
// another's beside it (#522).
func TestInstallAll_OneFilePerHookAgent_issue522(t *testing.T) {
	home := t.TempDir()
	hooky := func(name string) agent.Profile {
		p := stopProfile()
		p.Name, p.Command = name, agent.ClaudeCommand
		p.Caps = agent.Caps{Identity: agent.Assigned, Status: agent.StatusHooks}
		p.TranscriptPath = paths.Transcript
		p.Status = fakeAdapter{}
		p.ParseHook = hooks.ParsePayload
		return p
	}
	quiet := agent.Profile{Name: "quiet", Command: agent.ClaudeCommand}
	agents, err := agent.NewCatalog(hooky("claude"), hooky("toy"), quiet)
	if err != nil {
		t.Fatal(err)
	}

	files, err := hooks.InstallAll(agents, home)

	if err != nil {
		t.Fatal(err)
	}
	if files["claude"] != paths.HooksFile(home, "claude") || files["toy"] != paths.HooksFile(home, "toy") {
		t.Errorf("files = %v, want claude's and toy's own", files)
	}
	if _, ok := files["quiet"]; ok {
		t.Errorf("an agent without hooks got a settings file: %v", files)
	}
	for _, f := range []string{files["claude"], files["toy"]} {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("%s was not written: %v", f, err)
		}
		// Invariant 3, generalised: nothing outside omatty's own directory.
		if !strings.HasPrefix(f, paths.Root(home)+string(filepath.Separator)) {
			t.Errorf("%s is outside %s", f, paths.Root(home))
		}
	}
}

// fakeAdapter stands in for a hook agent's status parser, which this package
// never calls; the catalog only requires that one is declared (#520).
type fakeAdapter struct{}

func (fakeAdapter) ParseEntry([]byte) (status.Entry, bool) { return status.Entry{}, false }
func (fakeAdapter) DeriveKind([]status.Entry) (status.Kind, time.Time, bool) {
	return 0, time.Time{}, false
}
func (fakeAdapter) KindOf(status.HookPayload) (status.Kind, bool) { return 0, false }

// An agent whose hooks travel as arguments gets no settings file - codex
// would never read it (#152) - and its arguments, rendered for the running
// binary, instead.
func TestInstallAll_AnArgumentAgentGetsArgsNotAFile_issue152(t *testing.T) {
	home := t.TempDir()
	argy := stopProfile()
	argy.Name, argy.Command = "argy", agent.ClaudeCommand
	argy.Caps = agent.Caps{Identity: agent.Reported, Status: agent.StatusHooks}
	argy.TranscriptPath = paths.Transcript
	argy.Status = fakeAdapter{}
	argy.ParseHook = hooks.ParseCodexPayload
	argy.RenderSettings, argy.RenderArgs = nil, hooks.RenderCodexArgs
	agents, err := agent.NewCatalog(argy)
	if err != nil {
		t.Fatal(err)
	}
	files, err := hooks.InstallAll(agents, home)
	if err != nil || len(files) != 0 {
		t.Errorf("InstallAll = %v, %v; want no file for an argument agent", files, err)
	}
	args, err := hooks.RenderAllArgs(agents)
	if err != nil {
		t.Fatal(err)
	}
	self, _ := os.Executable()
	if got := strings.Join(args["argy"], " "); !strings.Contains(got, "-c hooks.Stop=") || !strings.Contains(got, self) {
		t.Errorf("args = %q, want a Stop hook naming the running binary %s", got, self)
	}
}
