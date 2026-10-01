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
| 1.3 | #624 | #627 | merged | 28 |
| 1.4 | #624 | #628 | merged | 28 |
| 1.5 | #624 | #629 | merged | 28 |
| 1.6 | #624 | #630 | merged | 28 |
| 1.7 | #624 | #631 | merged | 28 |
| 2.1 | #632 | #633 | merged | 28 |
| 2.2 | #632 | #634 | merged | 28 |
| Amendment 1 | #635 | #636 | merged | 28 |
| 3.3 | #635 | #637 | merged | 26 |
| 3.1 | #635 | #638 | merged | 25 |
| 3.2 | #635 | #639 | merged | 24 |
| Amendment 2 | #635 | #640 | merged | 24 |
| 3.4 | #635 | #641 | merged | 24 |
| 3.5 | #635 | #642 | merged | 23 |
| 3.6a | #635 | #643 | merged | 23 |
| 3.6b | #635 | #645 | merged | 23 |
| Amendment 3 | #635 | #646 | merged | 23 |
| Amendment 4 | #635 | #647 | merged | 23 |
| 3.8 | #635 | #648 | merged | 23 |
| 3.9 | #635 | #649 | merged | 20 |
| Amendment 5 | #650 | #651 | merged | 20 |
| 4.1 | #650 | #652 | merged | 20 |
| 5.1 | #653 | #654 | merged | 20 |
| Amendment 6 | #653 | #655 | merged | 20 |
| 5.2a | #653 | #656 | merged | 20 |
| 5.2c | #653 | #657 | merged | 20 |
| 5.2d (listener) | #653 | #660 | merged | 20 |
| 5.2d (move) | #653 | #661 | merged | 20 |
| Amendment 7 | #653 | #664 | merged | 20 |
| 5.2b-i | #653 | #665 | merged | 20 |
| 5.2b-ii | #653 | #666 | merged | 17 |
| 5.3a (exec) | #653 | #667 | merged | 16 |
| 5.3b (move) | #653 | #668 | merged | 16 |
| 5.3c (fsread) | #653 | #669 | merged | 15 |
| 5.3d (pubsub) | #653 | #670 | merged | 15 |
| 5.4a (store) | #653 | #671 | merged | 15 |
| 5.4b (move) | #653 | #672 | merged | 15 |
| 5.4c (ports) | #653 | #673 | merged | 13 |
| 5.4d (ctx) | #653 | #674 | merged | 13 |
| 5.5a (launch) | #653 | #675 | merged | 13 |
| 5.5b (agentcli) | #653 | #676 | merged | 11 |
| 5.5c (launcher) | #653 | #677 | merged | 8 |
| 5.6a | #653 | #678 | merged | 8 |
| 5.6b | #653 | #679 | merged | 8 |
| Amendment 8 + 5.7 | #653 | #680 | merged | 8 |
| 5.8a (fsread) | #653 | #681 | merged | 7 |
| 5.8b (ports) | #653 | #683 | merged | 4 |
| 5.8c (move) | #653 | this PR | open | 4 |

*Correction (3.6a):* 3.3's PR said gate's `os` and `syscall` findings
"belonged to the pure half". They did not. Those imports are in `run.go`,
`detect.go` and `procgroup_*.go`, which stayed. The 28 → 26 drop was
reclassifying the rest of `internal/gate` as service, which does not flag `os`
or `syscall`. The work is still visible as `gate` → `os/exec` and goes in 5.3.
From 3.6a on, a mixed package keeps its old classification until the step
that empties it, so the count moves only when code does.

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
     - every path a test builds relative to its own directory
       (`filepath.Join("..", "..", "testdata", ...)`, `golist.List("..", ...)`).
       These gain a level when the package moves under a layer directory.
       1.1 and 1.6 each found this through a failing test.
   - A move PR may exceed the 400-line target; every other PR stays under it.
3. **Extractions leave aliases behind.** When a type moves out of a mixed
   package, the old package keeps `type X = newpkg.X` so importers keep
   compiling. The importers then move over in small PRs. Stage 8 deletes the
   aliases, so none outlives the migration.
   - **Aliases alone can break the SDP gate.** If every importer keeps using
     the aliases, the new domain package has one dependent, so it can be
     *less* stable than the package that depends on it. Step 3.1 hit exactly
     this: `registry` → `domain/session` at -0.056. So an extraction also
     points enough of the type-only importers at the new package, in the same
     PR, to keep the margin healthy, and says which it moved.

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
| 3.4 | `refactor: domain/coverage` | the package moves to `domain/coverage` whole. *Amendment 2:* `load.go` and `module.go` (the two file reads) stay with it until step 5.3, because `ui` calls `coverage.Load` directly, and moving the reads to infra now would add a tui → infra finding. | M | ADR tree | 0 | revert |
| 3.5 | `refactor: domain/forge types out of infra/forge` | PR, Issue, Detail, Comment, CI, Kind, Label; aliases in `infra/forge` | M | ADR tree | 1.6 | revert |
| 3.6a | `refactor: domain/review from review (model)` | anchor, change, comments, compose, digest, entries, pair, place, tree, treecompact, turnplace, shippable; aliases | M | invariants 7 and 8 | 3.1 | revert |
| 3.6b | `refactor: go-gitdiff parsing to infra/gitdiff` | `review/parse.go`; the depguard `wrappers` rule retargeted | M | invariant 4 in spirit | 3.6a | revert |
| 3.6c | *Folded into 5.8 by Amendment 3.* | `ui` defaults `Deps.Preview` to `review.ReadPreview`, and the header read serves `Source.Generated`, so moving the reads before `service/review`'s `FileReader` port would add a tui → infra finding. | — | — | — | — |
| 3.7 | *Moved to 5.2b by Amendment 4.* | `agent.Claude()` composes the watcher's transcript parser, `hooks.Render` and `paths.Transcript`, and `Profile.Status` has the type `watcher.Adapter`, so a pure `domain/agent` needs the `Adapter` port first. | — | — | — | — |
| 3.8 | `refactor: move tally to domain` | after its inputs are domain types | L | ADR tree | 3.1, 3.5 | revert |
| 3.9 | `refactor: crap and depgraph take their own input types` | *Amendment 1.* Each declares the input struct it reads: import path, imports, file names and their source bytes and mtimes. `tools/crapcheck`, `depcheck` and `layercheck` convert `golist.Package` into it. crap's source and mtime reads (`score.go`, `stale.go`) move into `tools/crapcheck`. Removes 3 findings: crap → golist, crap → os, depgraph → golist. | M | ADR tree (domain is pure) | 2.2 | revert |

### Stage 4: every git call gets a context and a deadline (a behaviour change)

| # | PR | Scope | Risk | Serves | Needs | Rollback |
|---|---|---|---|---|---|---|
| 4.1 | `feat: every git call has a deadline` | *Amendment 5.* The two exec sites in `infra/vcs` (`capture`, `Attr`) use `exec.CommandContext` with a named deadline: git 30 s, `git worktree add` 60 s. A hung git is killed and reported as a `CommandError`, instead of freezing the TUI. **User-visible only when git hangs.** No signature changes: `ctx` parameters arrive with the ports in 5.4 and 5.8, which then own the deadlines. | M, **smoke** | P5, audit leak 6 | 1.2 | revert |

### Stage 5: pubsub and the services

These steps move logic, not just files. The key and message tables are the
net: a logic PR that changes a row says which row and why (rule 4).

| # | PR | Scope | Risk | Serves | Needs | Rollback |
|---|---|---|---|---|---|---|
| 5.1 | `feat: internal/pubsub` | `Broker[T]` with `Publish` (waits) and `Offer` (drops and counts); `Event[T]`. It has no users yet. | L | ADR event model | 0 | revert |
| 5.2a | `refactor: service/status publishes status events through pubsub` | *Amendment 6, first of three.* `watcher.Watch` publishes `Event[status.Event]` through a `pubsub.Broker`, and the TUI subscribes with a re-armed Cmd instead of reading a raw channel. The tailer and listener are unchanged inside it. The package keeps its path here: renaming its ~30 importers now would do it twice. | H, **smoke** | ADR event model, invariant 2 | 5.1 | revert; blocks 5.2c, 5.2d |
| 5.2c | `refactor: the transcript tailer moves to infra/transcript` | *Amendment 6.* The tailer yields raw lines behind the `Transcripts` port. Deriving kinds and usage (the ring, emit-on-change) moves up into `service/status`. | H, **smoke** | ADR ports, invariant 2 | 5.2a | revert |
| 5.2d | `refactor: the hook listener moves to infra/hookserver` | *Amendment 6.* The listener yields raw payloads behind the `HookEvents` port, published with `Offer` so a hook never waits (invariant 11). `KindOf` moves up into the service. With only the service left, `internal/watcher` moves to `internal/service/status` here. | H, **smoke** | ADR ports, invariant 11 | 5.2a | revert |
| 5.2b-i | `refactor: the status Adapter port moves to domain/status` | *Amendment 7.* The `Adapter` interface and `Entry`, the only type it names that is not already domain, move to `domain/status`; `service/status` keeps aliases and Claude's parser. A move: no importer changes. | L | ADR ports; P2 | 5.2 | revert |
| 5.2b-ii | `refactor: domain/agent; the profile catalog is composed in cmd` | *Amendment 4, was 3.7; Amendment 7.* `Profile` and the command template (`ClaudeCommand`) go to `domain/agent`, with `Status` typed by `domain/status`'s `Adapter`. The catalog (`Claude()`, `Lookup`, `Names`), which composes the parser, `hooks.Render` and `paths.Transcript`, is built in `cmd`. That is where M17's #521 `Catalog` lands. The TUI's fallback to `agent.Claude()` for an unset profile goes, since the TUI cannot reach `cmd`; `cmd` already sets it. Tests in `supervisor` and `ui` build the claude profile with a named helper. Removes agent → hooks, paths and service/status. | M | ADR Exceptions (paths); P2 | 5.2b-i | revert |
| 5.3 | `refactor: service/gate and infra/gateexec` | the runner and bound become the service, which publishes `Event[gate.Report]`. `run.go`, `procgroup_*` and `detect.go` go to `infra/gateexec`. The exec allowlist changes as the ADR records. *Amendment 2:* `domain/coverage`'s `load.go` and `module.go` move to a new `infra/fsread`, behind `service/gate`'s `ProfileReader` port; `ui` stops calling `coverage.Load`. Removes the coverage → os finding. | M, **smoke** (`gateprobe`) | invariant 12, ADR Enforcement | 3.3, 5.1 | revert |
| 5.4 | `refactor: service/sessions (state and commands)` | registry's commands and create become the service. `store.go` and carry's copy go to `infra/store` behind `StateStore`. The ports carry `ctx`. | M | ADR ports, P2 | 3.1, 4.1 | revert; blocks 5.5–5.7 |
| 5.5 | `refactor: starting a session returns session.Launch` | `supervisor/launch.go` becomes `sessions.Start`, which returns argv/env/dir. The `Holder` port works over argv. `tui/terminal` (still `termwrap`) spawns. `namer.go` goes to `infra/agentcli`. | H, **smoke** + `dtachprobe` | ADR "Starting a session", invariant 9 | 5.4 | revert |
| 5.6a | `refactor: create and start leave Update` | `sessionlife.go:90/44/107` become Cmds; `Event[session.Session]` feeds the sidebar | H, **smoke** | P5, audit leak 5 | 5.5 | revert |
| 5.6b | `refactor: archive, rename, rebind and fold leave Update` | `archive.go:201`, `rename.go:62`, the rebind and fold writes, `gaterun.go:129` tally | M, **smoke** | P5 | 5.6a | revert |
| 5.6c | `refactor: discovery and adoption leave Update` | `discovery.go:157` (git rev-parse per root), `adopt.go` | M | P5 | 5.6a, 5.9 | revert |
| 5.7 | `refactor: the idle sweep's policy moves to domain/status` | *Amendment 8.* `Settled`, `LastActive` and `Sweepable` - which session the idle sweep may stop - become pure functions in `domain/status`, tested there. The sweep and repo-stat timers stay in the TUI: both decide from state only the TUI holds (the selected pane, live terminals, typing, window focus), and the stat read already runs off `Update`. | L | P2 | 5.6a | revert |
| 5.8 | `refactor: service/review` | `review/source.go`, turn loading, ship, revert, behind `DiffSource`, `DiffParser`, `FileReader` and `Shipper`. *Amendment 3:* also 3.6c: `preview.go`'s read and `generated.go`'s header read move to `infra/fsread` behind `FileReader`, and `ui` stops defaulting to `review.ReadPreview` | M | ADR ports | 3.6b, 3.6c, 5.4 | revert |
| 5.9 | `refactor: service/tracker and service/discovery` | PR and issue polling (`Event[tracker.Snapshot]`), item, browse; `discover` | M | ADR event model | 3.5, 5.1 | revert |
| 5.10 | `refactor: cmd is the only composition root` | `ui/run.go`'s wiring goes to `cmd`. `RunDeps` and `modelFor` go. `Deps` shrinks to the services and presentation-only settings. Dependencies are plumbed once. | M, **smoke** | P2, audit leak 2, invariant 10 | 5.2–5.9 | revert |

**After 5.10:** M17's code issues may start (the decision above), on
`domain/agent` profiles plus the `domain/status` Adapter port (Amendment 7).

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
| 6.10 | `refactor: the TUI reaches chroma through a Highlighter port` | *Amendment 1.* `tui` declares `Highlighter{ Lines(path string, lines []string) []string }`, today's `highlight.Lines`. `cmd` injects `infra/highlight`. `diffhighlight.go:68` and `tree.go:227` call the port. The SGR goldens must not move. Removes the tui → infra/highlight finding. | L | ADR ports, Enforcement | 6.2 | revert |
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

48 PRs after this one: 7 in Stage 1, 2 in Stage 2, 11 in Stage 3, 1 in Stage
4, 12 in Stage 5, 10 in Stage 6, 2 in Stage 7, 3 in Stage 8 (Amendment 1 added
3.9 and 6.10).

- **Parallel work:** Stages 1 and 2, and most of Stage 3, can proceed in
  parallel branches, because their packages don't overlap.
- **Serial work:** Stage 5 onward is mostly serial, because each step feeds
  the next.

## Amendments

**Amendment 1** (2026-09-29, #635). After Stage 2, every one of the 28 layer
findings was checked against the steps meant to remove it. Four had no step:
- `domain/crap` → `infra/golist`, `domain/depgraph` → `infra/golist` and
  `domain/crap` → `os`. Both packages read `golist.Package`, and crap parses
  and stats source files. **New step 3.9:** each declares its own input
  types, and the tools do the reading.
- `ui` → `infra/highlight`. The TUI calls chroma directly while rendering,
  and the ADR had no port for it. **New step 6.10:** a `Highlighter` port,
  added to ADR 0001's ports table.

The maintainer chose both (over reclassifying crap/depgraph as tooling, and
over moving highlight into `tui/`). With them, every current finding has a
step that removes it, so Stage 8's zero is reachable.

**Amendment 2** (2026-09-29, #635). Step 3.4 as written moved coverage's
file reads to `infra/fsread`. But `ui` calls `coverage.Load` directly, so the
move would add a tui → infra finding and break rule 5. The maintainer chose to
defer the read: 3.4 moves the package whole, and the reads go to
`infra/fsread` in 5.3 behind the `ProfileReader` port ADR 0001 already plans.
The alternative was injecting a function through today's
Deps/RunDeps/modelFor plumbing, the very plumbing 5.10 removes.

**Amendment 3** (2026-09-29, #635). Step 3.6c hit the wall 3.4 did. `ui`
defaults `Deps.Preview` to `review.ReadPreview`, and `generated.go`'s header
read serves `Source.Generated`, so moving either read before
`service/review`'s `FileReader` port would add a tui → infra finding. The
maintainer folded 3.6c into 5.8, where that port arrives. Stage 3 continues
with 3.7 and 3.8.

**Amendment 4** (2026-09-29, #635). Step 3.7 was not a move. `agent.Claude()`
composes three implementations: the watcher's transcript parser,
`hooks.Render`, and `paths.Transcript` with `EvalSymlinks`. `Profile.Status`
is typed by `watcher.Adapter`, a service-layer type. A pure `domain/agent` needs
that `Adapter` contract to exist in a layer it may import, and the catalog
needs a composition root to live in. The maintainer moved the step to 5.2b,
right after `service/status` declares the port, with the catalog composed in
`cmd`. M17's injected `Catalog` (#521) will build on that. Stage 3 finishes
with 3.8 and 3.9.

**Amendment 5** (2026-09-30, Stage 4). Step 4.1 as written gave all 25
`vcs` methods a `ctx`, rippling through five interfaces (`vcs.Git`,
`registry.RepoRooter`/`SessionBrancher`/`BranchRenamer`, `discover.Git`) and
every caller in registry, review, discover, cmd and ui. Stage 5 retypes those
same interfaces as ports with `ctx` anyway. Every git call funnels through two
exec sites, so the maintainer chose to put the deadline there now: the same
user-visible fix, a small diff, and no plumbing done twice. The `ctx`
parameters, and the deadlines with them, move to the service ports in 5.4 and
5.8.

**Amendment 6** (2026-09-30, #653). Step 5.2 was more than a move. The
tailer and the hook listener both call the `Adapter` (`DeriveKind`,
`KindOf`), which an infra package cannot import once the `Adapter` is a
service port. So deriving kinds moves up into `service/status`, and the
adapters yield raw lines and raw payloads, as ADR 0001's `Transcripts` and
`HookEvents` ports already say. The maintainer split the step into three
High-risk PRs, each smoke-tested: 5.2a (the service and the TUI's
subscription), 5.2c (the tailer) and 5.2d (the listener).

**Amendment 7** (2026-09-30, #653). Step 5.2b as written typed
`domain/agent`'s `Profile.Status` by `service/status`'s `Adapter`: a domain →
service import, which ADR 0001's own layer table forbids. The ADR's ports
table said the same thing, so the conflict was in the ADR too. The `Adapter`
is a contract over domain types alone: `Kind`, `HookPayload` and `Tokens`
already live in `domain/status`, and `Entry` is a struct of strings, a time
and `Tokens`. So the maintainer moved the port down rather than the profile
up. The step splits in two: 5.2b-i moves `Adapter` and `Entry` to
`domain/status` (a move), and 5.2b-ii moves `Profile` to `domain/agent` and
the catalog to `cmd`. The ADR's `Adapter` row is corrected to match. Claude's
parser stays in `service/status`; only the contract moves.

**Amendment 8** (2026-09-30, #653). Step 5.7 moved the idle sweep and the
repo-stat tickers into `service/sessions`, publishing events. Both decide
from state only the TUI holds: the sweep spares the selected pane and needs
to know which sessions have a live terminal and when the operator last typed;
the stat poll pauses while the window is unfocused. A service would have to
be told all of that, which is the coupling the hexagon exists to avoid, and
the stat read already runs off `Update`. The maintainer chose to move the
sweep's pure policy to `domain/status` and leave both timers in the TUI as
presentation timers.

## When the plan is wrong

If a step shows the plan is wrong, the step stops and an amendment to this
file is proposed. Nothing is improvised. Typical causes:
- a move that cannot be mechanical
- a golden that has to change in a move
- a layer count that goes up

The amendment is its own small PR, reviewed before the step resumes.
