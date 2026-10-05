package hooks_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/infra/hooks"
)

// The hash codex 0.160.0 itself stored when its "Trust all" was chosen for
// this hook, captured in the #528 spike: SessionStart, the timeout left to
// its default, the command a logging script.
func TestCodexTrustHash_MatchesWhatCodexStored_issue152(t *testing.T) {
	cmd := "/private/tmp/claude-501/-Users-will--omatty-wt-omatty-chore-architectural-refact/" +
		"02f627e5-8cf3-4612-a4fa-9bf5cc52a5d4/scratchpad/spike/hooklog.sh SessionStart"
	got := hooks.CodexTrustHash("SessionStart", cmd, 600)
	want := "sha256:5b89282e5f314fa29f1d9dcf3ce55102b5e43542384a054934006924eedf509f"
	if got != want {
		t.Errorf("CodexTrustHash = %s, want %s", got, want)
	}
}

// omatty's real hook line has `2>/dev/null`. encoding/json escapes '>' as
// > by default and codex's serde does not, so a hash over Go's default
// encoding names a different hook and lands it back on codex's review
// screen. The vectors are the ones codex accepted in the #528 spike.
func TestCodexTrustHash_DoesNotHTMLEscapeTheCommand_issue152(t *testing.T) {
	line := hooks.HookCommand("/b/omatty", "codex")
	if got, want := hooks.CodexTrustHash("Stop", line, 5),
		"sha256:e9ce0303b8806f1a906ddfc1714d41077051cea06c30f49117253deba07c9e57"; got != want {
		t.Errorf("Stop hash = %s, want %s", got, want)
	}
	if got, want := hooks.CodexTrustHash("Interrupt", line, 3),
		"sha256:fe4cfd25f1cce38ccfc10942bae11c8e174f35ccb7d1d052fb55a6614f37262c"; got != want {
		t.Errorf("Interrupt hash = %s, want %s", got, want)
	}
}

// Every event gets one `-c hooks.<Event>=` declaration and one trust entry,
// all under the session-flags layer, so nothing is written to $CODEX_HOME
// (invariant 3).
func TestRenderCodexArgs_DeclaresEachHookAndItsTrust_issue152(t *testing.T) {
	args, err := hooks.RenderCodexArgs("/b/omatty", []string{"Stop", "Interrupt"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"-c", `hooks.Stop=[{hooks=[{type="command",command="'/b/omatty' hook --agent 'codex' 2>/dev/null || true",timeout=5}]}]`,
		"-c", `hooks.Interrupt=[{hooks=[{type="command",command="'/b/omatty' hook --agent 'codex' 2>/dev/null || true",timeout=3}]}]`,
		"-c", `hooks.state={"/<session-flags>/config.toml:stop:0:0"={trusted_hash="sha256:e9ce0303b8806f1a906ddfc1714d41077051cea06c30f49117253deba07c9e57"},` +
			`"/<session-flags>/config.toml:interrupt:0:0"={trusted_hash="sha256:fe4cfd25f1cce38ccfc10942bae11c8e174f35ccb7d1d052fb55a6614f37262c"}}`,
	}
	if !slices.Equal(args, want) {
		t.Errorf("args =\n%s\nwant\n%s", strings.Join(args, "\n"), strings.Join(want, "\n"))
	}
}

// Codex clamps Interrupt and SessionEnd to [1, 3] s and hashes the clamped
// value, so a 5 s declaration would be "Modified since last trusted" on
// every start. omatty declares what codex will normalise to.
func TestRenderCodexArgs_ClampsTheShortLivedEvents_issue152(t *testing.T) {
	args, err := hooks.RenderCodexArgs("/b/omatty", []string{"SessionEnd", "UserPromptSubmit"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(args[1], "timeout=3") || !strings.Contains(args[3], "timeout=5") {
		t.Errorf("args = %q, want SessionEnd at 3 s and UserPromptSubmit at 5 s", args)
	}
}

// A binary path with a quote or backslash must survive both the shell (the
// hook line quotes it, #56) and TOML (the -c value escapes it).
func TestRenderCodexArgs_EscapesTheCommandForTOML_issue152(t *testing.T) {
	args, err := hooks.RenderCodexArgs(`/b/it's "here"\omatty`, []string{"Stop"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(args[1], `command="'/b/it'\\''s \"here\"\\omatty' hook`) {
		t.Errorf("declaration = %s, want the shell-quoted line TOML-escaped", args[1])
	}
}

// codex's background memories thread fires the pane's own hooks with a null
// transcript_path; the real session always names its rollout. Dropping the
// payload in the hook keeps that thread from re-binding the row (#152).
func TestParseCodexPayload_DropsAThreadWithNoTranscript_issue152(t *testing.T) {
	own := `{"session_id":"s1","transcript_path":"/h/.codex/sessions/r.jsonl","cwd":"/w","hook_event_name":"SessionStart","source":"startup"}`
	memories := `{"session_id":"s2","transcript_path":null,"cwd":"/h/.codex/memories","hook_event_name":"SessionStart","source":"startup"}`
	missing := `{"session_id":"s3","hook_event_name":"Stop"}`
	p, ok := hooks.ParseCodexPayload(strings.NewReader(own))
	if !ok || p.SessionID != "s1" || p.HookEventName != "SessionStart" || p.Source != "startup" {
		t.Errorf("real session = %+v, %v; want it parsed", p, ok)
	}
	if _, ok := hooks.ParseCodexPayload(strings.NewReader(memories)); ok {
		t.Error("the memories thread's payload was accepted")
	}
	if _, ok := hooks.ParseCodexPayload(strings.NewReader(missing)); ok {
		t.Error("a payload with no transcript_path was accepted")
	}
}
