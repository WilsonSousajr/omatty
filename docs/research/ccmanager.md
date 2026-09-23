# `kbwo/ccmanager` — deep dive

> Captured **2026-09-18** against `kbwo/ccmanager` at `main`, MIT, TypeScript,
> 1,246 stars, v4.4.3 released 2026-09-13, 4 open issues. **Read at code
> level.** Analysis only.

The closest comparison omatty has: a terminal UI, no tmux, several agent CLIs,
multiple repositories in one interface, and per-session status in the list.
Four of omatty's eleven milestones have a counterpart here. It is also the
project that breaks `README.md`'s claim about the field.

## 1. Purpose and scope

"A CLI application for managing multiple AI coding assistant sessions (Claude
Code, Gemini CLI, Codex CLI, Cursor Agent, Copilot CLI, Cline CLI, OpenCode,
Kimi CLI) across Git worktrees and projects." Eight agent CLIs against
omatty's one-and-a-half (#152 is half-spiked).

Its README argues its own position against claude-squad directly and fairly —
"If you love tmux-based workflows, stick with Claude Squad!" — and claims two
differentiators: no tmux dependency, and real-time session state in the menu.
Both are true of omatty as well, which is the point of this document.

## 2. Session and process model

`src/services/sessionManager.ts` spawns each agent itself through node-pty;
there is no tmux and no multiplexer. Sessions are recorded durably in
`~/.config/ccmanager/sessions.json` (`%APPDATA%\ccmanager\sessions.json` on
Windows), and `src/services/sessionRestorer.ts` reopens them: "reopen the
sessions that were running when CCManager last exited or crashed", skipping
records whose worktree has gone, and re-recording each restored session under
its new id.

That is M6's promise reached by a different route. omatty keeps the *process*
alive across a quit via dtach and reattaches to it; ccmanager lets the process
die and starts a new one against the same worktree, restoring the list rather
than the session. omatty's is the stronger guarantee, and dtach is the price.

## 3. Status — where the two designs actually part

ccmanager's status indicators are produced by pattern-matching the agent's
terminal output. `src/utils/promptDetector.ts`, in full for the first
function:

```ts
export function includesPromptBoxLine(output: string): boolean {
  // Check if the output includes a prompt box line pattern (│ > [spaces])
  return output.split('\n').some(line => /│\s*>\s*/.test(line));
}
```

with siblings matching the box's top and bottom borders — `╭───╮`, `╰───╯`,
`──╮`, `──╯` — and rejecting lines where a `│` follows the corner. The README
calls this "configurable state detection strategies for different CLI tools";
the strategy is a regular expression over Claude Code's drawn frame.

omatty refuses this by invariant: status comes from Claude Code hooks and the
session's transcript, "never scraped from the screen". The two approaches fail
differently, and the failure is the argument:

- ccmanager's detector breaks when Claude Code changes how it draws its prompt
  box — a cosmetic release on someone else's schedule — and cannot distinguish
  a prompt box that is *displayed* from one that is *waiting*.
- omatty's breaks when the hook contract or the transcript shape changes,
  which is a documented interface, and it degrades to transcript-only rather
  than to nothing.

This is the single most useful finding in the deep-dive pass. Invariant 2's
"never scraped from the screen" was argued from first principles in M2; there
is now a shipped, popular, well-built counter-example to point at, and it
spells out exactly what the rule buys.

## 4. Multi-repository — and the README claim it breaks

A documented **Multi-Project Mode**: "CCManager can manage multiple git
repositories from a single interface", entered with
`CCMANAGER_MULTI_PROJECT_ROOT` and `--multi-project`, offering "automatic
project discovery: recursively finds all git repositories", recent projects
first, vi-like `/` search over projects and worktrees, and sessions that stay
alive while you switch projects.

That is M1's multi-project sidebar plus M4's `discover`, in a terminal,
shipped. `README.md`'s "Every other tool in this space is either a desktop app
or scoped to a single repository" is therefore **false as written**, and #300
owes it a correction rather than a defence.

## 5. Features omatty has no answer to

- **`.worktreeinclude`** — carries gitignored files (`.env`, local certs) into
  a newly created worktree, with the files copied *before* the post-creation
  hook runs so hooks can rely on them. omatty creates a worktree and leaves the
  operator to notice that `.env` is missing. This is the most directly useful
  thing found in the whole pass.
- **Command presets with automatic fallback** when the primary agent command
  fails (`sessionManager.ts` spawns a fallback).
- **Devcontainer integration**, and an experimental auto-approval that uses a
  second model to judge whether a prompt is safe. The latter is refused
  territory for omatty — it is "sending something without being asked" in
  everything but name — and belongs in "Ideas Not To Copy" with the refusal it
  breaks named: "sending anything without being asked".

## 6. Weaknesses to avoid

- Screen-scraped status, above.
- No review surface: there is no diff pane, no comments, and nothing to send
  back to the agent. The session list is the product. This is the shape of the
  camp M9 described, and ccmanager is a clean example of it: excellent at
  *managing* sessions, silent on whether the work is good.
- No gate. Same as everyone else (see `2026-landscape.md` §3.2).

## 7. Bottom line

The strongest terminal competitor, better than omatty at breadth (eight
agents, devcontainers, `.worktreeinclude`, Windows) and structurally weaker at
the two things omatty was built for: it cannot tell you whether the work is
good, and its idea of what a session is doing is a regex over a box drawing.
For #300: this is the tool a migration argument has to be made against, and
"we are multi-repository and terminal-native" is not that argument.

## Sources

- `github.com/kbwo/ccmanager`, `main`, read 2026-09-18: `README.md`,
  `src/utils/promptDetector.ts`, `src/services/sessionManager.ts`,
  `src/services/sessionRestorer.ts`.
- GitHub API for stars, release and issue counts.
