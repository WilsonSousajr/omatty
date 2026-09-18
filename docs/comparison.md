# How omatty compares

For anyone evaluating omatty against another way of running several coding
agents — or migrating from one. This page aims to be **fair and specific**:
what each approach does well, where omatty differs, and where others are
ahead. The analysis behind it is in [`docs/research/`](research/), captured
2026-09-18 from primary sources; every number and every competitor claim here
traces to a file path or an issue number there.

Where a claim about omatty appears below, it names the package that implements
it. Where a competitor is ahead, it says so.

## The short version

Most tools for running several agents optimise **how much agent-work can be in
flight at once**: fleets, worktrees, boards, queues. omatty optimises the other
thing — **how quickly you can tell whether what came back is any good.**

Concretely, and these are the only two claims worth checking first:

- **A project carries its gate** — its own `fmt`/`vet`/`lint`/`test`/coverage
  line. omatty runs it in each session's own worktree, puts the verdict on that
  session's card, and sends the failures back into the session that caused them
  with one keystroke (`internal/gate`, `internal/coverage`). As of September
  2026, **no other tool in this space does this** — not the terminal managers,
  not the desktop apps, not the boards.
- **Comments anchor to the content of a line, not its number**
  (`internal/review/anchor.go`, invariant 7). Claude edits files while you read
  them. A line-number anchor silently attaches your feedback to the wrong code;
  omatty re-resolves by content, and a comment it cannot place becomes an
  orphan of its file rather than a wrong line.

Everything else omatty does, someone else also does. That is deliberate, and
the rest of this page is about it.

## By camp

**Terminal session managers** — `smtg-ai/claude-squad` (tmux + `gh`, AGPL),
`kbwo/ccmanager` (no tmux, eight agent CLIs, Windows), `brizzai/fleet` (Go,
tmux, PR state on the row). This is omatty's camp. They are good at running and
switching between sessions; none of them has a review loop that sends comments
back, and none has a gate.

**Desktop and hybrid workspaces** — `stablyai/orca` (the field's largest by
far), `nimbalyst/nimbalyst` (formerly Crystal), `imbue-ai/sculptor`, Conductor.
Better rendering, mobile companions, cloud sessions. Not reachable over SSH on
a headless box.

**Boards and queues** — `BloopAI/vibe-kanban` and the rest of the camp, which
is an order of magnitude larger than the terminal camp. Work is tickets, agents
are workers. omatty disagrees with this shape rather than competing with it;
[`ROADMAP.md`'s "Not on the roadmap"](ROADMAP.md) says why, feature by feature.

**Claude Code itself.** `claude agents` — in research preview — gives you "one
screen for all your background sessions", each isolated in a worktree under
`.claude/worktrees/`. It is free and in the box. If a session list with live
state is what you want, **use it**; omatty is not a better version of it.

**Diff and verification tools** — `lazygit`, `delta`, `difftastic`, `gh`, CI.
See "Is this just lazygit?" below, because it is the right question.

## Where omatty is different

- **Terminal-native and interactive.** The real `claude` binary runs in an
  embedded pane you type into, with every keystroke routed to it except the
  `ctrl+o` leader (`internal/termwrap`, invariant 1). It works over SSH on a
  headless box. `claude agents` watches *background* sessions; the desktop apps
  need a desktop.
- **Several repositories side by side** (`internal/registry`,
  `internal/discover`). ccmanager and fleet do this too — see below.
- **Status from hooks and the transcript, never from the screen**
  (`internal/hooks`, `internal/watcher`, invariant 2). fleet does this too;
  claude-squad matches English UI strings in a captured tmux pane and ccmanager
  regex-matches Claude's drawn prompt box, and both carry a long tail of
  wrong-status bugs for it — claude-squad #51, #216, #189, #132; ccmanager
  #227.
- **Sessions outlive the app.** With `dtach`, quitting detaches rather than
  ends (`internal/detach`). ccmanager restores the session *records* and starts
  fresh processes; omatty keeps the process.
- **It does not delegate, plan, schedule or decide for you.** No coordinator,
  no agent-to-agent messaging, no unattended queues, no cloud. Each of those is
  refused with a stated reason in "Not on the roadmap".

## Where others are ahead

The list that makes the rest of the page worth reading.

| | Who | What omatty lacks |
|---|---|---|
| A worktree you can actually run | ccmanager (`.worktreeinclude`), fleet (`.fleet.json`) | Carrying gitignored files — `.env`, local certs — into a new worktree. omatty creates the worktree and stops. **The most-requested missing feature in the whole field**, and omatty has no answer. |
| PR and CI state | fleet | omatty shows the local gate verdict and nothing about the remote one. |
| Review scoped to *since when* | Orca (#11840, open) | omatty shows everything a session changed; there is no "changes this turn". |
| Breadth of agents | ccmanager (8 CLIs), claude-squad (`--program`) | omatty has Claude Code, and Codex half-spiked (#152). |
| Windows | ccmanager | omatty is Unix-only. |
| Distribution | claude-squad (Homebrew, one-line install) | omatty has neither. |
| Scrollback after a reattach | Orca | Lost; #191 repainted the pane, persisting the grid is still open. |
| Diff rendering | lazygit, delta, difftastic | They are better at this, by a wide margin. |

## Is this just lazygit and a CI badge?

The fair version of the objection, and it deserves the fair answer.

**For one session in one repository, largely yes** — and the incumbent is free.
Run `claude` in one pane, `lazygit` in another, `make test` in a third.
lazygit's diff viewer is better than omatty's, `delta` renders better, and CI's
verdict is authoritative where omatty's gate is local. If that is your setup,
omatty will not earn its install.

**What stops working at several sessions across several repositories:**

- lazygit shows the working tree. It does not know which *session* produced a
  change, so with six worktrees, "which diff am I looking at" is a manual step
  six times over.
- There is no route from lazygit back into the agent. Feedback means selecting,
  switching panes and retyping — and the file moves while you do it, which is
  the reason omatty's comments anchor to content.
- CI's verdict needs a push and minutes. omatty's gate runs the same commands
  in the session's own worktree before the push, and `S` sends the failures
  back into the session that caused them.
- None of them tells you a session is waiting for you.

So: **the case for omatty is N sessions across M repositories.** At N=1, M=1 it
is thin, and saying otherwise would be the sort of claim you could disprove in
a minute.

## Coming from another tool?

**From claude-squad** — the clearest case. You gain status that does not break
when tmux does, several repositories in one view (its #56, #299 and #238 are
all still open asking for this), a review loop that sends comments back, and a
gate. You give up Homebrew, `--autoyes` (omatty refuses it on purpose), and
tmux muscle memory.

**From ccmanager** — only if you want to *judge* the work. ccmanager is ahead
on breadth: eight agents, devcontainers, Windows, and `.worktreeinclude`, which
you will miss on day one. omatty is ahead on two things: status that is not a
regex over a box drawing, and a review-plus-gate loop ccmanager does not
attempt. As a pure session manager, moving is a downgrade.

**From fleet** — close, and possibly not worth it. Same architecture, same
hook-driven status, same repo grouping, same language. fleet is ahead on
worktree files, PR state, a scratch-shell drawer and session forking; omatty is
ahead on the review loop, the gate, and owning the PTY instead of tmux. If you
already run fleet and do not want a review loop, stay.

**From Orca, Nimbalyst or vibe-kanban** — different products for a different
buyer. If you want a desktop app, a mobile companion, or a ticket board driving
agents, omatty is not a smaller version of those; it disagrees with them.

**From `claude agents`** — it is free and in the box. Come to omatty for the
things it does not do: interactive panes rather than background sessions,
several repositories rather than one working directory, a review loop, and a
gate.

## Honest summary

omatty's differentiation is **one feature and one property** — the gate, and
content-anchored comments — and it is worth having at several sessions across
several repositories. Most of the rest is parity with a camp that has
independently converged on the same shape, and one part of it (the session
list) is now shipped by Claude Code itself.

The detailed, self-critical version of this page — including the moat lines
that were deleted for not being true — is
[`docs/research/competitive-parity.md`](research/competitive-parity.md).
