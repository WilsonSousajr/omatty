# `kbwo/ccmanager` — issue and PR pain-point synthesis

> Source: GitHub `kbwo/ccmanager`, all 58 issues read 2026-09-18. Tracker
> character: small, well-tended, and almost entirely closed — 4 open against
> 54 closed. Feature requests are labelled as such by convention
> (`[Feature Request]` in the title) and most of them shipped. This is what a
> healthy tracker looks like, and it makes the recurring items easy to read.

## Top recurring pain points, ranked

### A. State detection getting it wrong

- **#227 (4 comments)** "State detection shows idle when Claude Code is busy"
- **#195 (4)** "Tilt since version 3.6, i'm using 3.5"
- **#2 (4)** "Am I the only one with this issue?"

**The design choice behind it:** `src/utils/promptDetector.ts` decides state by
matching Claude Code's drawn prompt box with regular expressions — `/│\s*>\s*/`,
`╭───╮`, `╰───╯`. A box that is on screen but not waiting, an agent that draws
differently, or a version that changes the frame all produce a wrong state.

**Read this next to claude-squad's §A.** The two most popular terminal session
managers in the field pick different screen-scraping mechanisms and get the
same bug class. omatty's invariant 3 — status from hooks and the transcript,
"never scraped from the screen" — was argued from first principles in M2 with
no examples to point at. It now has two.

### B. Multi-project support — asked for, then built

- **#37 (5 comments, CLOSED)** "[Feature Request] Multi-project support for
  worktree management"

Filed by a user, shipped as Multi-Project Mode. The same thing claude-squad's
#56 is still asking for, and the reason `README.md`'s "every other tool is
either a desktop app or scoped to a single repository" no longer holds.

The useful part for omatty is not that ccmanager won a race. It is that the
need was strong enough to be filed against two separate projects and built by
one of them, which is as close to product-market evidence as a tracker gets.

### C. Carrying gitignored files into a new worktree

- **#7 (3, CLOSED)** "[Feature Request] A configure file with ability to copy
  (typically git ignored) files to worktree on init"

Shipped as `.worktreeinclude`, with the copy ordered *before* the
post-creation hook so hooks can rely on the files. Third of four independent
appearances of this need across the field; see `issues-synthesis.md` §1.

### D. Session persistence across a restart

- **#19 (3, CLOSED)** "How can I keep the ccmanager session like in TMUX"
- **#26 (3)** "ccmanager crashed during normal use and now fails to start
  completely"
- **#16 (3)** "When the `--resume` argument is set, the session will be
  terminated if there is no conversation history"

#16 is omatty's #36 almost exactly: `--resume` against a session with no
transcript fails, so the launcher must choose between `--session-id` and
`--resume` on whether a transcript exists. Two projects, the same bug, the
same cause — the transcript is the claim. omatty fixed it in M1 and wrote it
into invariant 9; ccmanager fixed it too. Nobody gets this right the first
time.

ccmanager's answer to #19 is `sessionRestorer.ts`: reopen the *records* after
a crash and start fresh processes. omatty's is dtach: keep the processes.
Stronger guarantee, hard dependency.

### E. Agent-specific escapes

- **#82 (3)** "Unable to return to the main menu from Codex"
- **#107 (3)** "cannot exit from codex"

The cost of the multi-agent seam: each agent CLI grabs the terminal
differently and the manager's own escape key has to survive it. omatty has one
and a half agents and a single `ctrl+o` leader (invariant 2); #152 should read
these two issues before adding the second profile, because they are what the
seam actually costs at eight.

## Feature requests worth reading as roadmap signal

| Issue | Ask | omatty |
|---|---|---|
| #37 | multiple repositories | **has it** |
| #7 | copy gitignored files into a new worktree | **no answer** |
| #11 | "Show git status on the right side of each worktree" | **has it** — M8's diffstat, M9's gate strip |
| #57 | "New hook expected" | M2 wires `Notification`, `Stop`, `PreToolUse` |
| #196 | conversation resume and search broken under the manager | invariant 9; omatty's #36 |

## Open issues the maintainers have not solved

Four, none architectural. There is no long-lived unsolved seam here of the
kind cognee-style trackers show. The project's hard problem is the one it does
not treat as a problem: state detection is a regex over someone else's UI, and
the bugs it produces are closed individually.

## Do-not-repeat lessons for omatty

1. **`--resume` versus `--session-id` is a trap two projects fell into.**
   Invariant 9 already encodes the answer; #152 must apply the same reasoning
   to Codex rather than assume its resume flag behaves the same way.
2. **Every added agent profile costs an escape-key bug.** Budget for it in
   #152 instead of discovering it.
3. **A small tracker is a design signal.** ccmanager has 4 open issues at 1,246
   stars because it does one thing. omatty's surface is already wider, and M12
   should not widen it without saying what it buys.
