# Migration plan: today's layout to ADR 0001

- **Status:** Approved 2026-09-29 (#623); executing (Phase 4).
- **Target:** `docs/adr/0001-architecture.md` (accepted, #618).
- **Evidence:** `docs/ARCHITECTURE_AUDIT.md` (#615).
- **Net:** the Phase 2 characterization tests (#620):
  - the `state.json` golden
  - screen snapshots of 13 scenes at 2 sizes
  - the key table: 13 contexts × 186 keys
  - the message table
  - the layer report, currently 28 findings
- **Order with M17:** decided 2026-09-29, the migration goes first.
  - M17's code issues (#520–#526 and the agent profiles, including #152) wait
    until Stage 5 has landed.
  - M17's spikes and docs (#519/#547, #527–#546) touch no Go code and may
    proceed in parallel.

The audit's pain points are cited as **P1**–**P5**, and the ADR's sections by
name.

## Progress

Each step's PR adds its own line here. "Findings" is `./scripts/check-layers.sh`'s
count after the step.

| Step | Issue | PR | State | Findings |
|---|---|---|---|---|
| 0 | #622 | #623 | merged | 28 |
| 1.1 | #624 | #625 | merged | 28 |
| 1.2 | #624 | #626 | merged | 28 |
| 1.3 | #624 | #627 | open | 28 |

## Rules every step follows

1. **One step, one PR, into `develop`.** Each compiles, passes the full gate,
   and can be merged on its own. None is merged without the maintainer's
   approval.
2. **Moves are mechanical and alone.**
   - A package move is `git mv` plus the import rewrite it forces, and nothing
     else. The diff is renames plus import lines. In Go a move cannot be edit-free:
     every importer's import line changes.
   - Each move PR also updates, in the same PR:
     - the paths in `.golangci.yml`
     - the `go list` assertions in `scripts/depguard_test.go` and
       `scripts/network_test.go`
     - the `tools/layercheck` transitional table: the entry is deleted,
       because the new path places the package itself
     - AGENTS.md's "Repository layout" and `docs/ARCHITECTURE.md`'s package
       table
   - A move PR may exceed the 400-line target; every other PR stays under it.
3. **Extractions leave aliases behind.** When a type moves out of a mixed
   package, the old package keeps `type X = newpkg.X` so importers keep
   compiling. The importers then move over in small PRs. Stage 8 deletes the
   aliases, so none outlives the migration.
4. **Moves never change behaviour, and the net says so.**
   - A move or extraction PR changes **no golden**.
   - A logic PR may change a golden only when its description names the row
     and says why.
   - A golden changing for any other reason is a regression. Fix the code,
     not the golden.
5. **The layer count only goes down.** Each PR body quotes `layer findings:
   N` before and after. A PR that raises it has broken ADR 0001's direction
   and is reworked, not merged.
6. **Real PTY smoke test.** Steps marked **smoke** also run AGENTS.md's
   real-PTY smoke test (`testdata/ptyrun` against a scratch HOME with
   `fake-claude`), and a person reads it.
7. **Rollback** is `git revert` of the merge commit, unless the table says
   otherwise. "Blocks" says which later steps a revert would also force out.

## The sequence

**Risk:** **L**ow means mechanical, with the net covering it. **M**edium moves
logic the net covers. **H**igh changes a runtime path the net covers only
partly, and needs smoke.

### Stage 0: this plan

| # | PR | Scope | Files | Risk | Serves | Needs | Rollback |
|---|---|---|---|---|---|---|---|
| 0 | `docs(#622): migration plan` | this document; ADR Enforcement wording (layercheck, not depguard) | `docs/MIGRATION_PLAN.md`, `docs/adr/0001-architecture.md` | L | Phase 3 | — | revert |

### Stage 1: move the adapters to `internal/infra/`

Every step here is a pure `git mv` plus the import rewrite. These packages
are adapters already, so they move as they are.

| # | PR | Scope | Importers | Risk | Serves | Needs | Rollback |
|---|---|---|---|---|---|---|---|
| 1.1 | `refactor: move paths to infra/paths` | `internal/paths` | 9 | L | ADR tree, Exceptions (paths is infra) | 0 | revert; blocks nothing |
| 1.2 | `refactor: move vcs to infra/vcs` | `internal/vcs`; `TestNoGitOutsideVcs` retargeted | 5 | L | invariant 4 | 0 | revert |
| 1.3 | `refactor: move detach to infra/detach` | `internal/detach` | 4 | L | ADR tree | 0 | revert |
| 1.4 | `refactor: move notify and highlight to infra` | two leaf adapters; chroma fence path | 2 + 2 | L | ADR tree | 0 | revert |
| 1.5 | `refactor: move golist and config to infra` | `golist` (tools import it), `config` (the only TOML importer) | 6 + 2 | L | ADR tree | 0 | revert |
| 1.6 | `refactor: move forge to infra/forge` | 40 production files; `TestNoGhOutsideForge` and the `net/http` allowlist retargeted | 5 | L (large but mechanical) | invariant 4, the network fence | 0 | revert |
| 1.7 | `refactor: move hooks to infra/hooks` | the hooks.json writer and the `omatty hook` client, byte-identical | 5 | L | invariants 3 and 11 | 0 | revert |

### Stage 2: move the already-pure packages to `internal/domain/`

| # | PR | Scope | Risk | Serves | Needs | Rollback |
|---|---|---|---|---|---|---|
| 2.1 | `refactor: move fuzzy and paste to domain` | two stdlib-only packages | L | ADR tree | 0 | revert |
| 2.2 | `refactor: move crap and depgraph to domain` | `tools/crapcheck` and `depcheck` import them | L | ADR Exceptions (they stay in the coverage gate) | 1.5 | revert |

### Stage 3: extract the domain out of the mixed packages

Each step creates `internal/domain/<x>`, moves the pure files with `git mv`,
and leaves aliases in the old package (rule 3). The mixed packages lose their
domain half one at a time.

| # | PR | Scope | Risk | Serves | Needs | Rollback |
|---|---|---|---|---|---|---|
| 3.1 | `refactor: domain/session from registry` | `registry/state.go` types and `ConversationID`, `naming.go`, the per-project gate config type; aliases in `registry`. The `state.json` golden must not move. | M | invariant 9, ADR ports (session types) | 3.3 | revert |
| 3.2 | `refactor: domain/status from watcher and hooks` | `watcher/event.go`, `status.go`; `hooks.Payload` → `status.HookPayload`; aliases | M | ADR event model | 1.7 | revert |
| 3.3 | `refactor: domain/gate from gate` | `Step`, `StepResult`, `Report`, `Verdict`, `compose.go`, the percentage parse (`coverage.go`); aliases. The runner stays. | M | invariant 12 | 0 | revert; blocks 3.1 |
| 3.4 | `refactor: domain/coverage; the profile read to infra/fsread` | the parsers to `domain/coverage`, `load.go` to a new `infra/fsread` | M | ADR tree | 0 | revert |
| 3.5 | `refactor: domain/forge types out of infra/forge` | PR, Issue, Detail, Comment, CI, Kind, Label; aliases in `infra/forge` | M | ADR tree | 1.6 | revert |
| 3.6a | `refactor: domain/review from review (model)` | anchor, change, comments, compose, digest, entries, pair, place, tree, treecompact, turnplace, shippable; aliases | M | invariants 7 and 8 | 3.1 | revert |
| 3.6b | `refactor: go-gitdiff parsing to infra/gitdiff` | `review/parse.go`; the depguard `wrappers` rule retargeted | M | invariant 4 in spirit | 3.6a | revert |
| 3.6c | `refactor: review's file reads to infra/fsread` | the `preview.go` read, the `generated.go` .gitattributes read | M | P5 | 3.4, 3.6a | revert |
| 3.7 | `refactor: domain/agent, cutting its edges to paths, hooks and watcher` | paths arrive as values from `cmd`; the Adapter types from `domain/status` | M | ADR Exceptions (paths); P2 | 3.2 | revert |
| 3.8 | `refactor: move tally to domain` | after its inputs are domain types | L | ADR tree | 3.1, 3.5 | revert |

### Stage 4: every git call gets a context and a deadline (a behaviour change)

| # | PR | Scope | Risk | Serves | Needs | Rollback |
|---|---|---|---|---|---|---|
| 4.1 | `feat: vcs takes a context; git has a deadline` | every `infra/vcs` method takes `ctx`; `exec.CommandContext`. The named deadlines are git 30 s and `worktree add` 60 s. Callers pass a timeout context. **User-visible only when git hangs:** omatty now reports an error instead of freezing. | M, **smoke** | P5, audit leak 6 | 1.2 | revert; blocks 5.4 |

### Stage 5: pubsub and the services

These steps move logic, not just files. The key and message tables are the
net: a logic PR that changes a row says which row and why (rule 4).

| # | PR | Scope | Risk | Serves | Needs | Rollback |
|---|---|---|---|---|---|---|
| 5.1 | `feat: internal/pubsub` | `Broker[T]` with `Publish` (waits) and `Offer` (drops and counts); `Event[T]`. It has no users yet. | L | ADR event model | 0 | revert |
| 5.2 | `refactor: service/status, infra/transcript, infra/hookserver` | `watcher/watch.go`, `adapter.go`, `guard.go` and `transcript.go` become the service. The tailer goes to `infra/transcript` and publishes with `Publish`. The listener goes to `infra/hookserver` and publishes with `Offer` (invariant 11). The TUI subscribes where it read `Deps.Events`. | H, **smoke** | ADR event model, invariants 2 and 11 | 3.2, 5.1 | revert; blocks 5.8, 7.1 |
| 5.3 | `refactor: service/gate and infra/gateexec` | the runner and bound become the service, which publishes `Event[gate.Report]`. `run.go`, `procgroup_*` and `detect.go` go to `infra/gateexec`. The exec allowlist changes as the ADR records. | M, **smoke** (`gateprobe`) | invariant 12, ADR Enforcement | 3.3, 5.1 | revert |
| 5.4 | `refactor: service/sessions (state and commands)` | registry's commands and create become the service. `store.go` and carry's copy go to `infra/store` behind `StateStore`. The ports carry `ctx`. | M | ADR ports, P2 | 3.1, 4.1 | revert; blocks 5.5–5.7 |
| 5.5 | `refactor: starting a session returns session.Launch` | `supervisor/launch.go` becomes `sessions.Start`, which returns argv/env/dir. The `Holder` port works over argv. `tui/terminal` (still `termwrap`) spawns. `namer.go` goes to `infra/agentcli`. | H, **smoke** + `dtachprobe` | ADR "Starting a session", invariant 9 | 5.4 | revert |
| 5.6a | `refactor: create and start leave Update` | `sessionlife.go:90/44/107` become Cmds; `Event[session.Session]` feeds the sidebar | H, **smoke** | P5, audit leak 5 | 5.5 | revert |
| 5.6b | `refactor: archive, rename, rebind and fold leave Update` | `archive.go:201`, `rename.go:62`, the rebind and fold writes, `gaterun.go:129` tally | M, **smoke** | P5 | 5.6a | revert |
| 5.6c | `refactor: discovery and adoption leave Update` | `discovery.go:157` (git rev-parse per root), `adopt.go` | M | P5 | 5.6a, 5.9 | revert |
| 5.7 | `refactor: the idle sweep and repo stat move to service/sessions` | the sweep and stat tickers publish events; the TUI's `repostat.go` and `sweep.go` ticks are removed | M | ADR event model (timers) | 5.6a | revert |
| 5.8 | `refactor: service/review` | `review/source.go`, turn loading, ship, revert, behind `DiffSource`, `DiffParser`, `FileReader` and `Shipper` | M | ADR ports | 3.6b, 3.6c, 5.4 | revert |
| 5.9 | `refactor: service/tracker and service/discovery` | PR and issue polling (`Event[tracker.Snapshot]`), item, browse; `discover` | M | ADR event model | 3.5, 5.1 | revert |
| 5.10 | `refactor: cmd is the only composition root` | `ui/run.go`'s wiring goes to `cmd`. `RunDeps` and `modelFor` go. `Deps` shrinks to the services and presentation-only settings. Dependencies are plumbed once. | M, **smoke** | P2, audit leak 2, invariant 10 | 5.2–5.9 | revert |

**After 5.10:** M17's code issues may start (the decision above), on
`domain/agent` profiles plus the `service/status` Adapter port.

### Stage 6: the TUI

| # | PR | Scope | Risk | Serves | Needs | Rollback |
|---|---|---|---|---|---|---|
| 6.1 | `refactor: ui to tui/app, termwrap to tui/terminal, keys to tui/keys` | three `git mv`s; the bubbletea and bubbleterm fence paths | L (220 files, mechanical) | ADR TUI | 5.10 | revert |
| 6.2 | `refactor: tui/theme` | the palette and every `lipgloss.NewStyle`, including the strays. The SGR goldens must not move. | L | ADR TUI (theme) | 6.1 | revert |
| 6.3 | `refactor: the Screen interface and the diff screen` | `tui/screen`; the root routes the review column through it; `screens/diff`. **Adds `charm.land/bubbles/v2`** for `key.Binding` (ADR-approved); the help modal renders from `Bindings()`. | H, **smoke** | P1, P3, P4 | 6.2 | revert; blocks 6.4–6.9 |
| 6.4 | `refactor: screens/tree and screens/preview` | two faces that share the file list | M | P1, P4 | 6.3 | revert |
| 6.5 | `refactor: screens/gate` | the gate face | M | P1, P4 | 6.3 | revert |
| 6.6 | `refactor: screens/tracker and screens/trackeritem` | tracker and item | M | P1, P4 | 6.3 | revert |
| 6.7 | `refactor: screens/modals, part 1` | the new-session prompt, rename, confirm | M | P4 | 6.3 | revert |
| 6.8 | `refactor: screens/modals, part 2` | list, picker, adopt, help, switcher | M | P4 | 6.7 | revert |
| 6.9 | `refactor: components` | sidebar, card, picklist, editline, listwindow, spinner, meter move out of `app`; `*Model` keeps only view state | M | P1 | 6.4–6.8 | revert |

### Stage 7: the second driving adapter

| # | PR | Scope | Risk | Serves | Needs | Rollback |
|---|---|---|---|---|---|---|
| 7.1 | `feat: omatty sessions --json and status --json` | `internal/cli`; the transcript `Read` port on `service/status`. **Adds two user-visible commands (approved in ADR review).** | M | ADR "The second driving adapter" | 5.10 | revert |
| 7.2 | `refactor: adopt, rm and stats run through internal/cli` | today's subcommands move out of `cmd/omatty/subcommands.go`; their CLI output is unchanged | M | invariant 10 | 7.1 | revert |

### Stage 8: the rule turns on

| # | PR | Scope | Risk | Serves | Needs | Rollback |
|---|---|---|---|---|---|---|
| 8.1 | `refactor: delete the migration aliases` | every alias rule 3 left; importers use the domain types | L | ADR Consequences | 7.2 | revert |
| 8.2 | `ci: enforce ADR 0001's layers` | `-enforce` in CI and AGENTS.md. `TestLayerCheck_enforceFailsOnFindings` is rewritten against a fixture with findings, since the repository has none now; the commit says why (AGENTS.md: a regression test is never quietly weakened). The transitional table is deleted. | L | ADR Enforcement | 8.1, and 0 findings | revert |
| 8.3 | `docs: ARCHITECTURE.md for the new shape` | the data-flow and package sections rewritten; the audit and plan marked done | L | — | 8.2 | revert |

## Also carried, from #620's review

These deferred items land where they fit, not as steps of their own:
- **`RUNEWIDTH_EASTASIAN=1` breaks every screen golden.** Pin it to `0` in
  the golden tests, in whichever of 6.1–6.3 first touches `golden_test.go`.
- **The determinism test writes goldens under a bare `-update`.** Make it
  compare-only, at the same time.
- **The transitional table lacks its per-entry target comments.** Moot
  after 8.2 deletes it; until then each Stage 1–3 PR removes its own entry.
- **#616** (`go test -cover` misreads `internal/ui`) can be fixed at any
  point; it is independent.

## Size

46 PRs after this one: 7 in Stage 1, 2 in Stage 2, 10 in Stage 3, 1 in Stage
4, 12 in Stage 5, 9 in Stage 6, 2 in Stage 7, 3 in Stage 8.

- **Parallel work:** Stages 1 and 2, and most of Stage 3, can proceed in
  parallel branches, because their packages don't overlap.
- **Serial work:** Stage 5 onward is mostly serial, because each step feeds
  the next.

## When the plan is wrong

If a step shows the plan is wrong, the step stops and an amendment to this
file is proposed. Nothing is improvised. Typical causes:
- a move that cannot be mechanical
- a golden that has to change in a move
- a layer count that goes up

The amendment is its own small PR, reviewed before the step resumes.
