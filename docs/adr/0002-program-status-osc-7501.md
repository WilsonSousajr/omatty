# ADR 0002: Claude Code's program status (OSC 7501) as a status source

- **Status:** Proposed 2026-11-18 (#816). Merging the pull request that
  carries this file accepts it, and with it the amendment to invariant 2
  under "Decision".
- **Date:** 2026-11-18
- **Evidence:** `docs/research/2026-landscape.md` §9.2 and §10.2 (#742,
  #810). The protocol details below were read in the Claude Code 2.1.327
  binary on 2026-11-18, because its documentation does not specify them.
- **Flags:** `invariant`. This ADR changes the wording of invariant 2.

## Context

omatty has two status sources for an agent, and invariant 2 names them:
"Status comes from JSONL and hooks, never from the rendered screen. No
package may parse the terminal cell grid to infer session state."

Claude Code 2.1.295 added a third kind of signal, the Program Status
Protocol: "terminals that implement it can show whether Claude Code is
working, waiting on you, or done" (`CHANGELOG.md`). 2.1.326 extended it:
"background commands, monitors and MCP tasks are listed, and a session
waiting on them shows as working instead of done".

That second line is a gap omatty has today. The `Stop` hook fires when a
turn ends. If the turn left a background command or a monitor running, the
card says idle while work is still going. The transcript does not say so
directly: a background command's tool result arrives at once, as its ID, not
when the command finishes.

### What the protocol is

Read in the 2.1.327 binary:

- **It is opt-in, by query.** At start-up Claude Code sends `OSC 7501 ; ?`
  and waits for a reply whose payload starts with `?`. With no reply
  ("probe: no reply to OSC 7501 ; ?") it records the terminal as not
  supporting the protocol and emits nothing. A terminal that only listens
  hears nothing.
- **The payload is structured.** Each report is
  `OSC 7501 ; state=<state>:app=claude-code[:id=…][:kind=…][:progress=N][:title=…][:msg=…]`,
  with `title` and `msg` base64-encoded and capped at 192 and 2,048 bytes.
  `kind` appears only on `blocked`. `state=clear` (optionally `:id=…`)
  withdraws a report.
- **It is a signal, not a drawing.** No cell of the grid changes when it is
  sent. bubbleterm's emulator handles OSC 0/1/2, 7, 8, 10-12 and 110-112, and
  drops everything else (`internal/tui/terminal/osc52.go`, the comment on
  `clipLift`). Today it drops these reports too.

### The precedent

omatty already lifts one OSC out of the PTY stream before the emulator sees
it. `clipLift` takes OSC 52 clipboard writes out of the child's output in
`internal/tui/terminal` (#212). It is bounded at 1 MiB, so a child that never
terminates a sequence cannot stall the read loop (#33). It also sits before
`c1Guard`, because `c1Guard` rewrites a byte OSC terminators use (#192).
`clipLift` deliberately does **not** answer the `?` read query. Answering
"needs a pending request per session to route the host's reply back into the
right PTY". Here the reply would come from omatty itself, not from the host
terminal, so that routing problem does not arise.

## Decision

**omatty may read OSC 7501 as a status source, and invariant 2 is amended to
say why that is not screen-reading.** The amended text, for `AGENTS.md`:

> 2. **Status comes from JSONL, hooks, and status signals the agent emits
>    on purpose, never from the rendered screen.** No package may parse the
>    terminal cell grid, or text the agent draws, to infer session state. A
>    status escape sequence the agent sends to its terminal as a protocol,
>    such as Claude Code's OSC 7501, is a structured channel like a hook, not
>    a rendering. It may be read, and only in `internal/tui/terminal`, which
>    owns the PTY.

Reading it is bound by these conditions. An implementation that cannot
meet one of them does not ship:

1. **Lifted, not parsed from the grid.** A reader beside `clipLift`, in
   `internal/tui/terminal`, takes the sequence out of the byte stream before
   the emulator. It is bounded like `clipLift`. A sequence that overruns the
   bound is dropped, never buffered.
2. **omatty answers the probe for the sessions it launched, and for no
   one else.** The reply goes only into the PTY of a session omatty started.
   Every other query still gets no answer, as now.
3. **It refines and never overrides.** For claude, which is Full tier,
   hooks and the transcript stay the truth. OSC 7501 may change one thing:
   a session the `Stop` hook marked idle shows as working while a report
   says background work is still running. A `blocked` report does not
   replace the `Notification` hook's "waiting for you"; it may only confirm
   it.
4. **Agent-agnostic by protocol, not by name.** The reader keys on the
   sequence, not on `app=claude-code`. An agent that ships the protocol and
   has no hooks would get a better status than Process tier offers. Its
   tier is still derived from `agent.Caps` (#520), never asserted because a
   report arrived.
5. **The status service stays the only place status is decided.** The
   terminal publishes lifted reports as typed events. `internal/service/status`
   applies them through the same `Apply` path hooks use. The TUI does not
   set a card's state itself (ADR 0001's layers).
6. **Detached sessions lose nothing they had.** Under dtach the probe is
   answered at launch, when omatty is attached. A session reattached by a
   later omatty keeps reporting, because claude already decided the terminal
   supports it. A session claude started while no omatty was attached gets
   no reply and falls back to hooks and transcript, exactly as today.

## Consequences

- Invariant 2's rule shifts from "these two sources" to "nothing drawn". The
  reason it exists, that a rendering changes with width, theme and
  scrollback while a fact does not, is unchanged. That reason is also why
  the amendment holds: an OSC 7501 report has no width, theme or
  scrollback.
- One new reader in `internal/tui/terminal`, which invariant 4 already
  fences. bubbleterm's blast radius does not grow, because the reader sits
  in front of it.
- omatty writes to an agent's input stream for the first time without a
  keystroke behind it: the probe reply. It is one fixed sequence, sent once
  per launch, and only in answer to the agent's own query.
- The card can be right about background work, which it is not today.
- A protocol omatty does not control can change. If Claude Code changes the
  payload, the reader drops what it cannot parse, and status falls back to
  hooks and transcript. That is the same degradation as a tier drop, not a
  wrong status.

## Not decided here

- Building it. That is a separate `feat` issue, filed once this ADR is
  accepted, with a golden test of a recorded OSC 7501 stream and a real-PTY
  smoke run against claude.
- Whether the card shows `progress`, `title` or `msg`. That is M18's
  question, not a status question.

## Alternatives considered

- **Leave invariant 2 as written and do not read it.** This is the
  zero-cost choice, and the honest default had the gap been cosmetic. It is
  not: an idle card over a running background command tells the person the
  session is free when it is not. That is the exact failure invariant 2
  exists to prevent.
- **Read it without answering the probe.** This does not work. Claude Code
  emits nothing to a terminal that did not reply.
- **Infer background work from the transcript.** A background command's
  result arrives at once, as its ID, so the transcript would have to track
  each ID until a later record mentions it finishing. That re-derives from a
  log what the agent states directly, and it is claude-specific in a way the
  protocol is not.
