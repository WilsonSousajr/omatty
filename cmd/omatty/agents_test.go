package main

import (
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// Invariant 9: every session row written before #46 has no agent, and it
// still relaunches.
func TestLookup_AnEmptyNameIsClaude_issue46(t *testing.T) {
	p, err := mustAgents(t).Lookup("")
	if err != nil || p.Name != "claude" {
		t.Fatalf("Lookup(\"\") = %q, %v; want claude", p.Name, err)
	}
	if byName, _ := mustAgents(t).Lookup("claude"); byName.Name != p.Name {
		t.Errorf("Lookup(claude) = %q, want the same profile", byName.Name)
	}
}

func TestLookup_UnknownAgentNamesItAndTheKnownOnes_issue46(t *testing.T) {
	_, err := mustAgents(t).Lookup("codex")
	if err == nil || !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), "claude") {
		t.Fatalf("Lookup(codex) error = %v, want it to name codex and the known profiles", err)
	}
}

// A nil field would panic at session start, not at build.
func TestClaude_EveryProfileFieldIsSet_issue46(t *testing.T) {
	p := claudeProfile()
	if p.Command == nil || p.TranscriptPath == nil || p.HookEvents == nil || p.RenderSettings == nil || p.Status == nil || p.DefaultBin == "" {
		t.Errorf("Claude() has a nil field: %+v", p)
	}
	if names := mustAgents(t).Names(); len(names) != 1 || names[0] != "claude" {
		t.Errorf("Names() = %v, want [claude]", names)
	}
}

// claude is the shape every tier is measured against (#520).
func TestClaude_DerivesFullTier_issue520(t *testing.T) {
	if got := claudeProfile().Caps.Tier(); got != agent.Full {
		t.Errorf("claude's tier = %v, want full (caps %+v)", got, claudeProfile().Caps)
	}
}

// A capability a profile declares and cannot serve would fail at session
// start, not at build: hooks need their events and renderer, a transcript
// its path and parser (#520).
func TestCatalog_EveryDeclaredCapabilityHasItsFunc_issue520(t *testing.T) {
	agents := mustAgents(t)
	for _, name := range agents.Names() {
		p, _ := agents.Lookup(name)
		if p.Caps.Status >= agent.StatusTranscript && (p.TranscriptPath == nil || p.Status == nil) {
			t.Errorf("%s declares a transcript and lacks its path or parser", name)
		}
		if p.Caps.Status == agent.StatusHooks && (p.HookEvents == nil || p.RenderSettings == nil) {
			t.Errorf("%s declares hooks and lacks their events or renderer", name)
		}
	}
}

// Pins the seam against the function it replaced.
func TestClaude_TranscriptPathMatchesPathsTranscript_issue46(t *testing.T) {
	if got, want := claudeProfile().TranscriptPath("/h", "/p/x", "id"), paths.Transcript("/h", "/p/x", "id"); got != want {
		t.Errorf("TranscriptPath = %q, want %q", got, want)
	}
}

// One real line through the adapter proves the delegation is wired rather
// than merely declared.
func TestClaude_StatusDerivesAPromptFromATypedLine_issue46(t *testing.T) {
	s := claudeProfile().Status
	e, ok := s.ParseEntry([]byte(`{"type":"user","timestamp":"2026-09-02T12:00:00Z","message":{"role":"user","content":"hi"}}`))
	if !ok || !e.UserIsPrompt {
		t.Fatalf("ParseEntry = %+v ok=%v, want a typed prompt", e, ok)
	}
	if k, _, ok := s.DeriveKind([]dstatus.Entry{e}); !ok || k != dstatus.PromptSubmitted {
		t.Errorf("DeriveKind = %v ok=%v, want PromptSubmitted", k, ok)
	}
	if k, ok := s.KindOf(dstatus.HookPayload{SessionID: "s", HookEventName: "Stop"}); !ok || k != dstatus.TurnEnded {
		t.Errorf("KindOf(Stop) = %v ok=%v, want TurnEnded", k, ok)
	}
	if len(claudeProfile().HookEvents()) != len(status.HookEventNames()) {
		t.Error("HookEvents is not the listener's own list (#78)")
	}
}

// symlinkedDir makes root/real/proj and returns root/link/proj, which reaches
// it through a symlink, together with the path claude would report for it:
// the kernel's, with every link resolved. t.TempDir is itself behind one on
// macOS (/var is /private/var), so the real path is resolved too.
func symlinkedDir(t *testing.T) (dir, physical string) {
	t.Helper()
	root := t.TempDir()
	target := filepath.Join(root, "real", "proj")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	physical, err := filepath.EvalSymlinks(target)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "link", "proj"), physical
}

// Regression, issue #564: claude names its transcript directory after its
// working directory as the kernel reports it, with symlinks resolved - a
// project registered as /tmp/x writes under -private-tmp-x on macOS. The
// profile slugged the path as registered, so the tailer never found the
// file and a crash restart used --session-id where it had to resume.
func TestClaude_TranscriptPathFollowsASymlinkedDir_issue564(t *testing.T) {
	dir, physical := symlinkedDir(t)
	got := claudeProfile().TranscriptPath("/h", dir, "id")
	if want := paths.Transcript("/h", physical, "id"); got != want {
		t.Errorf("TranscriptPath(%q) = %q, want the resolved directory's %q", dir, got, want)
	}
}

// Regression, issue #564: a session whose directory is reached through a
// symlink has its transcript under the resolved directory's slug, because
// that is where claude writes it. Resuming it is the #36 condition, so
// missing the file restarted it with --session-id, which claude refuses.
// Here since 5.2b (#653): it drives the launcher with the catalog's own
// profile, which only cmd composes.
func TestLauncher_ResumesASessionBehindASymlink_issue564(t *testing.T) {
	home := t.TempDir()
	dir, physical := symlinkedDir(t)
	transcript := paths.Transcript(home, physical, "abc-123")
	if err := os.MkdirAll(filepath.Dir(transcript), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(transcript, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	l := sessions.NewLauncher(catalogFor(t, claudeProfile(), "claude", "/h.json"), home, &detach.Plain{})

	cmd, err := l.Launch(session.Session{ID: "abc-123", Dir: dir})
	if err != nil {
		t.Fatalf("Command error = %v, want nil", err)
	}
	if args := strings.Join(cmd.Argv, " "); !strings.Contains(args, "--resume abc-123") {
		t.Errorf("args %q lack --resume for a session whose transcript is under its resolved directory", args)
	}
}

// catalogFor is a one-profile catalog running bin with hooksFile as its
// settings, for a launcher test (#521, #522).
func catalogFor(t *testing.T, p agent.Profile, bin, hooksFile string) agent.Catalog {
	t.Helper()
	c, err := agent.NewCatalog(p)
	if err != nil {
		t.Fatal(err)
	}
	return c.WithBins(map[string]string{p.Name: bin}).WithHooksFiles(map[string]string{p.Name: hooksFile})
}

// mustAgents is cmd's own catalog, which building must not fail (#521).
func mustAgents(t *testing.T) agent.Catalog {
	t.Helper()
	agents, err := agentCatalog()
	if err != nil {
		t.Fatalf("agentCatalog: %v", err)
	}
	return agents
}

// `omatty hook` with no flag is claude, which is every hooks.json written
// before M17; `--agent <name>` reads that agent's shape; anything else -
// an unknown agent, an agent without hooks, a malformed flag - yields no
// parser, and the hook exits 0 having sent nothing (invariant 11, #522).
func TestHookParser_ResolvesTheAgentsPayloadShape_issue522(t *testing.T) {
	if _, ok := hookParser(nil); !ok {
		t.Error("no flag: want claude's parser")
	}
	if _, ok := hookParser([]string{"--agent", "claude"}); !ok {
		t.Error("--agent claude: want claude's parser")
	}
	for _, args := range [][]string{{"--agent", "nope"}, {"--agent"}, {"--bogus"}, {"--agent", ""}} {
		if _, ok := hookParser(args); ok {
			t.Errorf("hookParser(%q) = ok, want no parser", args)
		}
	}
}

// ctrl+o n's agent step lists every agent in the catalog, each with whether
// its configured binary is installed (#524).
func TestAgentOptions_ListsEveryAgentWithItsInstalledBin_issue524(t *testing.T) {
	agents := mustAgents(t).WithBins(map[string]string{"claude": "/opt/claude"})
	var asked []string
	options := agentOptions(agents, func(bin string) bool {
		asked = append(asked, bin)
		return true
	})()
	if len(options) != 1 || options[0].Name != "claude" || !options[0].Installed {
		t.Errorf("options = %+v, want claude, installed", options)
	}
	if len(asked) != 1 || asked[0] != "/opt/claude" {
		t.Errorf("asked about %v, want the configured /opt/claude", asked)
	}
}

// A default_agent the catalog lacks would register sessions no launch can
// start; omatty refuses to start on it and names it (#524).
func TestCheckDefaultAgent_RefusesAnUnknownOne_issue524(t *testing.T) {
	if err := checkDefaultAgent(mustAgents(t), "claude"); err != nil {
		t.Errorf("claude: %v", err)
	}
	err := checkDefaultAgent(mustAgents(t), "codx")
	if err == nil || !strings.Contains(err.Error(), "codx") || !strings.Contains(err.Error(), "default_agent") {
		t.Errorf("error = %v, want one naming default_agent and codx", err)
	}
}
