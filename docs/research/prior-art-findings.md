# Prior art against our own code

> 2026-09-18. Sibling of `ai-memory`'s `prior-art-implementation-findings.md`:
> the pass that turns everything read in `2026-landscape.md`, the four deep
> dives and the five tracker documents into a ledger against omatty as it
> actually is on `develop` — v0.1.0 plus M9, M10 and M11.
>
> **Every "we already do this" line names the package that does it**, read in
> this tree, not the README's claim about it. Gaps are P0/P1/P2 and are inputs
> to #301, which is where the roadmap decides. **Analysis only.**

## Executive summary

omatty already holds the two structural bets the field validates — status off
the screen, and comments that survive the file changing — and holds one square
nobody else occupies, the gate. What the pass found is not a rewrite. It is a
short list of things every competitor treats as a product feature and omatty
treats as the operator's problem, one gap inside omatty's own thesis, and a
pitch that no longer matches the field it describes.

The most important finding is not on this list at all: `claude agents` shipped
(`2026-landscape.md` §3.1). Nothing below changes because of it, and everything
in `README.md` does.

## What omatty already does well, with the package that does it

### Status and liveness

`internal/hooks` (socket, `omatty hook`), `internal/watcher` (transcript tail),
`internal/notify`. Status comes from hook events and the session's JSONL and
never from the cell grid (invariant 2). The field's two most popular terminal
managers both read the screen and both carry a permanent bug class for it —
claude-squad, 10 of 133 issues; ccmanager #227. This was argued from first
principles in M2 with nothing to point at and is now the best-evidenced
decision in the repository.

### Review

`internal/review`: `anchor.go` anchors on `(file, hunk header, content hash,
nth occurrence)`; `place.go` resolves exact-then-content and degrades a
comment to an orphan of its file rather than to a wrong line; `compose.go`
sends one message; `internal/paste` makes it one bracketed paste with a single
`\r` (invariant 8). Orca — the largest tool in the field — anchors on
`lineNumber` and *flags* a stale note rather than repairing it. This is a real
difference, and it is one property rather than a verdict.

### The gate

`internal/gate`, `internal/coverage`, the Runner, the card strip, `S` to send
failures back. Across every README read in this pass, **no tool in the
terminal, desktop or board camps runs the project's own check line per session
and shows the verdict beside it.** The square is empty.

### Several repositories at once

`internal/registry`, `internal/discover`, the sidebar. claude-squad's users have
three open issues asking for it (#56 with 7 comments — its most-discussed open
issue — plus #299 and #238). ccmanager shipped it. This is validated demand and
**not** a unique property; see `competitive-parity.md`.

### Persistence

`internal/detach` (dtach). omatty keeps the process; ccmanager restores the
*records* and starts fresh ones (`sessionRestorer.ts`). Ours is the stronger
guarantee and costs a dependency.

### Owning the PTY

`internal/termwrap`. claude-squad #325 — "Bubble Tea's input reader races the
tmux attach reader for stdin" — is structurally impossible here. So is the
whole "error capturing pane content" cluster.

### The harness

M10 and M11: coverage and C.R.A.P. on the diff, import boundaries and module
hygiene as gate steps that fail. Orca's tracker shows the alternative at scale
— #21395 (225 files invisible to lint and CI), #20185 (static analysis red on
every PR). Not a product feature; a reason the product can be changed safely.

## Gaps

### P0-1. A fresh worktree you can actually run

**Evidence:** the field's most-repeated unmet need — ccmanager #7 (shipped as
`.worktreeinclude`, copy ordered *before* the post-creation hook), claude-squad
#260 and #277 (both open), fleet's `.fleet.json` + `internal/git/exclude.go`.
Four appearances, three projects, two shipped implementations.

**omatty today:** `ctrl+o n` creates the worktree (`internal/vcs`) and stops.
`.env`, local certs, `node_modules` and any port assignment are the operator's
problem, discovered after the fact.

**Why P0 rather than a nice-to-have:** M9 made the gate the product. A gate
step that fails because `.env` is missing is the gate being wrong about the
code, and a red card the operator has to learn to ignore is worse than no card.
This gap sits inside the thesis, not beside it.

**Shape to consider, not to adopt:** a per-project list of gitignored paths to
carry in, copied before anything else runs, in `state.json` where `Gate`
already lives. Both competitors chose a file in the repo; omatty's config
precedent is `Project.Gate`, which is `state.json` plus `omatty gate`. That
tension is the design question and belongs in the issue, not here.

### P1-1. PR and CI state on the session card

**Evidence:** Orca #18484 (CI state on the card, currently wrong), #18485
(unresolved PR comments), #18487 (behind-target); fleet ships it
(`internal/github/pr.go`).

**omatty today:** the card carries the local gate verdict and nothing about the
remote one.

**Why it fits:** it is verification, not orchestration — it reports a verdict
someone else computed and takes no action on it. That is the same shape as the
gate strip and clears "Not on the roadmap" without argument. The line it must
not cross is Orca #10131, "auto-run agents when PR checks fail".

**The honest counter-argument, for #301:** it adds a network dependency and a
`gh` dependency to a tool that currently needs neither, and "Cloud, accounts,
sync" is a stated refusal. Reading a PR's status with the user's existing `gh`
auth is not that, but the boundary needs stating before it is built.

### P1-2. The review pane has no notion of *since when*

**Evidence:** Orca #11840 (open) — "Turn-scoped diff review — 'changes this
turn' / 'since my last review' scopes backed by a refs/orca baseline snapped at
agent turn boundaries".

**omatty today:** `internal/review` shows everything a session changed against
its base. After three turns you re-read three turns.

**Why it matters more here than there:** omatty's entire claim is
time-to-judgement. A reviewer who must re-scan the whole diff every turn is
paying the cost the product exists to remove. This is the one gap in the pass
that is a gap *in the thesis*.

### P1-3. A comment does not know it was sent

**Evidence:** Orca's `DiffComment.sentAt` — "set after the note has been handed
to an agent. Edits clear it."

**omatty today:** `internal/review/comments.go` has no sent state, so composing
twice sends the same feedback twice with nothing to say so.

### P1-4. Scrollback is still lost on reattach

**Evidence:** Orca advertises "scrollback that survives restarts";
claude-squad #60 asks for scrolling in the preview pane.

**omatty today:** #191 got the pane repainted after a dtach reattach and its
own PR recorded that persisting the grid would be a separate issue. It still is.

### P2-1. Per-file "reviewed, and unchanged since"

Orca tracks `isReviewed` and `changedSinceReview` per file. `internal/review/
tree.go` marks changed files; it has no memory of what has been read.

### P2-2. Generated files are reviewed like any other

Orca filters `/build/`, `/coverage/`, `*.generated.*` out of the review queue.
omatty shows every changed file, and M10's coverage markers make a generated
file look worse than it is.

### P2-3. Sub-line and multiple comments per line

Orca #18120. Worth recording that omatty's anchor *generalises* here where a
line number does not: `Anchor{File, Hunk, Hash, Nth}` already disambiguates
repeated lines, and a character range is a fifth field rather than a redesign.

### P2-4. Distribution

claude-squad has Homebrew (#97) and a one-line installer; omatty has neither,
and `$OMATTY_HOME` exists in the smoke recipe rather than as a supported
variable (claude-squad #245 is the same request). Not research, but the pass
surfaced it and `README.md` claims a stranger can clone `main`.

## Ideas Not To Copy

The section that matters most in six weeks, when one of these is proposed again
by someone who has not read this file. Each names the rule it breaks.

| Idea | Where it ships | Why not |
|---|---|---|
| `--autoyes` / bypass-permissions mode | claude-squad (`-y`, #222, #151); ccmanager's experimental AI auto-approval | **"Not on the roadmap": sending anything without being asked.** ccmanager's own README argues against claude-squad's version — "bypasses Claude Code's built-in security confirmations - not recommended for safe operation". A refusal with a competitor's public objection attached is stronger than a refusal from principle. |
| An agent-facing CLI so agents dispatch to each other | `fleet skill install` — an Agent Skill teaching agents `fleet wt`, `fleet send` | **Refused: agent-to-agent messaging, and agents that spawn agents.** Well made, opt-in, and still the thing omatty says it will not build. Its existence is the point: the option was available and the reason for declining has not changed. |
| Auto-run agents when PR checks fail | Orca #10131 | **Refused: unattended task queues, scheduled runs, PR subscriptions.** One step past #233's auto-run, which gates a session and sends nothing. |
| A board inside the TUI | vibe-kanban's whole product | **Refused: two sources of truth.** The board is GitHub project 13. |
| Mobile companion, cloud sessions, account sync | Orca, Nimbalyst | **Refused: cloud, accounts, sync.** |
| Screen-scraped session state | claude-squad, ccmanager | **Invariant 2**, and now with three trackers of evidence. |
| Line-number comment anchors | Orca | **Invariant 7.** |
| Forking a session's conversation | fleet (`f`) | Not refused — unanswered. **Invariant 9** asks the first question: what is the copy's uuid, and which transcript does it claim? fleet #142 and #226 are both identity bugs. If it is ever proposed, that is the question, not the UI. |

## Corrections this pass owes other issues

- **#152 — Codex hooks exist.** `fleet`'s `internal/hooks/codex_hooks.go`
  subscribes to `SessionStart`, `UserPromptSubmit`, `PermissionRequest` and
  `Stop` at `$CODEX_HOME/hooks.json`. The spike's "Codex has no hook mechanism,
  so a Codex session can never show 'waiting for you'" is wrong. **Posted to
  #152 on 2026-09-18.** fleet #220 is the caveat: a partial hook set leaves a
  session confidently wrong, so #152 needs a fallback for the events Codex does
  not send, not just a hook path.
- **`README.md` — the field claim is false.** ccmanager is a terminal tool with
  a documented Multi-Project Mode; fleet groups sessions by repo in a TUI.
  #300 verifies, #301 rewrites.
- **M9's thesis — "roughly a hundred and fifty" is unsourced.**
  `2026-landscape.md` §2 gives queries and counts; the claim survives as an
  order of magnitude and only that.

## Bottom line

Three things to build (P0-1, P1-1, P1-2), two of which the field asked someone
else for and one of which is a hole in omatty's own thesis; four smaller ones;
eight things to keep refusing, now with citations. And one sentence in
`README.md` that is wrong twice over — false about the field, and written
before the first party shipped the category's core.
