# `stablyai/orca` — issue pain-point probe

> Source: GitHub `stablyai/orca`, 2026-09-18. **Not a full read.** The tracker
> holds 3,036 open and 2,783 closed issues, the largest in the field by an
> order of magnitude, and reading it exhaustively is its own project. This is
> a *targeted probe*: `gh search issues --repo stablyai/orca` against the terms
> omatty's own design touches — diff comments, review scope, CI state, worktree
> setup. Everything below is what those queries returned; the absence of a
> theme here is not evidence that the theme is absent from the tracker.

## A. The review surface is where Orca's users are pushing

- **#18120 (open)** "[Feature]: Sub-line comments — character range and/or
  multiple comments per diff line"
- **#11840 (open)** "[Feature]: Turn-scoped diff review — 'changes this turn' /
  'since my last review' scopes backed by a ref"
- **#19718 (open)** "[Feature]: Option to omit excerpts from markdown review
  notes"
- **#7991 (closed)** "Source control freezes when a branch has too many changed
  files"

**#11840 is the most interesting issue found in the entire pass.** "Changes
this turn" and "since my last review", backed by a git ref, is a demand for
exactly the thing omatty says it optimises — how fast a person can tell
whether what came back is any good — from the users of the largest tool in the
field. omatty's review pane shows everything a session changed; it has no
notion of *since when*. That is a roadmap candidate with external demand
already attached, and it fits the thesis rather than stretching it.

#18120 (multiple comments per line, sub-line ranges) is a natural extension of
omatty's anchor: `Anchor{File, Hunk, Hash, Nth}` already disambiguates repeated
lines, and a character range would be a fifth field rather than a redesign.
Worth recording that the anchor generalises where a line number does not.

## B. Verdict state on the card — demanded, and not as a local gate

- **#18484 (open)** "[Bug]: Sidebar card MR icon stays green (open) when GitLab
  CI failed — unknown CI status is indistinguishable"
- **#18485 (open)** "[Feature]: Show unresolved PR/MR comment count on the
  workspace card"
- **#18487 (open)** "[Feature]: Show merge-conflict / behind-target state on
  the workspace card"
- **#10131 (open)** "[Feature]: CI-based automations — auto-run agents when PR
  checks fail"

This qualifies `2026-landscape.md` §3.2 in a way that matters. The empty square
is real — nobody runs the project's check line per session — but the demand
Orca's users express is for the **remote** verdict pulled onto the card: CI
state, PR comments, behind-target. Not a local gate.

Two honest readings, and #300 has to pick:

- *For omatty:* the need behind all four is "tell me on the card whether this
  work is sound", and M9 answers it faster than CI can, because it needs no
  push and no runner. These users are asking for the slow version because the
  fast one is not on offer.
- *Against omatty:* the authoritative verdict genuinely does live in CI, and a
  green local gate is not the same claim. **PR and CI state on the card** is
  the shape the field is actually asking for — fleet already ships it
  (`internal/github/pr.go`) — and omatty has no answer to it at all.

Both readings can be true, and #10131 is where they part: "auto-run agents when
PR checks fail" is the orchestrator move, refused by invariant 12. omatty's
`S` sends failures into the session **when a person presses it**, and #233's
auto-run gates a session when its turn ends without sending anything. The line
between those and #10131 is exactly the line "Not on the roadmap" draws, and it
is worth citing #10131 there as a live example of the drift.

## C. Their own gate hurts, which is a different lesson

- **#21395 (open)** "docs/audits is gitignored: 225 tracked evidence files are
  invisible to lint, format, typecheck and CI"
- **#14479 (open)** "pnpm lint cannot pass on a stock Windows checkout (CRLF)"
- **#20185 (open)** "react-doctor violations in native-chat/annotate redden
  static analysis on every PR"

Not a product finding — a development finding, and the mirror of M11. A project
shipping daily at this size has a check line that is red on every PR and files
that fall outside it. omatty made its rules into gate steps that fail (M11) and
enforces coverage and C.R.A.P. on the diff (M10); these three issues are what
the alternative looks like at scale.

## What a full read would still owe

This probe cannot support any claim about what Orca's users complain about
*most*, only about what they say on the subjects queried. A full pass — the
tracker's character, the ranked clusters, the design choices behind them — is
its own slice, and is worth opening if M12 ever revisits this. Recorded here so
the omission is on the record rather than mistaken for absence.
