// The agent catalog: every profile omatty can run, each composed from the
// implementations it carries. It lives in cmd, the composition root, because
// a profile names a transcript parser (internal/service/status), a settings
// renderer (internal/infra/hooks) and a path function (internal/infra/paths),
// and internal/domain/agent may import none of them (ADR 0001, migration step
// 5.2b, #653). M17's Catalog (#521) lands here.

package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/domain/agent"
	"github.com/WilsonSousajr/omatty/internal/infra/hooks"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
	"github.com/WilsonSousajr/omatty/internal/service/status"
)

// claudeProfile is the profile for Anthropic's claude binary, the only agent
// omatty runs today (#46).
//
//	l := sessions.NewLauncher(claudeProfile(), cfg.ClaudeBin, hooksFile, home, holder)
func claudeProfile() agent.Profile {
	return agent.Profile{
		Name:           "claude",
		DefaultBin:     "claude",
		Command:        agent.ClaudeCommand,
		TranscriptPath: claudeTranscript,
		HookEvents:     status.HookEventNames,
		RenderSettings: hooks.Render,
		Status:         status.ClaudeAdapter(),
	}
}

// lookupAgent returns the profile a Session names. An empty name is claude:
// every session written before #46 has one, and an empty value that is
// derivable is what lets state.json stay at Version 1 (invariant 9).
//
//	p, err := lookupAgent(sess.Agent)
func lookupAgent(name string) (agent.Profile, error) {
	if name == "" || name == claudeProfile().Name {
		return claudeProfile(), nil
	}
	return agent.Profile{}, fmt.Errorf("agent %q is not one omatty knows, want one of %s", name, strings.Join(agentNames(), ", "))
}

// agentNames is every profile omatty knows, for lookupAgent's error and for
// the config file's documentation.
func agentNames() []string { return []string{claudeProfile().Name} }

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
