package agent

import (
	"github.com/WilsonSousajr/omatty/internal/hooks"
	"github.com/WilsonSousajr/omatty/internal/paths"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// Claude is the profile for Anthropic's claude binary, the only agent omatty
// runs today (#46).
//
//	l := supervisor.NewLauncher(agent.Claude(), cfg.ClaudeBin, hooksFile, home, holder)
func Claude() Profile {
	return Profile{
		Name:           "claude",
		DefaultBin:     "claude",
		Command:        claudeCommand,
		TranscriptPath: paths.Transcript,
		HookEvents:     watcher.HookEventNames,
		RenderSettings: hooks.Render,
		Status:         watcher.ClaudeAdapter(),
	}
}

// claudeCommand is claude's argument list. A session that has never spoken
// starts with --session-id, which lets omatty choose the uuid and so know
// the transcript path (invariant 2). Once a transcript exists claude refuses
// that flag - "Session ID <uuid> is already in use" - because the transcript
// itself is the claim, so it is resumed instead (#36). Either way --settings
// names omatty's own file, never the user's (invariant 3).
func claudeCommand(bin, sessionID, _ string, resume bool, settingsFile string) []string {
	flag := "--session-id"
	if resume {
		flag = "--resume"
	}
	return []string{bin, flag, sessionID, "--settings", settingsFile}
}
