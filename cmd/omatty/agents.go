// The agent catalog: every profile omatty can run, each composed from the
// implementations it carries. It lives in cmd, the composition root, because
// a profile names a transcript parser (internal/service/status), a settings
// renderer (internal/infra/hooks) and a path function (internal/infra/paths),
// and internal/domain/agent may import none of them (ADR 0001, migration step
// 5.2b, #653), and joined into the agent.Catalog the launcher and the
// watcher resolve each session through (#521).

package main

import (
	dstatus "github.com/WilsonSousajr/omatty/internal/domain/status"
	"io"
	"path/filepath"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/infra/hooks"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// claudeProfile is the profile for Anthropic's claude binary, the only agent
// omatty runs today (#46).
//
//	agents, err := agent.NewCatalog(claudeProfile())
func claudeProfile() agent.Profile {
	return agent.Profile{
		Name:       "claude",
		DefaultBin: "claude",
		Caps: agent.Caps{Identity: agent.Assigned, Status: agent.StatusHooks,
			Waiting: true, Resume: true, TurnBoundary: true},
		Command:        agent.ClaudeCommand,
		TranscriptPath: claudeTranscript,
		HookEvents:     status.HookEventNames,
		RenderSettings: hooks.Render,
		ParseHook:      hooks.ParsePayload,
		Status:         status.ClaudeAdapter(),
	}
}

// agentCatalog is every agent this omatty can run, claude first: an empty
// agent name in state.json is every row written before #46, and the catalog
// reads it as its first profile (invariant 9). Built once here and injected,
// so no package below cmd keeps a registry (#521).
//
//	agents, err := agentCatalog()
func agentCatalog() (agent.Catalog, error) {
	return agent.NewCatalog(claudeProfile())
}

// claudeTranscript is where claude writes the session's JSONL. claude names
// the directory after its working directory as the kernel reports it, every
// symlink resolved: a project registered as /tmp/x writes under
// -private-tmp-x on macOS. Slugging dir as registered missed the file for any
// project behind a link, so status fell back to hooks alone and a crash
// restart used --session-id where it had to resume (#564). A dir that does
// not exist yet has written nothing, and is used as given.
//
//	claudeTranscript("/h", "/tmp/x", "id") // "/h/.claude/projects/-private-tmp-x/id.jsonl" on macOS
func claudeTranscript(home, dir, sessionID string) string {
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return paths.Transcript(home, dir, sessionID)
}

// hookParser resolves `omatty hook`'s arguments to the payload parser of the
// agent they name. No arguments is claude, which is every hooks.json written
// before M17; `--agent <name>` is that agent's shape (#522). Anything else -
// an unknown agent, one without hooks, a malformed flag - is no parser, and
// the hook sends nothing.
//
// The lookup is the in-memory catalog, which reads no config and no file:
// the agents that take hooks are the built-in profiles, never a generic one
// from config.toml (#525), so invariant 11's "no config read on this path"
// still holds.
//
//	parse, ok := hookParser([]string{"--agent", "codex"})
func hookParser(args []string) (func(io.Reader) (dstatus.HookPayload, bool), bool) {
	name := ""
	if len(args) > 0 {
		if len(args) != 2 || args[0] != "--agent" || args[1] == "" {
			return nil, false
		}
		name = args[1]
	}
	agents, err := agentCatalog()
	if err != nil {
		return nil, false
	}
	profile, err := agents.Lookup(name)
	if err != nil || profile.ParseHook == nil {
		return nil, false
	}
	return profile.ParseHook, true
}
