# `smtg-ai/claude-squad` — deep dive

> Captured **2026-09-18** against `smtg-ai/claude-squad` at `main`, AGPL-3.0,
> Go, 8,495 stars, v1.0.20 released 2026-08-20, 19 open issues. **Read at code
> level.** Analysis only.

The tool that defined omatty's camp, and the one every other project in it
positions against. Also the only one in the table whose last commit and last
release are the same day, four weeks before capture.

## 1. Purpose and scope

"A terminal app that manages multiple Claude Code, Codex, Gemini (and other
local agents including Aider) in separate workspaces." Installs as `cs`.
Requires **tmux** and **`gh`**. Licensed AGPL-3.0 — the only copyleft licence
in the field, and a real consideration for anyone thinking about borrowing
code rather than ideas.

## 2. Session and process model

tmux does the work: one tmux session per instance, one git worktree per task,
"so no conflicts". `session/tmux/tmux.go` drives it, `session/git/` owns the
worktrees, `session/storage.go` persists instances. There is a `daemon/` and a
`web/` directory, so the project has grown beyond the single TUI its README
describes.

Delegating to tmux is the decision omatty made the other way (`internal/termwrap`
owns the PTY, `internal/detach` uses dtach only for persistence), and the
trade is legible: claude-squad gets battle-tested multiplexing and a hard
dependency, omatty gets control of the emulator and had to fix #192's C1-byte
bug itself.

## 3. Status — literal English UI strings

`session/tmux/tmux.go`, in `HasUpdated()`:

```go
// Only set hasPrompt for claude and aider. Use these strings to check for a prompt.
hasPrompt = strings.Contains(content, "No, and tell Claude what to do differently")
hasPrompt = strings.Contains(content, "(Y)es/(N)o/(D)on't ask again")
hasPrompt = strings.Contains(content, "Yes, allow once")
```

`content` is the captured tmux pane. The detector matches **the literal English
text of Claude Code's permission prompt**. There is a matching
`CheckAndHandleTrustPrompt()` that scans the pane for the trust prompt and
dismisses it.

This is the same class of dependency as ccmanager's box-drawing regex and a
step more fragile: it breaks on a copy edit, and it breaks for any user whose
agent is not running in English. Together, the two projects make invariant 2's
case better than the invariant's own prose does — the field's two most popular
terminal managers both read the screen, and both are one upstream string
change from being wrong.

## 4. Review surface

Real, and display-only. `session/git/diff.go` shells out to `git diff` against
the base branch and parses `--numstat` for the added/removed counts; the UI
offers "a diff tab beside a preview tab" and the README's "review changes
before applying them, checkout changes before pushing them".

So: a diff you read, with no comments, no anchors and nothing sent back. M3's
loop — comment on lines, compose one message, paste it into the session — has
no counterpart here.

## 5. Gate and verification

None.

## 6. `--autoyes`, and the line omatty drew

claude-squad ships `-y, --autoyes`: "[experimental] If enabled, all instances
will automatically accept prompts for claude code & aider", plus the trust
prompt auto-dismissal above. ccmanager's README argues against it by name:
"this bypasses Claude Code's built-in security confirmations - not recommended
for safe operation."

omatty refuses this territory in "Not on the roadmap" under "sending anything
without being asked", and the refusal is currently argued from principle. It
now has a shipped example and a competitor's public objection to cite. Worth
adding to the refusal's row in #301.

## 7. Strengths worth borrowing

- **Agent-agnostic from the start** via a plain `--program` flag, where
  omatty's `internal/agent` seam needs a profile per agent. Lower ceiling,
  much lower floor.
- **Homebrew and a one-line installer**, and a binary named `cs`. omatty has
  neither; #301 should note the distribution gap even though it is not
  research.

## 8. Weaknesses to avoid

- Screen-scraped status against English UI copy.
- A hard dependency on both tmux and `gh`.
- AGPL-3.0, which makes reading this code for ideas fine and copying it a
  licensing decision. **Nothing in omatty may be derived from this source.**

## 9. Bottom line

The camp's reference implementation, plateauing (four weeks quiet, 19 open
issues), strong on isolation and distribution, and stopping exactly where
omatty starts: it shows you the diff and has no opinion about whether it is
any good. #297 reads the tracker to find out whether its users are asking for
one.

## Sources

- `github.com/smtg-ai/claude-squad`, `main`, read 2026-09-18: `README.md`,
  `session/tmux/tmux.go`, `session/git/diff.go`, `session/storage.go`,
  repository layout.
- GitHub API for stars, release date and issue counts.
- `kbwo/ccmanager`'s `README.md` for the quoted objection to `--autoyes`,
  cited as a competitor's stated position rather than as an independent source.
