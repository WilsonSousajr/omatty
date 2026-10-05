# M17 - The Agents — design (#519)

Approved 2026-09-27 in a brainstorming session. Twenty-eight implementation
issues, #520-#546 and the re-scoped #152, one PR each. This document and the
ROADMAP section are #519. Every issue is in Backlog: M17 is designed and
captured, not scheduled.

## Context

omatty runs one agent, and that agent is claude. The seam for more has existed
since #46: `internal/agent.Profile` is a command template, a transcript path,
hook events, a settings renderer and a `watcher.Adapter`. But it has one entry,
and the code around it assumes one:

- `cmd/omatty/wiring.go` calls `agent.Lookup("")`.
- `supervisor.Launcher` holds a single profile. Its doc comment says "When a
  second arrives, Start resolves sess.Agent".
- `watcher.WatchDeps` carries one `Adapter` for every session.
- `ui/run.go` falls back to `agent.Claude()`.
- `paths.HooksFile(home)` is one file.
- `registry.Session.Agent` exists and is always empty. `Session.Conversation`
  (#316) already separates omatty's key from the id the agent is on now.

#152 (Codex) has been open since M7 and is half-spiked. It blocked on the
question omatty's whole architecture rests on: whether the agent accepts a
session id that omatty assigns.

**This reverses a written decision.** M12's "Deliberately cut" says: *Widening
the agent seam to match `ccmanager`'s eight. #152 stays the scope.
`ccmanager`'s #82 and #107 are what each added profile costs: an escape-key
bug per agent.* It is reversed for two reasons:

1. **The cost it names came from key handling, and invariant 1 already rules
   that out.** ccmanager's per-agent escape bugs come from inspecting keys
   per agent. omatty forwards every keystroke but the leader, for every agent,
   so there is no per-agent key table to get wrong. Each spike still checks
   Esc and ctrl chords (question 5 below), because a promise is not a probe.
2. **Tiers cap what each profile costs.** A profile claims only what its
   agent exposes. An agent that exposes nothing still runs, at the Process
   tier, and its profile has almost no code. The expensive part, a status
   adapter, is paid only where the agent makes it possible.

Decisions taken with the user:

| Decision | Choice |
|---|---|
| Support bar | **Tiered.** Every agent gets the agent-agnostic surface. Live status and resume come only where the agent exposes them, and a published matrix states each agent's tier from real probes. |
| First-class profiles | **Codex, OpenCode, Gemini CLI, Qwen Code, Antigravity (`agy`), GitHub Copilot CLI, cursor-agent, Amp, Kiro CLI.** |
| Everything else | **A generic agent declared in config** (Aider, Goose, Crush and Droid are the recipes the close-out verifies). |
| Footprint | **Zero, for every agent.** Invariant 3 generalises. An agent that can take hooks only from a global or in-repo file gets no hooks and drops a tier. |
| Choosing | **A project default plus a per-session override** on `ctrl+o n`. |
| Commitment | **A milestone, M17, every issue in Backlog.** |

## Approaches considered

**A. Capabilities on the Profile, tier derived from them. Chosen.** It is
#46's seam grown in the direction #46 pointed: one package, one file per
agent, which is also how M16 grew `internal/forge`.

**B. Declarative adapters in config.** This means transcript shapes described
as JSON-path rules in TOML, with no Go per agent. **Refused.** It is untyped
input crossing a package boundary, which AGENTS.md bans, and the nine
transcript formats differ in structure, not just in field names: pairing,
envelopes, and ids in paths. #525's generic agent takes `command` only, and
that is the line that stops B creeping back in.

**C. Speak ACP (Agent Client Protocol).** Several of these agents speak it,
and it would give structured status for free. **Refused**, and listed in the
ROADMAP's "Not on the roadmap". ACP means omatty renders the conversation
itself instead of running the agent's own TUI in a PTY. That breaks the first
line of the design: *omatty never reimplements Claude's interactive surface.*
The same holds for every agent, not only claude.

## What every agent gets

The surface below reads only the PTY, git, the gate and the forge, never the
agent. It works the same at every tier:

- the pane (the real binary in a PTY, rendered through termwrap);
- the diff, content-anchored comments, and review submission by bracketed
  paste (invariant 8, confirmed per agent by spike question 5);
- the gate and its verdicts (M9), coverage on the diff (M10);
- worktrees, `carry`, the tracker, PR and CI state on the card, ship (#331);
- detach and reattach (M6).

## Tiers

A profile declares `Caps` (#520). The tier is **computed** from them and never
written by hand, so the support matrix (#546) can be generated from the
profiles rather than maintained beside them.

| `Caps` field | Values |
|---|---|
| `Identity` | `Assigned` (omatty hands the id, like `--session-id`), `Reported` (a startup hook reports the agent's own id), `Scanned` (found in the agent's store by cwd and start time), `None` |
| `Status` | `Hooks` (hooks plus transcript), `Transcript`, `Process` |
| `Waiting` | can report "waiting for you" (`PermissionRequested`) |
| `Resume` | a resume flag exists |
| `TurnBoundary` | reports a turn's end, which #233's auto-run, #311's since-turn review and notifications all key off |

| Tier | Requires | Gets |
|---|---|---|
| **Full** | identity known, `Hooks`, `Waiting`, `Resume` | Everything claude has today. |
| **Transcript** | identity known, `Status` at least `Transcript` | Busy and idle from the transcript, resume if `Resume`, and whatever `TurnBoundary` and `Waiting` allow. |
| **Process** | anything else | `running` or `exited`, from the process alone. |

claude derives Full, and a test pins it.

What a lower tier does not get is **shown as absent, with a reason** (#526).
A Process-tier card has no idle glyph, because an agent that cannot report busy
must never look idle. It has no token meter either. A session with no
`TurnBoundary` refuses auto-run and "since this turn" and says why. A session
without `Resume` offers "start fresh" after a crash and says the conversation
is lost.

## Identity: Reported and Scanned (#523)

omatty's `ID` stays the key and the agent's own id goes in `Conversation`, as
#316 already does for `/clear`.

- **Reported.** The supervisor sets `OMATTY_SESSION` for every agent, as it
  does for claude. The agent's startup hook reports its own id, and
  `registry.Rebind` records it. This is #316's path with a second caller.
- **Scanned.** The profile's locator lists transcripts in the agent's store
  for the session's cwd, created at or after its start. The watcher binds
  only on **exactly one** candidate. With two sessions in one directory
  started together it refuses to guess: the session stays unbound at the
  Process tier, and a log line names the candidates. A wrong binding would
  show one session's status on another's card, which is worse than showing
  none.
- The transcript path becomes a locator that can say "not yet". Codex needs
  this even with an assigned id, because its path contains a date directory
  and a timestamp that cannot be predicted (#152's half-spike).

## Invariants

- **Invariant 1** is untouched and does the work M12's cut feared: there is
  no per-agent key handling to get wrong.
- **Invariant 2** is untouched. Status comes from hooks, transcripts or the
  process, never the screen. The Process tier exists precisely so that an
  agent with no structured output is not scraped.
- **Invariant 3 generalises (#522, labelled `invariant`):** *omatty never
  writes any agent's user configuration, and never writes an agent config file
  inside the project.* The second half is new. A `.gemini/settings.json` or
  `.cursor/hooks.json` written into the repository dirties the operator's tree
  and gets committed. Allowed routes are per-invocation only: a flag, or an
  env var naming an extra file the agent *merges*. A route that replaces the
  user's config is refused too, because it hides their auth.
- **Invariant 4** holds. The exec allowlist does not grow. The installed check
  for the picker is `supervisor.Installed`, injected into the ui, because
  `exec.LookPath` lives in `os/exec` (#524).
- **Invariant 6:** a session naming an agent the catalog no longer has (a
  removed generic block) shows `✕` with the name and starts nothing.
- **Invariant 9 is amended (#520, labelled `invariant`):** "state.json
  suffices to relaunch every session with `--resume`" cannot hold literally
  for an agent without resume. It becomes: *state.json suffices to do
  everything the agent allows*, meaning resume where `Resume` is set and a
  fresh start in `Dir` otherwise, with the loss stated. `Session.Agent`,
  `Project.Agent` and `Conversation` are all `omitempty` with derivable empty
  values, so `Version` stays 1.
- **Invariant 11** holds for every agent. `omatty hook --agent <name>` is
  bounded, silent and exits 0, including for an unknown agent name.
- **claude's hooks file does not move.** Live detached sessions (M6) carry
  `~/.omatty/hooks.json` in their argv. Other agents get
  `~/.omatty/hooks-<agent>.json` when their route needs a file.

## The slices

**Foundation**, in order:

| # | Slice |
|---|---|
| #520 | `Caps` on the Profile and a derived tier; invariant 9 amended |
| #521 | an injected `agent.Catalog`; the launcher, watcher and ui dispatch per session |
| #522 | hook settings per agent, `omatty hook --agent`; invariant 3 generalised |
| #523 | learned identity: Reported and Scanned into `Conversation` |
| #524 | `default_agent`, `Project.Agent`, `[agents.<name>] bin`, and the `ctrl+o n` picker |
| #525 | a generic agent declared in config, at the Process tier |
| #526 | a tier-aware surface |
| #527 | `testdata/fake-agent`, a scripted stand-in per profile shape |

**Then a spike and a profile per agent.** Every spike answers the same six
questions against the real binary, recording its version, in
`docs/research/agents/<agent>.md`:

1. **Identity:** assigned, reported, scanned, or none.
2. **Hooks without a footprint:** a per-invocation route, and which events
   exist.
3. **Transcript:** where it is, its shape, busy/idle pairing, a waiting
   signal, and usage.
4. **Resume:** the flag, the id, and what a missing id does.
5. **Input:** a bracketed-paste prompt plus one `\r` submits once, and Esc
   and ctrl chords behave with every key forwarded.
6. **Tier:** the one the answers imply.

Where only a missing upstream flag keeps an agent below Full, the spike files
a feature request upstream rather than working around it with a write.

| Agent | Spike | Profile |
|---|---|---|
| Codex | #528 | #152 |
| OpenCode | #529 | #530 |
| Gemini CLI | #531 | #532 |
| Qwen Code | #533 | #534 (tries Gemini's adapter first) |
| Antigravity (`agy`) | #535 | #536 |
| GitHub Copilot CLI | #537 | #538 |
| cursor-agent | #539 | #540 |
| Amp | #541 | #542 |
| Kiro CLI | #543 | #544 |

**Then the close-out:**

- **#545** `agentprobe`: each real binary inside omatty in a sized PTY, read by
  a person. It needs omatty's state in a scratch directory while the agent
  keeps its real home and auth. That is an open design point, settled in that
  PR.
- **#546** the support matrix in `docs/agents.md`, README and
  `docs/comparison.md`, with generic recipes for Aider, Goose, Crush and
  Droid. Its cells come from the probes, not from vendor docs. **An agent not
  probed is listed as untested, not claimed.**

## Testing

- TDD as always. Every profile has argv goldens for fresh and resume, and an
  adapter test over sanitised fixtures recorded from the real binary. The
  test asserts both the `Kind`s the tier claims and that the ones it cannot
  produce stay absent.
- Tests never run a real agent. `testdata/fake-agent` (#527) stands in, as
  fake-claude does.
- Fixtures are sanitised before commit. Transcripts carry user code and
  prompts (AGENTS.md, Security).
- The gate is necessary, not sufficient. #545 is M17's real-binary smoke test,
  and ROADMAP rule 2 applies to it.

## Done when

Each of the nine agents runs in omatty at the tier its probe showed. A generic
agent runs from a config block. The matrix states every row from a probe run a
person read. Nothing in the README claims an agent the matrix lists as
untested.

## Deliberately out

- **ACP**, and any mode where omatty renders an agent's conversation itself
  (approach C).
- **Declarative transcript adapters** in config (approach B).
- **Adopting other agents' existing sessions.** Discovery (#91) reads claude's
  store only. A per-agent version would be its own issue, as #152 said.
- **Writing any agent's config**, globally or in the repo, including an opt-in
  install command.
- **Model and provider pickers**, per-agent keybindings, and running one
  prompt through several agents to compare them (already cut:
  "Running N sessions on one task and comparing the results").
