// Package paths resolves every filesystem location omatty reads or writes.
//
// Every function takes the home directory explicitly rather than calling
// os.UserHomeDir, so tests never touch the real ~/.omatty or ~/.claude.
package paths

import (
	"path/filepath"
	"regexp"
)

// Root returns omatty's private directory, e.g. paths.Root("/home/u") is
// "/home/u/.omatty".
func Root(home string) string { return filepath.Join(home, ".omatty") }

// StateFile returns the session registry file.
func StateFile(home string) string { return filepath.Join(Root(home), "state.json") }

// HooksFile returns the settings file omatty passes to `claude --settings`.
// Invariant 3: omatty never writes the user's own settings.
func HooksFile(home string) string { return filepath.Join(Root(home), "hooks.json") }

// HookSocket returns the unix socket the injected hooks report to.
func HookSocket(home string) string { return filepath.Join(Root(home), "sock") }

// LogDir returns where the slog file handler writes (invariant 5).
func LogDir(home string) string { return filepath.Join(Root(home), "logs") }

// WorktreeDir returns where omatty places a worktree for a project branch.
func WorktreeDir(home, project, branch string) string {
	return filepath.Join(Root(home), "wt", project, branch)
}

// slugPattern matches every character Claude Code replaces when it names a
// transcript directory: anything outside [a-zA-Z0-9] becomes '-'. Taken from
// the claude binary's own transform. Issue #60 was a path with a space that
// the old '/'-and-'.'-only mapping missed, leaving the tailer blind and every
// restart using --session-id (the #36 failure). Invariant 2 depends on this
// being exact.
var slugPattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

// TranscriptSlug converts an absolute working directory into the directory
// name Claude Code uses under ~/.claude/projects.
//
//	paths.TranscriptSlug("/Users/w/LAB SD") // "-Users-w-LAB-SD"
func TranscriptSlug(dir string) string { return slugPattern.ReplaceAllString(dir, "-") }

// Transcript returns the JSONL file Claude Code writes for sessionID running
// in dir, e.g. "/home/u/.claude/projects/-home-u-p/abc-123.jsonl".
func Transcript(home, dir, sessionID string) string {
	return filepath.Join(home, ".claude", "projects", TranscriptSlug(dir), sessionID+".jsonl")
}
