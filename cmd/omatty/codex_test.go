package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
	"github.com/WilsonSousajr/omatty/internal/infra/config"
	"github.com/WilsonSousajr/omatty/internal/infra/detach"
	"github.com/WilsonSousajr/omatty/internal/service/sessions"
)

// The #528 spike's verdict, pinned: codex reports its own id, takes hooks
// and their trust per invocation, reports waiting, resumes by id and marks
// a turn's end (#152).
func TestCodex_DerivesFullTier_issue152(t *testing.T) {
	p := codexProfile()
	want := agent.Caps{Identity: agent.Reported, Status: agent.StatusHooks, Waiting: true, Resume: true, TurnBoundary: true}
	if p.Caps != want || p.Caps.Tier() != agent.Full {
		t.Errorf("codex caps = %+v (tier %v), want %+v at full", p.Caps, p.Caps.Tier(), want)
	}
	if p.RenderSettings != nil || p.RenderArgs == nil {
		t.Error("codex must render its hooks as -c arguments, never a settings file it would not read")
	}
}

// claude stays first - an empty agent name is every row written before
// #46 (invariant 9) - and codex joins it (#152).
func TestAgentCatalog_ClaudeThenCodex_issue152(t *testing.T) {
	if got := strings.Join(mustAgents(t).Names(), ","); got != "claude,codex" {
		t.Errorf("Names() = %s, want claude,codex", got)
	}
}

// `omatty hook --agent codex` reads codex's payload, and drops the
// memories thread's, whose transcript_path is null (#152).
func TestHookParser_CodexDropsTheMemoriesThread_issue152(t *testing.T) {
	parse, ok := hookParser([]string{"--agent", "codex"})
	if !ok {
		t.Fatal("--agent codex: want codex's parser")
	}
	own := `{"session_id":"s1","transcript_path":"/r.jsonl","hook_event_name":"Stop"}`
	if _, ok := parse(strings.NewReader(own)); !ok {
		t.Error("the session's own Stop was dropped")
	}
	memories := `{"session_id":"s2","transcript_path":null,"hook_event_name":"SessionStart","source":"startup"}`
	if _, ok := parse(strings.NewReader(memories)); ok {
		t.Error("the memories thread's SessionStart was accepted")
	}
}

// writeRollout puts a rollout for conversation under store, where codex
// would: a date directory and a launch time omatty cannot predict.
func writeRollout(t *testing.T, store, conversation string) string {
	t.Helper()
	day := filepath.Join(store, "sessions", "2026", "10", "04")
	if err := os.MkdirAll(day, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(day, "rollout-2026-10-04T06-50-16-"+conversation+".jsonl")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The rollout is found by id under ~/.codex, or under $CODEX_HOME when the
// user moved codex's store (#152).
func TestCodex_TranscriptPathFindsTheRollout_issue152(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", "")
	want := writeRollout(t, filepath.Join(home, ".codex"), "c-1")
	if got := codexProfile().TranscriptPath(home, "/w", "c-1"); got != want {
		t.Errorf("under ~/.codex: TranscriptPath = %q, want %q", got, want)
	}
	moved := t.TempDir()
	t.Setenv("CODEX_HOME", moved)
	want = writeRollout(t, moved, "c-2")
	if got := codexProfile().TranscriptPath(home, "/w", "c-2"); got != want {
		t.Errorf("under $CODEX_HOME: TranscriptPath = %q, want %q", got, want)
	}
}

// A conversation codex has not written is no file at all, so the launcher
// starts codex fresh rather than resuming an id codex does not know.
func TestCodex_AnUnwrittenConversationStartsFresh_issue152(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", "")
	p := codexProfile()
	if path := p.TranscriptPath(home, "/w", "pane-1"); fileExists(path) {
		t.Errorf("an unwritten conversation resolved to an existing file %q", path)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// End to end through the launcher: a codex row bound to a conversation
// codex wrote resumes it, with its hooks' -c flags last (#152, invariant 9).
func TestLauncher_ResumesACodexConversationWithItsHooks_issue152(t *testing.T) {
	home := t.TempDir()
	t.Setenv("CODEX_HOME", "")
	writeRollout(t, filepath.Join(home, ".codex"), "c-1")
	agents := mustAgents(t).WithHookArgs(map[string][]string{"codex": {"-c", "hooks.Stop=[]"}})
	l := sessions.NewLauncher(agents, home, &detach.Plain{})

	launch, err := l.Launch(session.Session{ID: "pane-1", Dir: "/w", Agent: "codex", Conversation: "c-1"})

	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(launch.Argv, " "); got != "codex resume c-1 -c hooks.Stop=[]" {
		t.Errorf("argv = %q, want codex resume c-1 with its hook flags", got)
	}
	fresh, _ := l.Launch(session.Session{ID: "pane-2", Dir: "/w", Agent: "codex"})
	if got := strings.Join(fresh.Argv, " "); got != "codex -c hooks.Stop=[]" {
		t.Errorf("fresh argv = %q, want codex with its hook flags", got)
	}
}

// The -c flags rendered at start reach the running launcher, so a codex
// session omatty starts reports through its hooks (#152).
func TestRuntimeFor_PassesCodexItsHookArgs_issue152(t *testing.T) {
	env := tuiEnv{Home: "/h", Agents: mustAgents(t), HooksFiles: map[string]string{"claude": "/h/hooks.json"},
		HookArgs: map[string][]string{"codex": {"-c", "hooks.Stop=[]"}}, Holder: &detach.Plain{}, Width: 80, Height: 24}
	env.Cfg = config.Defaults("/h")
	t.Setenv("CODEX_HOME", "")

	launch, err := runtimeFor(env).Launch.Launch(session.Session{ID: "pane-1", Dir: "/w", Agent: "codex"})

	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(launch.Argv, " "); got != "codex -c hooks.Stop=[]" {
		t.Errorf("argv = %q, want codex with the rendered hook flags", got)
	}
}
