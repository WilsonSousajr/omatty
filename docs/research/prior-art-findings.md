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

## Pass 4 additions, 2026-10-28

> The projects `2026-landscape.md` §9 found, held against omatty on
> `develop` at v0.11.0. Each competitor feature names the file or release
> note it was read in on 2026-10-28; each omatty line names the package that
> does it, read in this tree. Package paths are the ADR 0001 layout, not the
> pre-migration paths the sections above use. **Analysis only**: the P
> numbers continue the list above and are inputs to `docs/ROADMAP.md`.

### Feature comparison

| Feature | Who ships it, and where it was read | omatty |
|---|---|---|
| Undo a session's last turn | Tellurion: "**Undo last turn** puts your files back" (README) | **Has it.** `tui/app/revert.go` reverts to the turn baseline that `infra/vcs/turn.go` snapshots with `write-tree`. |
| Line comments sent back to the session in one message | omux (`Changes.swift`, `sendNotes`), local-review (`hooks/register.tsx`), paseo 0.12.0 (#6455), herdr-reviewr | **Has it**, and anchored on content (`domain/review/anchor.go`, invariant 7) where all four use line numbers (`2026-landscape.md` §9.4). |
| A comment on part of a line | local-review keeps `code` on the comment (`hooks/viewer.tsx`) | **Has it.** `Comment.Fragment` (`domain/review/comments.go`, #339). |
| Pull request state on each worktree session | omux runs `gh pr view` "about once a minute" (README) | **Has it.** `tui/app/prpoll.go` over `infra/forge`, across four forges (M16). |
| The gate must be green before the work ships | pullboard: "Submit needs a green gate" (`src/commands/submit.js`) | **Has it.** `tui/app/ship.go` refuses with "the gate is not green". |
| Hooks passed at launch, user config untouched | gilvt: "injected through launch" (README) | **Has it.** Invariant 3; `infra/hooks`. |
| Don't re-run a gate on a tree that already passed | pullboard: a green run of a clean tree writes `pullboard-gate-green` with the tree hash, and the next run on that tree returns `isCached` (`src/gate.js`, `isStampedGreen`) | **Gap, P1-5.** |
| Refuse to send while the agent is at a permission prompt | herdr-reviewr v0.42.0: "Send refuses an agent at a permission prompt, which drops a paste, and keeps every comment" (release notes) | **Gap, P1-6.** |
| Side-by-side diff | herdr-reviewr v0.47.0 `diff_view = "split"` (release notes); `neteye-platform/cc-split-diff-view` (mod) | **Gap, P2-5.** Unified only. |
| The operator's own text above every review | herdr-reviewr v0.48.0 `review_prefix` (release notes, #129) | **Gap, P2-6.** |
| Read the pull request draft before it is opened | praft: title (editable), description, size and commits in a pane, then approve (README) | **Gap, P2-7.** |
| A session's subagents, live | omux reads `~/.claude/projects/<project>/<session>/subagents/agent-<id>.jsonl` (`Sources/omux/Agents.swift`) | **Gap, P2-8.** |
| Parallel sessions colliding over ports and processes | claude-bulkhead: hooks that block `pkill node` and a second server on a taken port, and reap orphans (README) | **Gap, P2-9**, but not the way bulkhead fills it (see below). |

### P1-5. The gate re-runs on a tree that has already passed

`autoGate` (`tui/app/gaterun.go`) runs the gate whenever a turn ends, and
`runGate` checks only that a run is not already going. A turn that answered a
question and edited nothing runs the whole line again: on this repository,
`go test -race` and the coverage and C.R.A.P. scripts. Pullboard keys a
green result to the tree hash and skips the run when the tree has not
changed (`src/gate.js`). omatty already computes that hash for the turn
baseline (`infra/vcs/turn.go`, `write-tree` on a temporary index, which
includes untracked files). So the input exists and only the comparison is
missing.

The design questions are omatty's, not pullboard's. A red result must never
be cached, because flaky steps exist and a re-run is how you find them. The
card must say "unchanged since it passed at 14:02", not show a fresh green
it did not earn. Changing the gate's own definition must invalidate the
cache. An explicit `ctrl+o g` should always run.

### P1-6. A review or gate send can land in a permission prompt

`submitReview` (`tui/app/reviewkeys.go`) and `submitGate`
(`tui/app/gatepane.go`) check that the session has a terminal, and the gate
send checks that it is not a repeat. Neither checks the session's status.
omatty knows when claude is waiting on a permission: `PermissionRequested`
is a status kind (`domain/status/event.go`). herdr-reviewr found that a
paste into a permission prompt is dropped. If omatty behaves the same, the
comments are marked sent (`cs.MarkSent`) when they never arrived. That
would be a bug, not a missing feature.

**Confirmed on 2026-10-28, and worse than a dropped paste: #745.** A PTY
probe ran real `claude` 2.1.312 with `--permission-mode default` until it
reached its dialog, then wrote `paste.BracketedPaste`'s exact bytes. The
trailing `\r` selected "Yes", and the pending `touch` ran. The comments
appear nowhere in the transcript. A control run that wrote nothing stayed
blocked. So `S` at a permission prompt approves a tool call nobody approved
and marks unsent comments sent. The fix refuses the send and keeps the
comments pending, as herdr-reviewr does, with a regression test first
(AGENTS.md).

### P2-5. Side-by-side diff

Two projects shipped it in the window. omatty's review column is unified. At
the widths omatty's layout leaves for review, split may only be worth it in
a zoomed review. That is a question for the ROADMAP, not evidence of demand.

### P2-6. A review prefix

One config key (`review_prefix`) puts the operator's standing text, such as
"address each comment, then run the tests", above every composed review.
`domain/review.Compose` is the one place it would go. herdr-reviewr's #129
is one user asking. It is small, and it is cheap to say no to.

### P2-7. Read the pull request before opening it

`infra/forge/ship.go` opens the pull request with the session's title and
`gh pr create --fill`, so the body is the branch's commit messages, and that
choice is deliberate ("omatty composes nothing on their behalf"). praft
shows the title, body, size and commits *before* the request exists and
lets you edit the title. omatty would keep its refusal to write the body and
show a preview of what `--fill` will produce, with the title editable. The
ship path is where a wrong title is most expensive to take back.

### P2-8. A session's subagents

Claude Code writes each subagent's transcript to
`<session>/subagents/agent-<id>.jsonl` beside the session's own (omux,
`Agents.swift`, which also reads its `.meta.json`). That is structured
data, so reading it is within invariant 2. The card could say "3 subagents
· 1 running" where it now says only "working". omatty does not read the
folder today: no file under `internal/` names it. Invariant 9 does not
apply, because a subagent is not a session omatty resumes. The cost is a
second tail per running subagent.

### P2-9. Parallel sessions sharing one machine

bulkhead cites Claude Code issues #11502 and #25180 for the problem:
worktrees isolate files, not ports or processes. Several sessions in
worktrees of one app is omatty's normal case. One thing omatty can do
within its rules is give each session a stable, distinct environment
variable at launch, such as a port base derived from the session, that a
project's own `dev` script may read. That costs nothing for projects that
ignore it. Blocking commands is the part not to copy (below).

### Ideas Not To Copy, pass 4

| Idea | Where it ships | Why not |
|---|---|---|
| A second agent verifies the commit, and the tool accepts or rejects the agent's submit | pullboard ("a different agent verifies that exact commit", README) | **"omatty is a window, not an orchestrator"** (`docs/ROADMAP.md`). The gate's verdict goes to a person's card, and the person decides what goes back. |
| Waivers and flake quarantine keyed on test names parsed from output | pullboard (`tapTests`, `waivedFailures` in `src/gate.js`, `src/commands/gate.js`) | **Invariant 12.** A red run turned "waived, not green" by matching TAP rows lets output text change what a person reads as the verdict. If a step is flaky, the project changes its own line. |
| "Quiet — check it" when the screen has not changed for five minutes | neurosquad-cli (README, `quietAfter`) | **Invariant 2.** The need is real: a prompt from `git`, `sudo` or ssh fires no hook. The admissible version is "working, and no transcript line for N minutes", from the transcript omatty already tails (`domain/status/idle.go`), with no claim about what the screen shows. |
| Hooks that block an agent's commands (`pkill`, a second server on a taken port) | claude-bulkhead (README) | **Invariant 11.** omatty's hook must never block or fail an agent. A project that wants bulkhead can install it, and omatty passes its settings through untouched (invariant 3). |
| Verdicts parsed from a tool's diagnostics or a runner's output | `claude-code-mod-lens`, `agent-race` (`2026-landscape.md` §8.3) | **Invariant 12.** R17's "only what this change brought" idea is separate, and stands. |
| Approvals from a phone | Tellurion, pitupitu (READMEs) | **Refused: cloud, accounts, sync.** It also answers a permission prompt for a session the operator is not looking at, which is the half of the loop omatty keeps with the person. |

### Bottom line, pass 4

Most of what the new projects lead with, omatty already does: undo a turn,
line comments back into the session, PR state on the card, a green gate
before shipping, zero footprint. Its anchor is still the only one on
content. Two gaps became issues. P1-5 (#744) saves the most common wasted
gate run. P1-6 turned out to be a real bug (#745): a send at a permission
prompt approves the tool call. P2-5 to P2-9
are small, independent, and each has someone else's users behind it. Six
more ideas are refused, each with its rule.
