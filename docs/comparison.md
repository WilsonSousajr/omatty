# How omatty compares

For anyone evaluating omatty against another way of running several coding
agents, or migrating from one. This page aims to be **fair and specific**:
what each approach does well, where omatty differs, and where others are
ahead. The analysis behind it is in [`docs/research/`](research/), captured
2026-09-18 and refreshed 2026-09-27 from primary sources
(`2026-landscape.md` §7, `herdr.md`); every number and every competitor claim
here traces to a file path or an issue number there.

Where a claim about omatty appears below, it names the package that implements
it. Where a competitor is ahead, it says so.

## The short version

Most tools for running several agents optimise **how much agent-work can be in
flight at once**: fleets, worktrees, boards, queues. omatty optimises the other
thing: **how quickly you can tell whether what came back is any good.**

Concretely, and these are the claims worth checking first:

- **A project carries its gate**: its own `fmt`/`vet`/`lint`/`test`/coverage
  line. omatty runs it in each session's own worktree, puts a verdict per step
  on that session's card, and sends the failures back into the session that
  caused them with one keystroke (`internal/gate`, `internal/domain/coverage`).
  **As of 2026-09-27, no session manager or agent workspace above about a
  hundred stars does this.** The larger tools show the *remote* verdict, the
  pull request's CI, and so does `claude agents`. Four small tools each do part
  of it locally, judged by exit status as omatty is: `nccapo/stvena` (one
  command), `axonel/axonel` (one command per mission), and two herdr plugins,
  `herdr-plugin-odysseus` and `herdr-testrun` (`2026-landscape.md` §7.4). The
  claim is dated because it will not stay true.
- **Comments anchor to the content of a line, not its number**
  (`internal/review/anchor.go`, invariant 7). Claude edits files while you read
  them. A line-number anchor silently attaches your feedback to the wrong code;
  omatty re-resolves by content, and a comment it cannot place becomes an
  orphan of its file rather than a wrong line. Orca and herdr-reviewr both
  anchor by line number (`orca.md`; `herdr.md` §6).
- **Status from hooks and the transcript, never from the screen**, and **no
  write to `~/.claude/settings.json`** (invariants 2 and 3). herdr reads
  Claude Code's state from a screen manifest and installs its hook into your
  settings (`herdr.md` §4-5).

Everything else omatty does, someone else also does. That is deliberate, and
the rest of this page is about it.

## By camp

**Terminal session managers**: `herdrdev/herdr` (Rust, ~41k stars, owns the
PTYs, many agents, several machines, plugins), `smtg-ai/claude-squad` (tmux +
`gh`, AGPL), `kbwo/ccmanager` (no tmux, nine agent CLIs, Windows),
`brizzai/fleet` (Go, tmux, Claude Code, Codex and OpenCode, PR state on the
row). This is omatty's camp. They are good at running and switching between
sessions. herdr, with its `herdr-reviewr` plugin, also has a review loop that
sends comments back, anchored by line number. None of them has a gate in
core.

**Desktop and hybrid workspaces**: `stablyai/orca` (the field's largest by
far; runs any CLI agent), `nimbalyst/nimbalyst` (formerly Crystal),
`imbue-ai/sculptor`, Conductor. Better rendering, mobile companions, cloud
sessions. Orca also runs headless (`orca serve`,
`docs/reference/headless-linux-server.md`) and has SSH worktrees.

**Boards and queues**: the camp `BloopAI/vibe-kanban` led, which is an order
of magnitude larger than the terminal camp. vibe-kanban itself is sunsetting
(its README banner since 2026-04-24). Work is tickets, agents are workers.
omatty disagrees with this shape rather than competing with it;
[`ROADMAP.md`'s "Not on the roadmap"](ROADMAP.md) says why, feature by feature.

**Claude Code itself.** `claude agents`, in research preview, lists every
background session you have started "across all your projects", each isolated
in a worktree, attaches to one interactively, and colours each session's pull
request by its checks (`code.claude.com/docs/en/agent-view`). It is free and
in the box. If a session list with live state is what you want, **use it**;
omatty is not a better version of it.

**Diff and verification tools**: `lazygit`, `delta`, `difftastic`, `gh`, CI.
See "Is this just lazygit?" below, because it is the right question.

## Where omatty is different

- **Several live panes, each the real `claude` binary.** The binary runs in an
  embedded pane you type into, with every keystroke routed to it except the
  `ctrl+o` leader (`internal/keys/router.go`, invariant 1). `claude agents`
  attaches to one background session at a time.
- **Status from hooks and the transcript, never from the screen**
  (`internal/infra/hooks`, `internal/service/status`, invariant 2). fleet is hook-driven
  too, with pane heuristics as a fallback for states no hook reports.
  claude-squad matches English UI strings in a captured tmux pane, ccmanager
  regex-matches Claude's drawn prompt box, and herdr runs a regex manifest
  over the rendered screen for Claude Code. Screen-reading has cost users
  there: ccmanager #227 is a wrong-status bug, and claude-squad #51, #216 and
  #189 were pane-capture failures (all since closed).
- **Zero footprint in your Claude setup.** Hooks are passed to each session on
  its command line (invariant 3). herdr's Claude integration writes
  `~/.claude/settings.json`.
- **Sessions outlive the app.** With `dtach`, quitting detaches rather than
  ends (`internal/infra/detach`). ccmanager restores the session *records* and starts
  fresh processes; omatty keeps the process. Keeping the process keeps its
  history: after a reattach `pgup` still reaches the whole conversation in
  Claude Code's own pager, a turn that finished while omatty was closed
  included (checked against Claude Code 2.1.283, #336). herdr does the same as
  omatty, and more (see "Where others are ahead").
- **It does not delegate, plan, schedule or decide for you.** No coordinator,
  no agent-to-agent messaging, no unattended queues, no cloud. Each of those is
  refused with a stated reason in "Not on the roadmap".

Several repositories in one window (`internal/registry`, `internal/discover`)
and working over SSH are facts about omatty, not distinctions: `claude agents`,
herdr, ccmanager and fleet span repositories, and herdr, Orca and emdash reach
remote machines.

Several forges are the same kind of fact (M16). omatty reads a project's pull
requests, CI, issues and items on GitHub, GitLab, Azure DevOps,
Gitea/Forgejo/Codeberg and Bitbucket Cloud and Data Center, through each
forge's own CLI or its REST API (`internal/infra/forge`), and ships to each with
`ctrl+o p`. The README's Forges table says which of those a real run has
shown and which are still untested. Others got there first on the forges most
teams use: Orca's cards carry GitLab merge requests and their CI
(`issues-orca.md` B, #18484), and herdr-reviewr's read-only pull request tab
covers GitHub, GitLab and Azure DevOps (`herdr.md` §6). What omatty adds is
breadth - Gitea, Forgejo, Codeberg and Bitbucket too - and that each forge's
merge sends #331's bounds in its own words.

## Where others are ahead

The list that makes the rest of the page worth reading.

| | Who | What omatty lacks |
|---|---|---|
| Breadth of agents | herdr (twenty-odd), ccmanager (9 CLIs), Orca (any CLI agent), fleet (3), claude-squad (`--program`) | The agent seam exists (#46) and carries one profile. A second is open as #152. |
| Windows | herdr, ccmanager | omatty is Unix-only; the release builds darwin and linux only. |
| Several machines in one window | herdr | omatty runs in the terminal you SSH into; it does not aggregate machines. |
| Upgrading without ending sessions | herdr (`update --handoff`, experimental) | A new omatty binary reattaches to dtach; it does not hand off live PTYs. |
| Distribution breadth | herdr (a core Homebrew formula, `mise`, a Nix flake) | omatty has a Homebrew tap, release archives and a one-line installer (`curl -fsSL https://omatty.com/install.sh \| sh`, #517), but no core formula and no Linux package (apt, AUR, nix). |
| Scrollback of the pane's own | Orca, herdr (`pane_history`, experimental) | omatty's pane keeps none. With Claude Code that costs nothing, since its pager holds the conversation and survives a reattach (#336); a program that keeps no history of its own would lose it. |
| Diff rendering | lazygit, delta, difftastic | They are better at this, by a wide margin. |

## Is this just lazygit and a CI badge?

The fair version of the objection, and it deserves the fair answer.

**For one session in one repository, largely yes**, and the incumbent is free.
Run `claude` in one pane, `lazygit` in another, `make test` in a third.
lazygit's diff viewer is better than omatty's, `delta` renders better, and CI's
verdict is authoritative where omatty's gate is local. If that is your setup,
omatty will not earn its install.

**What stops working at several sessions across several repositories:**

- lazygit shows the working tree. It does not know which *session* produced a
  change, so with six worktrees, "which diff am I looking at" is a manual step
  six times over.
- There is no route from lazygit back into the agent. Feedback means selecting,
  switching panes and retyping, and the file moves while you do it, which is
  the reason omatty's comments anchor to content.
- CI's verdict needs a push and minutes. omatty's gate runs the same commands
  in the session's own worktree before the push, and `S` sends the failures
  back into the session that caused them.
- None of them tells you a session is waiting for you.

So: **the case for omatty is N sessions across M repositories.** At N=1, M=1 it
is thin, and saying otherwise would be the sort of claim you could disprove in
a minute.

## Coming from another tool?

**From herdr**: the closest large tool, and ahead on breadth (the table
above). You gain the gate, comments that stay on the right line while Claude
edits, status that does not depend on Claude's screen, and nothing written to
your Claude settings. You give up other agents, Windows, several machines in
one window, plugins, and general-purpose panes. If you do not want a gate,
stay.

**From claude-squad**: the clearest case. You gain status that does not break
when tmux does, several repositories in one view (its #56, #299 and #238 are
all still open asking for this), a review loop that sends comments back, and a
gate. You give up `--autoyes` (omatty refuses it on purpose), other agents,
and tmux muscle memory.

**From ccmanager**: only if you want to *judge* the work. ccmanager is ahead
on breadth: nine agents, devcontainers and Windows. omatty is ahead on two
things: status that is not a regex over a box drawing, and a review-plus-gate
loop ccmanager does not attempt. As a pure session manager, moving is a
downgrade.

**From fleet**: close, and possibly not worth it. Same architecture, same
hook-driven status, same repo grouping, same language. fleet is ahead on a
scratch-shell drawer, session forking and two more agents; worktree files
(`.worktreeinclude`, #309) and PR state on the card (#310) now exist in both.
omatty is ahead on the review loop, the gate, and owning the PTY instead of
tmux. If you already run fleet and do not want a review loop, stay.

**From Orca or Nimbalyst**: different products for a different buyer. If you
want a desktop app, a mobile companion, or a ticket board driving agents,
omatty is not a smaller version of those; it disagrees with them.

**From `claude agents`**: it is free and in the box, spans your projects and
shows each pull request's checks. Come to omatty for the things it does not
do: several live panes side by side rather than one attached session, a
review loop, and your own check line run per session with the verdict on the
card.

## Honest summary

omatty's differentiation is **one feature and two properties**: the gate,
content-anchored comments, and status that never reads the screen. It is
worth having at several sessions across several repositories. Most of the
rest is parity with a camp that has independently converged on the same
shape, and part of it (the session list across projects) is now shipped by
Claude Code itself.

The detailed, self-critical version of this page, including the moat lines
that were deleted for not being true, is
[`docs/research/competitive-parity.md`](research/competitive-parity.md).
