package agent

import (
	"path/filepath"

	"github.com/WilsonSousajr/omatty/internal/infra/hooks"
	"github.com/WilsonSousajr/omatty/internal/infra/paths"
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
		TranscriptPath: claudeTranscript,
		HookEvents:     watcher.HookEventNames,
		RenderSettings: hooks.Render,
		Status:         watcher.ClaudeAdapter(),
	}
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
