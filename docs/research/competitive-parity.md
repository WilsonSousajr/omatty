# Competitive parity — is the moat real?

> A self-critical internal audit, 2026-09-18. Sibling of the public
> `comparison.md` (#301, the fair pitch) and of `2026-landscape.md` and
> `prior-art-findings.md`. This document asks the question a pitch cannot:
>
> **In the goals where omatty overlaps a competitor, is it actually better —
> or did it arrive at the same ideas without improving on them? If someone
> already uses X, is switching worth it? And is the thing omatty calls its
> moat actually lazygit plus a CI badge?**
>
> Every omatty claim below was verified against the code and names its package.
> A moat line that cannot name a file was deleted before this document was
> committed, and three were. Competitor claims trace to the deep dives.

## The migration bar

Someone leaves a tool only if the new one (a) does the thing they rely on at
parity or better, and (b) adds something they cannot get where they are. Both
halves, or it is not a migration — it is a demo.

So the audit rests on two honest ledgers: the verified moat, and the real gaps.

## omatty's verified moat

Seven lines. No competitor found in this pass has more than three of them, and
none has all seven — but read the qualifiers, because two of these are weaker
than `README.md` implies.

1. **A project's own check line, run per session, with the verdict on the card
   and the failures sent back into the session that caused them.**
   `internal/gate`, `internal/coverage`, the Runner, the card strip, `S`.
   Across every README read in this pass — terminal, desktop and board camps —
   **nobody else has this.** The strongest line in the ledger.
2. **Coverage on the diff.** M10: of the lines a session added, the ones no test
   covers, marked per file. `internal/coverage`. Nobody else, and it is
   downstream of (1), which is why nobody else can have it cheaply.
3. **Comments anchored on content, not line numbers.**
   `internal/review/anchor.go`, `place.go`, invariant 7. Orca — 71,802 stars —
   stores `lineNumber` and flags a note stale when the file's diff identity
   changes, which includes its added/removed counts, so one unrelated edit
   invalidates every note in the file. Narrow, verified, real.
4. **Status from hooks and the transcript, never the screen.**
   `internal/hooks`, `internal/watcher`, invariant 2. claude-squad matches
   English UI strings in a tmux pane; ccmanager regex-matches a box drawing.
   **fleet does this too**, so it is a two-way property, not a moat against
   the whole field.
5. **The real binary, interactive, in a pane you type into, over SSH on a
   headless box.** `internal/termwrap` owns the PTY; invariant 1 routes every
   key but `ctrl+o` into it. The first party's `claude agents` watches
   *background* sessions; Orca reaches a remote box from a desktop app.
   Terminal-native and interactive together is, so far, unmatched.
6. **The process survives a quit.** `internal/detach`, dtach. ccmanager
   restores session *records* and starts new processes; omatty keeps the
   process. Stronger guarantee, and it costs a dependency and produced #191.
7. **Several repositories in one view.** `internal/registry`,
   `internal/discover`, the sidebar. **This is not a moat.** ccmanager shipped
   Multi-Project Mode with recursive discovery; fleet groups sessions by repo.
   It is validated demand — claude-squad #56, #299 and #238 are all open asks
   — and `README.md` is wrong to imply omatty is alone in it.

## Honest gaps

Where "better" is not true today. From `prior-art-findings.md`, restated as
what a migrating user would lose:

- ~~**A worktree that can actually run the project.**~~ Closed by #309:
  `omatty carry` copies a project's gitignored paths into every new worktree,
  before claude starts in it. ccmanager's `.worktreeinclude` and fleet's
  `.fleet.json` keep the list in the repository; omatty keeps it per project
  in `state.json`, so a clone cannot choose what is copied off your disk. The
  cost of that choice is that the list is not shared with a team.
- ~~**PR and CI state.**~~ Closed by #310, released in v0.3.0: `internal/forge`
  reads the pull request and its checks through `gh`, and the session card
  carries the verdict. fleet shows it on the row; omatty shows it on the card.
- **Breadth of agents.** ccmanager supports eight agent CLIs; claude-squad is
  agent-agnostic through one `--program` flag. omatty has claude and a
  half-spiked Codex (#152).
- **Windows.** ccmanager runs there. omatty does not, and neither does
  claude-squad (#275).
- **Distribution**, partly closed. v0.3.0 shipped a Homebrew cask and four
  release archives with checksums (#327, #361), so "nothing for omatty" is no
  longer true. What claude-squad still has and omatty does not is a one-line
  installer, and neither a Linux package (apt, AUR, nix).
- **Scrollback after a reattach** (#191's remainder, open as #336), **a scratch
  shell** (fleet's terminal drawer), and **session forking** (fleet).
- ~~**Review scoped to "since my last read"**~~ (Orca #11840). Closed by #337,
  released in v0.6.0: `v` marks a file read and it says `~` once its diff
  changes. #311 had shipped scoping by *time*, which is a different question
  from scoping by what you have read, and both now exist.
- ~~**A comment that knows it was sent**~~ (Orca's `sentAt`). Closed by #335,
  released in v0.3.0.

No count of what is left is written here on purpose. One was, and it was stale
within days: this list is read against the board, which is where the open
issues actually live.

## Did omatty copy without improving?

The premise worth testing, because arriving second at a good idea is not a
moat.

| Idea | Who else | Did omatty improve on it? |
|---|---|---|
| A session list with live state | everyone, now including `claude agents` | **No — parity at best.** The list is table stakes and the first party gives it away. |
| Worktree per session | everyone | **No.** Parity. |
| Comment on a diff, send it back | Orca, vibe-kanban, Nimbalyst | **Yes, narrowly and verifiably** — content anchoring with two-pass resolution against line numbers with staleness flagged. One property. |
| Status without screen-scraping | fleet | **No — parity with fleet**, ahead of claude-squad and ccmanager. |
| Several repositories | ccmanager, fleet | **No.** Arrived at independently; ccmanager shipped discovery too. |
| Running the project's checks per session | **nobody** | **Not an improvement — an addition.** This is the whole moat. |

Read plainly: **omatty's differentiation is one feature and one property.** The
gate (with coverage-on-the-diff downstream of it) and content anchoring.
Everything else is parity or catch-up. That is a narrower claim than `README.md`
makes, and it is defensible precisely because it is narrow.

## Is the moat lazygit plus a CI badge?

The question this audit exists to answer, since omatty competes for the
verification hour rather than the parallelism hour.

**The honest concession first.** For **one** session in **one** repository, the
incumbent is very strong and free: run `claude` in one pane, `lazygit` in
another, `make test` in a third. lazygit's diff is better than omatty's — 82,458
stars of better — `delta` and `difftastic` render better, and CI's verdict is
authoritative where omatty's gate is local. A single-session user gains
little and gives up a diff viewer they know.

**Where the combination stops working**, and it is not a matter of taste:

- lazygit shows the working tree. It has no idea which *session* produced a
  change, and with several sessions in several worktrees, "which diff am I
  looking at" becomes a manual step per session.
- There is no route from lazygit back into the agent. Feedback means selecting
  text, switching panes, and retyping it — and `internal/review`'s anchoring
  exists because the file moves while you do that.
- CI's verdict requires a push and minutes. omatty's gate is the same commands
  in the session's own worktree, before the push, with the failures going back
  into the session that caused them in one keystroke.
- None of the three knows a session is waiting for you.

**The verdict, stated as a scope rather than a win:** the moat is real at
**N sessions across M repositories**, and thin at N=1, M=1. omatty should say
so. A pitch that claims to beat lazygit is a pitch a reader disproves in one
minute; a pitch that says "lazygit is better at diffs, and it cannot tell you
which of your six sessions just went red" is one they can check and believe.

## Migration verdicts

**From `smtg-ai/claude-squad` — yes, for most of its users.** omatty is better
on the four things its tracker complains about: status that does not break when
tmux does (10 of 133 issues), multiple repositories (#56, #299, #238 — all
open), a review loop that sends comments back, and a gate. They give up
Homebrew, `--autoyes` (deliberately), Windows (neither has it), and tmux
muscle memory. **The clearest migration case in the field**, and it is helped
by claude-squad being four weeks quiet.

**From `kbwo/ccmanager` — only for people who want to judge the work.** At
parity or worse on breadth: eight agents to our one and a half, devcontainers,
Windows, `.worktreeinclude`. Better on exactly two axes, and they are the two
that matter to omatty's buyer: status that is not a regex over a box drawing,
and a review-plus-gate loop ccmanager does not attempt. **A real migration for
a reviewer; a downgrade for a session manager.** Say so.

**From `brizzai/fleet` — genuinely close, and not obviously worth it.** Same
architecture, same hooks, same repo grouping, in the same language. fleet is
ahead on worktree files, PR state, a scratch shell and forking; omatty is
ahead on review, the gate, and owning the PTY instead of tmux. If P0-1 and
P1-1 land, this flips. Today it is honest to say: if you already run fleet and
do not need a review loop, stay.

**From `stablyai/orca` — different buyer, not a migration target.** Desktop,
mobile, fleets of agents fanned across worktrees, a whole orchestration
product. Its users who want a terminal will not have chosen it. The overlap is
the review loop, where omatty has one verifiable advantage and Orca has five
features omatty lacks.

**From `BloopAI/vibe-kanban` — different buyer entirely.** Tickets and workers.
M9 refuses that shape by name. Not a comparison; a disagreement.

**From `claude agents` (first party) — the one that matters for new users, and
the hardest.** It is free, in the box, needs no install, and does the session
list and worktree isolation. omatty must justify itself on what agent view is
not: interactive panes rather than background sessions, several repositories
rather than one working directory, a review loop, and a gate. All four are
true today. None of them is in `README.md`'s opening paragraph.

## The two repairs this audit owes the repository

### 1. `README.md`'s field claim is false. Replace it.

Current: *"Every other tool in this space is either a desktop app or scoped to
a single repository."* False — `kbwo/ccmanager` is a terminal tool with a
documented Multi-Project Mode, and `brizzai/fleet` groups sessions by repo in a
Go TUI.

What survives contact with the table, and what #301 should write instead:

> omatty is terminal-native — it works over SSH on a headless box — and shows
> sessions from several repositories side by side. Other terminal managers do
> the second part; the desktop apps do neither. What none of them do is run
> the project's own check line in each session's worktree and put the verdict
> on the card.

The claim moves from *"we are the only terminal multi-repo tool"* (false) to
*"the gate is ours alone"* (true as of this capture, §3.2 of the landscape),
and it survives the first party shipping `claude agents`, which the current
sentence does not.

### 2. "Roughly a hundred and fifty agent orchestrators" is unsourced. Soften it.

`2026-landscape.md` §2 probed it: `topic:parallel-agents` 184,
`claude code parallel in:description stars:>10` 205,
`coding agent orchestrator in:description stars:>10` 462,
`claude code worktree in:description` 1,300 — none filtered for maintenance.

The number survives as an order of magnitude only. M9's argument does not need
it: it needs "a field full of tools that optimise how much agent-work is in
flight", which every document in `docs/research/` now supports by name. Either
cite a query with its date or drop the figure.

## Bottom line

The moat is **one feature and one property** — the gate, and content anchoring
— scoped to several sessions across several repositories. It is real, it is
narrower than the README claims, and two of the seven "moat" lines are parity
rather than advantage. The field validated the architecture on two counts
(hooks over screen-scraping, multiple repositories) and commoditised the
headline on a third (`claude agents`). The correct response is not to widen the
surface. It is to close P0-1 and P1-2, which are inside the thesis, and to say
a narrower true thing in `README.md` than the broad false thing it says now.
