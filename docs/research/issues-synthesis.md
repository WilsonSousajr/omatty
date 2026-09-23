# What four trackers say, together

> Cross-tracker reading, 2026-09-18. Sources are the four
> `issues-*.md` documents beside this one: full reads of `smtg-ai/claude-squad`
> (133 issues), `kbwo/ccmanager` (58) and `brizzai/fleet` (54), plus a targeted
> probe of `stablyai/orca` (3,036 open; not read exhaustively, and that
> document says so).
>
> The per-project documents rank each tracker's own pain. This one is the
> reason the pass was worth running: **a need that appears independently in
> three trackers is a different kind of fact from a need that appears in one.**
> Analysis only — #299 turns these into P0/P1/P2 against our code, and #301
> decides.

## 1. The most-repeated unmet need in the field: a usable fresh worktree

Four independent appearances, three projects, two of them shipped:

| Where | What |
|---|---|
| ccmanager **#7** (closed) | "a configure file with ability to copy (typically git ignored) files to worktree on init" — shipped as `.worktreeinclude`, files copied *before* the post-creation hook so hooks can rely on them |
| claude-squad **#260** (open) | "worktree environment setup hook (deps, env files, port isolation)" |
| claude-squad **#277** (open) | new sessions do not inherit custom env vars |
| fleet | shipped: `.fleet.json` workspace commands, `internal/git/exclude.go` |

**omatty has no answer.** `ctrl+o n` creates a worktree and the operator
discovers that `.env` is missing, `node_modules` is absent and the dev server's
port collides with the session next door. Every competitor in the terminal camp
has either built this or has it open as its most concrete feature request.

It is also squarely inside omatty's thesis rather than a stretch of it: a
session that cannot run the project is a session whose work you cannot judge,
and M9 made judging the product. A gate step that fails because `.env` is
missing is the gate being wrong about the code.

**The strongest P0 candidate out of the whole pass.**

## 2. Screen-scraped status is the field's dominant bug class

| Project | Mechanism | Evidence |
|---|---|---|
| claude-squad | literal English UI strings in a captured tmux pane | 10 of 133 issues, including the tracker's most-discussed (#51, 18 comments) |
| ccmanager | regex over Claude Code's drawn prompt box | #227 "State detection shows idle when Claude Code is busy" |
| fleet | **hooks** | one status bug (#220), and only on Codex, whose hook set is incomplete |
| omatty | hooks + transcript (invariant 2) | — |

The correlation is clean enough to state plainly: the two projects that read
the screen have a permanent bug class, and the one that reads hooks has an
incident. Invariant 2 was argued from first principles in M2 with nothing to
point at. It now has three projects' trackers behind it, and `comparison.md`
(#301) should say so in one sentence with the issue numbers, because it is the
rare competitive claim that is verifiable by a reader in thirty seconds.

**Caveat that belongs in the same sentence:** fleet shows hooks are not free.
A partial hook set — Codex sends a busy event and no matching idle one — leaves
a session *confidently wrong*, which is worse than unknown. #152 inherits this.

## 3. The demand is for a verdict on the card. The field's answer is remote; omatty's is local

Nobody runs the project's own check line per session (`2026-landscape.md` §3.2
holds). But three Orca issues and one fleet feature show what users do ask for:

- Orca **#18484** CI state on the card, and it is currently wrong
- Orca **#18485** unresolved PR comment count on the card
- Orca **#18487** merge-conflict / behind-target state on the card
- fleet ships PR state on the row (`internal/github/pr.go`)

omatty's card carries a **local** gate verdict — faster, needs no push, and
narrower, because CI is the authoritative one. The two are complements, not
competitors, and omatty has only one of them. **"PR and CI state on the session
card" is the best-evidenced P1 in the pass**, and it is the one feature the
field wants that omatty's thesis endorses without qualification: it is
verification, not orchestration.

The line to hold is Orca **#10131**, "auto-run agents when PR checks fail".
That is the orchestrator move, refused in "Not on the roadmap", and it sits one step
past #233's auto-run. Worth citing by number in "Not on the roadmap" as a live
example of the drift, rather than as a hypothetical.

## 4. Things omatty already has that other trackers are still asking for

Useful for `comparison.md`, and only because each is a live request rather than
a feature comparison invented by us:

| Ask | Where | omatty |
|---|---|---|
| multiple git repositories in one view | claude-squad **#56** (7 comments, open — the most-discussed open issue there), **#299**, **#238** | M1 |
| type into a session straight from the list | claude-squad **#312** | M1 — the pane *is* the session |
| git status per worktree on the row | ccmanager **#11** | M8 diffstat, M9 gate strip |
| run commands in the session's own directory | fleet **#28** | M9 |
| select and copy text out of the agent's output | fleet **#85** | #217 and #212 — **both closed this month** |
| resume a session whose transcript already exists | ccmanager **#16** | #36, invariant 9 |

Two notes on honesty. The multi-repo row is the strongest external validation
omatty has, and it arrives in the same pass that proves `README.md` is wrong to
claim omatty is alone in it — ccmanager shipped it after its own #37. And the
copy/select row was true for about a week before capture; it is evidence that
omatty is current, not that it is ahead.

## 5. Traps other people have already paid for

Each of these cost another project a bug, and each maps to something omatty has
or is about to build:

- **`--resume` against a transcript-less session fails** (ccmanager #16) —
  omatty's #36, already invariant 9. Two projects, same bug, same cause. #152
  must not assume Codex's resume flag behaves the same way.
- **Prompt sent before the agent is ready, and lost** (claude-squad #266) —
  invariant 8 and `internal/paste` cover it, for a different original reason.
- **The multiplexer's reader racing the UI's reader for stdin**
  (claude-squad #325) — impossible while `internal/termwrap` owns the PTY.
- **Every added agent profile costs an escape-key bug** (ccmanager #82, #107).
  Budget for it in #152.
- **Forking a session is an identity problem, not a UI one** (fleet #142, #226).
  If it is ever proposed, invariant 9 asks the first question.

## 6. What this pass did not do

The Orca tracker was probed, not read: 3,036 open issues against queries chosen
from omatty's own design, which finds what we already thought to look for and
cannot support any claim about what its users complain about most. Nimbalyst
(573 open) and vibe-kanban (384 open) were not mined at all. Camps D and E were
not mined. If M12 is ever re-run, that is where it starts.
