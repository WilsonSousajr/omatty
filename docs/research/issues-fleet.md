# `brizzai/fleet` — issue and PR pain-point synthesis

> Source: GitHub `brizzai/fleet`, all 54 issues read 2026-09-18. Tracker
> character: young, fast-closing, informal — issue titles include "yoooooooo
> why the fork dont work" and "third time a charm" — with 13 open against 41
> closed and a release the day before capture. Low ceremony, high cadence.

## Top recurring pain points, ranked

### A. Wrong status, from the agent that has no hooks

- **#220 (2 comments)** "Wrong status: showed running, expected idle — codex
  shows running for no reason"

fleet reads Claude and Codex state from hooks (`internal/hooks/`), which is
omatty's design, and still gets a wrong-status bug — on Codex, the agent whose
hook set `codex_hooks.go` itself documents as incomplete: "Codex has no
SessionEnd/Notification events — `dead` comes from tmux pane-death."

**This is the most useful single issue in the pass for #152.** It is what a
partial hook set looks like in production: the busy signal arrives, the
matching idle signal does not, and the session sits reading "running" forever.
omatty's `DeriveKind` will face the same gap, and #152 should plan for a
transcript-side or timeout-side fallback for the events Codex does not send —
not because the hooks are unavailable (they are; see `fleet.md` §2) but
because they are incomplete.

### B. tmux and terminal-key collisions

- **#139 (2)** "Ctrl+K command palette shows an empty screen under some
  tmux/terminal key setups"
- **#41 (4)** "Infinite mallock broken ui"

The tmux tax again, milder than claude-squad's because fleet uses tmux for
multiplexing and hooks for status, so a tmux problem stays a tmux problem
instead of becoming a status lie. That separation is the thing worth noting:
fleet made the same architectural split omatty did, and its bug reports are
correspondingly better-behaved.

### C. Selecting and copying text out of an agent's output

- **#85** "Can't select text from Claude's responses. I want to copy it"

omatty had this exact bug and closed it twice: **#217** (2026-09-18) gave the
mouse back with `ctrl+o m` because `?1002h` was held for the whole life of a
session, and **#212** (2026-09-10) forwarded OSC 52 so the agent's own copy
reaches the host clipboard. A tool that captures the mouse for a session's
whole lifetime will get this issue filed against it; omatty is, on this one
narrow point, ahead of the field and only since this month.

### D. Session forking

- **#142 (3)** "forked session is opened with the wrong session id"
- **#226 (2)** "yoooooooo why the fork dont work"

fleet's `f` forks an agent's conversation and runs both branches. Two of its
noisier bugs are about it, both about identity. omatty has no fork and
invariant 9 is why: a session's identity is its uuid and everything is
recomputable from `state.json`, so forking would need an answer for what the
copy's uuid is and which transcript it claims. **If forking is ever proposed
for omatty, these two issues are the prior art, and the question to answer
first is identity, not UI.**

## Feature requests worth reading as roadmap signal

| Issue | Ask | omatty |
|---|---|---|
| #28 | "run CLI commands in session's workspace directory" | **has it** — `internal/gate` runs in the session's own directory (M9) |
| #274 | pass model and effort to `fleet add` | `internal/agent` command template |
| #310 | paste failing in a connected integration | invariant 8, `internal/paste` |
| — | PR state on the row (shipped, `internal/github/pr.go`) | **no answer**; see `issues-synthesis.md` §3 |

## Open issues the maintainers have not solved

Thirteen, mostly young. #292 — "when switching accounts the session went back
about 10 turns" — is the only one that looks structural, and it is about
account switching, which omatty does not do.

## Do-not-repeat lessons for omatty

1. **A partial hook set is worse than a documented absence**, because a busy
   event with no idle event leaves a session permanently wrong rather than
   permanently unknown. #152 needs a fallback, not just a hook path.
2. **Keeping status off the transport pays off even when you still use the
   transport.** fleet uses tmux and hooks; its tmux bugs stayed tmux bugs.
3. **Forking is an identity problem.** Two of fleet's louder bugs say so.
