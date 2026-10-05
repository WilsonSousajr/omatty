// The codex profile (#152), composed here for the reason claude's is: it
// names a payload parser and an argument renderer (internal/infra/hooks), a
// rollout locator (internal/infra/fsread) and a status adapter
// (internal/service/status), and internal/domain/agent may import none of
// them. The research behind every field is docs/research/agents/codex.md
// (#528).

package main

import (
	"os"
	"path/filepath"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/infra/fsread"
	"github.com/WilsonSousajr/omatty/internal/infra/hooks"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// codexProfile is OpenAI's codex at the Full tier: it reports the id it
// chooses on SessionStart (Reported), takes its hooks and their trust as
// -c flags so nothing is written to ~/.codex (invariant 3), reports waiting
// through PermissionRequest, and resumes by id.
//
//	agents, err := agent.NewCatalog(claudeProfile(), codexProfile())
func codexProfile() agent.Profile {
	return agent.Profile{
		Name:       "codex",
		DefaultBin: "codex",
		Caps: agent.Caps{Identity: agent.Reported, Status: agent.StatusHooks,
			Waiting: true, Resume: true, TurnBoundary: true},
		Command:        agent.CodexCommand,
		TranscriptPath: codexTranscript,
		HookEvents:     status.CodexHookEventNames,
		RenderArgs:     hooks.RenderCodexArgs,
		ParseHook:      hooks.ParseCodexPayload,
		Status:         status.CodexAdapter(),
	}
}

// codexTranscript is the conversation's rollout, found by id under codex's
// store. One codex has not written - a pane nobody has typed into, or the
// pane's own id before codex reported one - names a file that will never
// exist, so the launcher starts codex fresh and the tailer reads nothing.
//
// The tailer resolves this once, when the SessionStart re-bind re-adds the
// row, and the rollout is there by then: codex materialises it before it
// runs SessionStart, to put its path in the payload (core/src/session,
// hook_transcript_path -> ensure_rollout_materialized). #152's smoke test
// showed the meter filling on a fresh pane.
func codexTranscript(home, _, conversation string) string {
	store := codexStore(home)
	if path, ok := fsread.CodexRollout(store, conversation); ok {
		return path
	}
	return filepath.Join(store, "sessions", "unwritten-"+conversation+".jsonl")
}

// codexStore is where codex keeps its sessions: $CODEX_HOME as codex itself
// reads it, else ~/.codex.
func codexStore(home string) string {
	if dir := os.Getenv("CODEX_HOME"); dir != "" {
		return dir
	}
	return filepath.Join(home, ".codex")
}
