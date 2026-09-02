# omatty roadmap

Last revised 2026-09-01, after M1 was built and used for the first time.

omatty is a terminal ADE: several projects and several parallel Claude Code
sessions in one window, each session the real `claude` binary in an embedded
pane. It is built for one person first and opened up later, so every
milestone is ranked by how much daily friction it removes, and nothing on
this list exists only for strangers until M7.

Milestones are vertical slices. Each one produces software that is usable on
its own, and each ends with a smoke test of the real binary in a real PTY -
not only the coverage gate. See "Rules" at the end for why.

## Where things stand

| | Milestone | Status |
|---|---|---|
| M1 | Skeleton | Built. **Not done** until #36 and #35 land. PR #14 open. |
| M2 | Status | Next. Issues #17-#20, #37-#39 in Backlog. |
| M3 | Review | Planned. Issues #21-#23 in Backlog. |
| M4 | Lifecycle | Planned. Issues #15, #40-#42 in Backlog. |
| M5 | File tree | Planned. Issue #24 in Backlog. |
| M6 | Persistence | Planned. Issue #43 in Backlog. |
| M7 | Reach | Planned. Issues #44-#46 in Backlog. |

The board at github.com/users/WilsonSousajr/projects/13 is the live view;
this document is the reasoning behind its order.

---

## M1 - Skeleton *(close-out)*

**Delivers:** projects and sessions registered, worktrees created on demand,
the real `claude` running inside an embedded pane, modal key routing so every
keystroke reaches Claude except the `ctrl+o` leader.

**Built:** #1-#13, then seven bugs found by actually running it (#28-#34),
each fixed with a failing test first.

**Still open, and blocking the merge:**

- **#36 - restarting omatty kills every session that has ever been used.**
  `StartTerminals` launches `claude --session-id <uuid>`; once a session has a
  transcript, claude refuses with "Session ID is already in use". There is no
  lock file - the transcript is the claim. The launcher must use
  `--resume <uuid>` when `paths.Transcript` exists and `--session-id` only
  when it does not. Invariant 9 promised exactly this; the code never did it.
- **#35 - panes are stacked; the approved design is side by side.** The
  sidebar renders above the terminal, so a growing session list pushes the
  thing you are reading down the screen. Fixing this rewrites `View`, so real
  styling (lipgloss borders, focused-pane highlight, status colours) lands in
  the same change rather than touching `View` twice.

**Done when:** a used session survives quit-and-relaunch, and the sidebar
sits beside the terminal.

## M2 - Status

**Delivers:** the sidebar tells you what every session is doing, across all
projects, without switching into any of them.

**Why next:** it is the single thing that stops omatty being the daily driver
over terminal tabs. Six sessions all showing `-` is six sessions you have to
click into.

**Contents:**

- #17 write real hooks into `~/.omatty/hooks.json` (`Notification`, `Stop`,
  `PreToolUse`). #31 stubbed this file as `{"hooks":{}}` so sessions could
  start; M2 fills it in. Invariant 3: omatty's own file, never the user's.
- #18 a unix socket the hooks report to - low latency.
- #19 tail `~/.claude/projects/<slug>/<uuid>.jsonl` - the truth. This is why
  omatty assigns the session UUID itself (invariant 2). Status is derived from
  structured data, never scraped from the rendered pane.
- #20 glyphs in the sidebar: `● thinking` `⚙ tool` `⏸ waiting for you`
  `✓ done` `✗ error`.
- **Last-activity age** - `parser-fix ⏸ 4m`. The thing that stops a waiting
  session being forgotten. One more column from the same tailer.
- **Desktop notification** when a session needs you while omatty is in the
  background. Fires from the hook event; deferred from v1 and now cheap.
- **Token / cost per session**, read from the JSONL, so you can see which
  session is spending your quota.

The last three ride on the tailer and are each about a day. They are in M2
rather than a follow-on because they are the same data shown three ways, and
status without "for how long" is half a feature.

**Done when:** you can glance at the sidebar and know which session is
waiting on a permission prompt, and how long it has been waiting.

## M3 - Review

**Delivers:** the reason the tool exists. Review a session's diff without
leaving omatty, comment on lines, send the whole batch back to Claude as one
message.

**Contents, exactly as approved and nothing more:**

- #21 diff pane: merge-base against the session's base branch, unioned with
  uncommitted changes - everything this session changed, committed or not.
  Parsed from git's unified output with `bluekeyes/go-gitdiff`, through
  `internal/vcs` (invariant 4).
- #22 comments anchor on `(file, hunk header, line hash)`, never a line
  number. Claude edits files while you read them; a line-number anchor
  silently attaches feedback to the wrong code (invariant 7). Orphans float to
  the top of the file marked moved.
- #23 `[S]` composes one message - `file:line`, the quoted line, your note,
  per comment - and writes it to the PTY as a bracketed paste followed by one
  `\r` (invariant 8). This is the `SendInput` path; keystrokes use `Update`.

**Deliberately out:** asking Claude to self-review, commit/push/PR from
omatty, running N sessions on one task and comparing, broadcasting a prompt.
All considered; all cut. Review stays a person reading a diff and commenting.
Shipping stays in git.

**Done when:** you review a two-file change, leave three comments, press
`[S]`, and Claude receives them as one message and acts on all three.

## M4 - Lifecycle

**Delivers:** managing sessions once you have many of them.

**Why here:** by the end of M3 you will have ten or more sessions and no way
to get rid of one except editing `state.json`. None of this depends on M2 or
M3, so it could go anywhere; after the two big milestones is where the pain
peaks.

**Contents:**

- **Kill / archive** (`ctrl+o x`): stop the process, optionally
  `git worktree remove`, drop it from the sidebar. A confirmation, because
  the worktree may hold uncommitted work.
- **Rename** (`ctrl+o R`): retitle in place. Title is display-only, so this
  is a `state.json` edit and a sidebar rebuild.
- **Restart a crashed session** (`ctrl+o r`): the crash frame has advertised
  this since #13 (issue #15). Relaunch with `--resume` in place. Small once
  #36 exists.
- **Fuzzy switcher** (`ctrl+o /`): type a few letters, jump to a session
  across all projects. `j/k` stops scaling past about eight sessions.

**Done when:** a session can be created, renamed, crashed, restarted,
archived and its worktree removed, all from inside omatty.

## M5 - File tree

**Delivers:** #24 - browse each session's worktree, see which files that
session touched, preview one without leaving omatty.

Unchanged from the original plan. Sits after Lifecycle because it is one
feature against four, and the four are what you reach for daily.

## M6 - Persistence

**Delivers:** sessions outlive omatty. Quit, close the terminal, reboot the
shell; every claude is still running mid-turn and reattaches on relaunch.

**How:** each `claude` runs under `dtach` (tiny, no UI of its own) rather
than tmux, so omatty inherits no prefix key, no status bar and no nested
multiplexer. On relaunch omatty attaches to the socket instead of starting a
new process. #36's `--resume` remains the fallback when the socket is gone.

**Why this late:** it was cut from v1 and you confirmed it is not the top
pain. It is the biggest technical risk left - nested terminal emulation on
top of a detached PTY - and it is fully isolated from everything above it. It
belongs after the tool has proven itself, not before.

**Done when:** you quit omatty mid-turn, relaunch, and the turn finishes on
screen.

## M7 - Reach

**Delivers:** the open-source door.

- **Config file** (`~/.omatty/config.toml`): leader key, claude binary path,
  worktree root, default base branch. All hardcoded today. First because the
  other two need it.
- **Mouse**: click a session row, scroll the pane. bubbleterm already forwards
  mouse events; the sidebar needs hit-testing.
- **Other agents** (Codex, opencode): an agent is a command template plus a
  status adapter. The seam is kept thin on purpose; this is the milestone
  that widens it.

Nothing in M1-M6 is allowed to bake in a personal path or assumption that
M7 would have to undo. That is the cost of "open source later" and it is
paid continuously, not here.

---

## Not on the roadmap

Considered and cut, so they do not creep back in through the side door:

- Claude self-reviewing its own diff
- Commit / push / PR from inside omatty
- Running N sessions on one task and comparing the results
- Broadcasting one prompt to several sessions
- SSH / remote sessions
- Themes beyond one, or keybinding customisation beyond the leader
- Any orchestration, planning board, or agent-to-agent messaging. omatty is a
  window, not an orchestrator.

## Rules

1. **Every bug gets a failing test before the fix.** Procedure in AGENTS.md.
2. **Every milestone ends with a real-binary smoke test in a sized PTY.**
   M1's three worst bugs - sessions dying at start (#31), terminals never
   pumped (#33), sessions dying on restart (#36) - all passed a 92% coverage
   gate, because every test substituted a fake for claude. Coverage measures
   units; these were failures of the wiring between them. `testdata/`
   carries a PTY harness for this; using it is part of "done".
3. **A milestone is not done while a blocker is open.** M1 is the example.
4. **Invariants are argued, never assumed.** Anything touching the ten in
   AGENTS.md says so in its commit message.
