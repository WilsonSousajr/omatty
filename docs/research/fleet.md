# `brizzai/fleet` — deep dive

> Captured **2026-09-18** against `brizzai/fleet` at `master`, Apache-2.0, Go,
> 53 stars, v2.43.0 released 2026-09-17, 13 open issues. **Read at code
> level.** Analysis only.

The smallest project in the table and the most useful one to read. fleet is
Go, Bubble Tea, terminal-first, hook-driven and repo-grouped — the same five
decisions omatty made — which makes every place it diverges a genuine
second opinion rather than a difference of genre. It also contains the single
finding in this pass that unblocks an omatty issue.

## 1. Purpose and scope

"A terminal cockpit for orchestrating Claude Code, Codex & OpenCode sessions
in parallel. See which agents need you. Jump in, direct, jump out." Sessions
are **grouped by repo**; status is real-time and hook-driven; PR state is
shown on the row.

The interaction loop is nearly word for word omatty's: `Space` jumps to the
next session that needs attention, `Enter` attaches, `Ctrl+Q` detaches. It
requires tmux (≥ 3.3 recommended on Linux), which omatty does not.

## 2. The finding: Codex has a hook mechanism, and #152 says it does not

omatty's #152 half-spike, recorded 2026-09-18, concluded that Codex's status
could only ever come from its transcript, and therefore that "`DeriveKind` can
never report `PermissionRequested` from a transcript, and Codex has no hook
mechanism, so a Codex session can never show 'waiting for you'."

`internal/hooks/codex_hooks.go` contradicts that, in code that ships:

```go
// codexHookEvents lists the Codex hook events fleet subscribes to. Field names
// in Codex's payload match Claude's (hook_event_name, session_id, prompt), so
// `fleet hook-handler` and the status pipeline are reused unchanged. Codex has
// no SessionEnd/Notification events — `dead` comes from tmux pane-death.
var codexHookEvents = []string{
	"SessionStart",
	"UserPromptSubmit",
	"PermissionRequest",
	"Stop",
}
```

`InjectCodexHooks` merges these into `$CODEX_HOME/hooks.json` (default
`~/.codex/hooks.json`), whose shape is documented in the same file as
`{"hooks": {"<Event>": [ {"hooks": [ {"type","command"} ]} ]}}` — "the same
event-map structure as Claude's `settings.json["hooks"]`", with the Claude-only
`async` field omitted.

So: **Codex has hooks, at a known path, in a shape omatty's `internal/hooks`
already understands, including a `PermissionRequest` event** — the exact
signal #152 recorded as unobtainable. Two of #152's three open questions are
answered by reading this file; what remains is whether `codex` accepts an
externally assigned session id, which still needs the real binary.

This is the market-research method paying for itself. The finding cost one
`grep` and was sitting in a fifty-three-star repository; #152's spike spent a
session on 55 real rollouts and reached the opposite conclusion. **#299 must
carry this to #152 as a correction**, and the issue's recorded finding should
be amended rather than left to mislead the next person.

One caution before anyone acts on it: fleet's comment is evidence that this
worked against some version of Codex on some date, not a specification.
Verify against the installed binary before building on it — which is exactly
what #152's remaining half requires anyway.

## 3. Also worth borrowing

- **`.fleet.json` for custom workspace commands** and an `exclude` mechanism
  in `internal/git/exclude.go` — fleet's answer to ccmanager's
  `.worktreeinclude`. Two independent projects solved "a new worktree is
  missing the files git ignores"; omatty has not.
- **PR state on the session row** (`internal/github/pr.go`). omatty's card
  shows the gate; it does not show whether the branch has a PR or what CI said
  about it. Given M9's thesis, that is a closer neighbour to omatty's own bet
  than anything else found in this pass.
- **A terminal drawer** (`` ` ``) for real shells scoped to the repo or
  worktree — dev servers, log tails, `lazygit` — "rendered through a true
  terminal emulator so streaming output, colors, and full-screen tools just
  work". omatty has one pane per session and no scratch shell.
- **Session forking** (`f`): branch the agent's conversation at a point and
  run both. omatty has no counterpart and this would need a hard look at
  invariant 9 before it could have one.

## 4. Where fleet goes somewhere omatty refuses

`fleet skill install` writes an Agent Skill "that teaches Claude Code, Codex,
Cursor, and OpenCode how to use fleet's CLI — so an agent can hand independent
work to a fresh worktree session (`fleet wt fix-242 -p "…"`), message one
that's already running (`fleet send`), and see what's in flight."

That is agent-to-agent messaging and agents spawning agents, both refused by
name in "Not on the roadmap", implemented as an opt-in skill. It is a well-made
version of the thing omatty says it will not build, and its existence is worth
a line in the refusal's row: the option was available, someone took it, and
the reason for declining is unchanged.

`Y` — "already trust the prompt? approves without attaching" — is the same
territory as claude-squad's `--autoyes`, with the blast radius of one prompt
rather than all of them.

## 5. Weaknesses to avoid

- **tmux as a hard dependency**, with a documented version floor.
- **No diff review.** `internal/ui/preview.go` is a preview, and there is no
  comment, anchor or send-back. Same silence as the rest of the camp on
  whether the work is good.
- No gate.

## 6. Bottom line

The nearest architectural sibling omatty has, arrived at independently, and
the proof that the shape — Go TUI, hooks not screen-scraping, sessions grouped
by repo — is not idiosyncratic. It is ahead of omatty on worktree file
carrying, PR state and a scratch shell; it is behind on review, which it does
not attempt. Read its tracker in #297 with more care than its star count
suggests.

## Sources

- `github.com/brizzai/fleet`, `master`, read 2026-09-18: `README.md`,
  `internal/hooks/codex_hooks.go`, `internal/hooks/claude_hooks.go`,
  `internal/git/exclude.go`, `internal/github/pr.go`, `internal/ui/preview.go`,
  package layout.
- GitHub API for stars, release date and issue counts.
- omatty issue #152 for the finding this corrects.
