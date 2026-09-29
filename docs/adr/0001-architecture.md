# ADR 0001: A hexagonal layout for omatty

- **Status:** Proposed (#618). Nothing moves until this is accepted.
- **Date:** 2026-09-29
- **Evidence:** `docs/ARCHITECTURE_AUDIT.md` (#615). Every "pain point" below
  refers to that document.
- **Decided in design review, 2026-09-29:**
  - the tier: full hexagonal
  - the layout: nested, keeping today's names
  - a second driving adapter: `--json` CLI commands
  - `bubbles/v2` adopted for `key.Binding` only

## Context

omatty is 32,077 lines of Go in 27 packages. Twenty-six of them have one job
each, sit behind depguard seams, and pass the Stable Dependencies gate. The
twenty-seventh, `internal/ui`, holds 45% of the code and all five of the
audit's pain points:

1. a `*Model` with 100 fields and 518 methods;
2. dependencies plumbed through `wiring.go` → `RunDeps` → `modelFor` → `Deps`;
3. 24 separate `switch key` sites, with hand-kept help tables beside them;
4. `ReviewPane` and `modal` as unions of six and ten modes;
5. git and `state.json` work done synchronously inside `Update`, with no
   timeout on git.

The audit also found the boundaries leaking below `ui`:
- `ui/run.go` is a second composition root.
- `registry` does git work.
- `review` mixes the diff model with file reads.

Three things push towards a real layering rather than a tidier `ui`:
- the maintainer wants the core provably free of UI and I/O;
- a second driving adapter (a `--json` CLI) is in scope;
- M17 is about to widen the agent seam to nine agents, which multiplies
  whatever the current shape costs.

## Decision

### The layers

Dependencies point one way. Each rule is enforced by depguard (see
"Enforcement"):

| Layer | May import | Must not import |
|---|---|---|
| `internal/domain/*` | stdlib, other `domain/*` | anything else in the module; any I/O package |
| `internal/service/*` | `domain`, `pubsub`, stdlib | `infra`, `tui`, `cli`, bubbletea, `os/exec`, `net/http` |
| `internal/infra/*` | `domain`, stdlib, the one third-party library or CLI it owns | `service`, `tui`, `cli`, `pubsub` |
| `internal/pubsub` | stdlib | everything in the module |
| `internal/tui/*` | `service`, `domain`, `pubsub`, bubbletea, lipgloss, bubbles | `infra` |
| `internal/cli` | `service`, `domain` | `infra`, `tui`, bubbletea |
| `cmd/omatty` | everything | nothing. It is the only composition root. |

An infra adapter satisfies a service's port structurally. It never imports
the service that declares the port, which keeps the rule acyclic without a
shared "ports" package.

### The tree

Each entry names the package it comes from.

```
cmd/omatty/            flags → config → adapters → services → tui.Run or cli
internal/
├── domain/                        stdlib only, no I/O
│   ├── session/       registry/state.go types, naming, per-project gate config
│   ├── review/        review's pure part: Diff, Hunk, Entry, Comment, anchor,
│   │                  change, compose, digest, entries, pair, place, tree,
│   │                  treecompact, turnplace, shippable
│   ├── gate/          gate/gate.go, compose.go, coverage.go (percentage parse)
│   ├── coverage/      coverage minus load.go: Profile, goprofile, lcov, blocks
│   ├── forge/         forge's types: PR, Issue, Item, CI, Kind, Label
│   ├── status/        watcher/event.go, status.go; the hook payload type
│   ├── agent/         agent: Profile, Caps, the command template
│   ├── tally/         tally
│   ├── fuzzy/ paste/  unchanged
│   └── crap/ depgraph/ unchanged (fed by tools/, see below)
├── service/                       use cases; each declares its own ports
│   ├── sessions/      registry commands, create, fold, rebind, carry logic;
│   │                  supervisor/launch; the idle-stop sweep; repo stat
│   ├── review/        review/source.go, turn loading, tree, preview, ship, revert
│   ├── gate/          gate/runner.go, bound.go
│   ├── status/        watcher/watch.go, adapter.go, guard.go, transcript.go
│   ├── tracker/       the polling and folding half of ui's tracker
│   └── discovery/     discover/choose.go, discover.go
├── infra/                         driven adapters; one owner per CLI or library
│   ├── vcs/ forge/ detach/ notify/ highlight/ golist/ config/ paths/
│   ├── store/         registry/store.go and carry's file copy
│   ├── gitdiff/       review/parse.go (the only go-gitdiff importer)
│   ├── gateexec/      gate/run.go, procgroup_*.go, detect.go
│   ├── transcript/    watcher/tailer.go, discover/sessions.go
│   ├── hookserver/    watcher/listener.go
│   ├── hooks/         hooks: the hooks.json writer and the `omatty hook` client
│   ├── agentcli/      supervisor/namer.go (`claude -p`)
│   └── fsread/        review/preview.go, review/generated.go reads, coverage/load.go
├── pubsub/            Broker[T], Event[T]
├── cli/               driving adapter 2: sessions/status --json, adopt, rm, stats
└── tui/               driving adapter 1: today's internal/ui
    ├── app/           root model: layout, focus, leader, overlays, tea.View
    ├── msg/           cross-screen messages
    ├── screen/        the Screen interface
    ├── keys/          today's internal/keys leader router
    ├── theme/         palette and every lipgloss style
    ├── components/    picklist, editline, listwindow, spinner, meter, card, sidebar
    ├── terminal/      today's internal/termwrap (see Exceptions)
    └── screens/       diff/ tree/ preview/ gate/ tracker/ trackeritem/ modals/*
```

**Exceptions, stated so nobody fixes them by accident:**
- **`termwrap` → `tui/terminal`, not `infra`.** bubbleterm is a bubbletea
  component that owns its own PTY: it takes the command and starts it. So the
  package that spawns claude is necessarily a TUI component. This is today's
  exception (AGENTS.md "Repository layout"), moved rather than solved. The
  session service hands it a `session.Launch` value (below), so the service
  itself stays UI-free. `creack/pty` stays fenced to this one package.
- **`paths` is infra.** Where `~/.omatty` lives is an adapter's business, and
  the domain never learns it. `agent`'s current imports of `paths`, `hooks`
  and `watcher` are cut: the paths arrive as values from `cmd`.
- **`crap`, `depgraph` and `golist` stay under `internal/`**, as
  `domain/crap`, `domain/depgraph` and `infra/golist`, rather than moving
  beside `tools/`. Anything outside `./internal/...` drops out of the 90%
  coverage gate.

### The ports

These rules hold for every port:
- **The consumer declares it**, as `registry.RepoRooter` and `watcher.Adapter`
  do today.
- **It is narrow:** one to three methods, named for the capability.
- **Every method that does I/O takes a `context.Context`.** The service that
  owns the call sets the deadline as a named constant: git 30 s, `git
  worktree add` 60 s, the forge's existing timeouts unchanged. This is what
  ends pain point 5.

| Declared in | Port | Methods | Implemented by |
|---|---|---|---|
| `service/sessions` | `StateStore` | `Load(ctx) (session.State, error)`; `Save(ctx, session.State) error` | `infra/store` |
| `service/sessions` | `RepoRooter`, `SessionBrancher`, `BranchRenamer` | today's methods, plus `ctx` | `infra/vcs` |
| `service/sessions` | `Worktrees` | `Add(ctx, root, dir, branch, base string) error`; `Remove(ctx, root, dir string) error` | `infra/vcs` |
| `service/sessions` | `Holder` | `Wrap(id string, argv []string) ([]string, error)`; `Stop(id string) error`; `Persists() bool`; `Held(id string) (bool, error)`. This is today's `detach.Holder` re-expressed over argv, because its `*exec.Cmd` signature would put `os/exec` in the service layer. | `infra/detach` |
| `service/sessions` | `Namer` | `Name(ctx, transcript string) (string, error)` | `infra/agentcli` |
| `service/sessions` | `RepoStats` | `Stat(ctx, dir, base string) (sessions.RepoStat, error)` | `infra/vcs` |
| `service/review` | `DiffSource` | `Diff(ctx, dir, base string) ([]byte, error)`; `Snapshot(ctx, dir string) (string, error)` | `infra/vcs` |
| `service/review` | `DiffParser` | `Parse(raw []byte) (review.Diff, error)` | `infra/gitdiff` |
| `service/review` | `FileReader` | `Preview(ctx, dir, rel string) (review.Preview, error)`; `Generated(ctx, dir string) ([]string, error)` | `infra/fsread` |
| `service/review` | `Shipper` | `Push`, `OpenPR`, `Merge`, as today's `ShipFuncs` | `infra/vcs`, `infra/forge` |
| `service/gate` | `StepRunner` | `Run(ctx, dir string, s gate.Step) gate.StepResult` | `infra/gateexec` |
| `service/gate` | `ProfileReader` | `Load(ctx, path, dir string) (coverage.Profile, error)` | `infra/fsread` |
| `service/status` | `Transcripts` | `Tail(ctx, path string) (<-chan []byte, error)`; `Read(ctx, path string) ([]byte, error)` | `infra/transcript` |
| `service/status` | `HookEvents` | `Listen(ctx) (<-chan status.HookPayload, error)` | `infra/hookserver` |
| `service/status` | `Adapter` | today's `watcher.Adapter`, with `hooks.Payload` becoming `status.HookPayload` | `domain/agent` profiles |
| `service/status` | `Notifier` | today's `notify.Notifier` | `infra/notify` |
| `service/tracker` | `Lister`; `ItemReader`; `Browser` | `PRs`/`Issues(ctx, root)` and the cache-only `Label(root)`; `Item(ctx, root, ref)`; `Open(ctx, url)`, as today's `ForgeBrowseFuncs` | `infra/forge` Router |
| `service/discovery` | `TranscriptStore`; `Git` | `Sessions(ctx)`; today's `discover.Git` | `infra/transcript`, `infra/vcs` |

**Starting a session.** `service/sessions.Start` returns a value,
`session.Launch{Argv, Env, Dir}`: the agent's command template, filled in and
wrapped by the `Holder`. `tui/terminal` spawns it. The service decides what
runs, the terminal component runs it, and neither imports the other's
libraries.

**The driving side.** `tui/screens/*/cmds.go` and `internal/cli` call services
through small interfaces that each screen or command declares for itself: the
same consumer rule, one level up. Screens are tested with named fakes
(AGENTS.md), never with the real services.

**Errors.** Domain errors live in their domain package, for example
`session.ErrUnknownSession`, and carry the offending value, per AGENTS.md.

### The event model

Services publish; the TUI and the notify adapter subscribe.

| Topic | Publisher | Replaces |
|---|---|---|
| `Event[status.Event]` | `service/status` | `Deps.Events` and `ui/status.go:29` |
| `Event[gate.Report]` | `service/gate` | `Deps.GateReports` and `ui/gaterun.go:26` |
| `Event[session.Session]` (Created, Updated, Deleted) | `service/sessions` | the TUI rebuilding the sidebar from its own copy of `State` after each command |
| `Event[tracker.Snapshot]` | `service/tracker`, on its own ticker at today's intervals | the `prpoll.go` and `issuepoll.go` tick chains |
| `Event[sessions.RepoStat]` | `service/sessions` | the `repostat.go` tick |
| `Event[status.Attention]` | `service/status` | the TUI calling `notify` itself |

```go
// internal/pubsub
type Kind int // Created, Updated, Deleted

type Event[T any] struct {
    Kind    Kind
    Payload T
}

type Broker[T any] struct{ /* subscribers, each a bounded buffer */ }

func (b *Broker[T]) Subscribe(ctx context.Context) <-chan Event[T]
func (b *Broker[T]) Publish(ctx context.Context, e Event[T]) // waits for room; never drops
func (b *Broker[T]) Offer(e Event[T]) bool                    // never waits; drops and reports it
```

**Delivery semantics are today's, kept exactly, and chosen per publisher:**
- **The transcript tailer and the gate runner use `Publish`.** Both block on
  a full buffer today (`watcher/tailer.go:190`, `gate/runner.go:182`), and a
  lost verdict or status would contradict invariant 2's "status is truth".
- **The hook server uses `Offer`.** Today's listener drops on a full sink and
  counts it (`watcher/listener.go:224`), so a hook never waits on omatty
  (invariant 11). The tailer re-derives the truth from the JSONL anyway.
- Buffers keep today's size of 64 per subscriber. A subscriber's context
  ending unsubscribes it.

**How the TUI consumes events.** `tui/app` subscribes in `Init` and drains
each topic with one re-armed Cmd: today's `waitForEvent`, generalized once.
There is no `p.Send` and no global channel. `internal/cli` subscribes to
nothing, because its commands are one-shot reads.

**Timers that are presentation stay in the TUI:** the spinner and the repaint
heartbeat. Timers that are lifecycle or data move to their service: the
idle-stop sweep, repo stat, and the PR and issue polls.

### The TUI

- **`tui/app` is the root and the only thing that returns `tea.View`.** It
  owns the layout math, with sizes flowing down through `SetSize` from
  `WindowSizeMsg`. It also owns focus, the overlay stack, and a read-only
  `session.State` kept current from `Event[session.Session]`.
- **The leader router runs before any screen sees a key.** That is
  `tui/keys`, today's `keys.Router`, and it is how invariant 1 is kept: while
  the terminal has focus, every key except the leader goes to the PTY.
- **`Screen` is used where the code switches on a mode enum today:** the
  review column's six views and the ten modal kinds. The sidebar and the
  terminal are fixed panes, so they are components, not screens.

  ```go
  type Screen interface {
      Init() tea.Cmd
      Update(tea.Msg) (Screen, tea.Cmd)
      View() string
      SetSize(w, h int)
      Title(budget int) string  // the column title rules, #283/#285/#287
      Bindings() []key.Binding  // what help and the footer list
  }
  ```
- **One package per screen:** `model.go`, `view.go`, `cmds.go`, `keys.go`.
  `ReviewPane`'s fields and `modal`'s union dissolve into the screen that owns
  each one. Per-session data (status, gates, PRs, issues, repo stat) is
  service state that reaches the TUI as events. `*Model` keeps only view state.
- **`tui/theme` holds the palette and every `lipgloss.NewStyle`,** including
  today's strays in `card.go`, `hairline.go`, `itempage.go`, `gatesearch.go`
  and `diffhighlight.go`.
- **Keys use `key.Binding` from `charm.land/bubbles/v2/key`: one binding per
  action, carrying every spelling that action accepts today.** The help modal
  keeps its own renderer, fed from `Bindings()` instead of the tables at
  `modalview.go:42-171`. `help.Model` is not adopted, because it would change
  how help looks. **This adds a dependency, and here is the argument AGENTS.md
  asks for:** the project has no binding type of its own, and pain point 3 is
  drift between 24 key-handling sites and a help table nothing ties to them.
  The dead `"shift+N"` spelling still accepted at three sites, after #87
  recorded it as dead, is that drift. No other bubbles package is imported,
  and depguard allows bubbles in `tui/*` only.

### The second driving adapter

`internal/cli` adds `omatty sessions --json` and `omatty status --json`. These
are one-shot reads through `service/sessions` and `service/status`; the
status command uses the transcript `Read` port rather than a tail. Today's
`adopt`, `rm` and `stats` subcommands move onto the same services.

The adapter exists to prove the rule, not as a product surface. If `cli`
compiles and passes its tests with no bubbletea in its import graph, the core
is UI-free. The JSON shape is not frozen below 1.0, like `state.json` and the
key table.

### Enforcement

- **New depguard rules** encode the layer table above. They run
  **report-only** from the safety-net phase and switch to **enforcing** in the
  last migration PR.
- **Today's fences carry over under the new paths**, and their `go list`
  assertions (`scripts/depguard_test.go`, `scripts/network_test.go`) move with
  them:
  - bubbleterm and pty → `tui/terminal`
  - go-gitdiff → `infra/gitdiff`
  - chroma → `infra/highlight`
  - `net/http` → `infra/forge`
  - `TestNoGitOutsideVcs` → `infra/vcs`
  - `TestNoGhOutsideForge` → `infra/forge`
- **The `os/exec` allowlist changes, and this is the decision AGENTS.md says
  must be written down:**
  - out: `supervisor`, `gate`, `termwrap`
  - in: `infra/gateexec`, `infra/agentcli`, `tui/terminal`
  - kept, with new paths: `infra/{detach,forge,golist,notify,vcs}`
- **The SDP gate is unchanged** and must stay green on every PR. Layering
  should widen its margins: domain packages sit near I=0, adapters near 1.
- **The 500-line file gate, funlen 20, gocognit 15, C.R.A.P. 12 and the 90%
  coverage floor** keep running throughout. No PR raises a limit.

### Invariants

All twelve stand. These are the ones the move touches, and how each is kept:

| # | Held by |
|---|---|
| 1 Modal key routing | `tui/keys` runs in `tui/app` before any screen |
| 2 Status from JSONL and hooks | `service/status` is the only producer of `status.Event`; no package reads the cell grid |
| 3 Zero footprint | `infra/hooks` still writes only `~/.omatty/hooks.json` |
| 4 bubbleterm and git behind one package each | fences retargeted (Enforcement) |
| 5 stdout belongs to the TUI | forbidigo is unchanged; `internal/cli` prints only on the non-TUI path, which `cmd` selects before the TUI starts |
| 9 `state.json` relaunches everything | `domain/session` keeps the JSON tags byte-identical; a golden round-trip test pins the schema before the move |
| 10 `cmd/` stays thin | `cmd` wires and dispatches; the subcommand logic moves into `internal/cli` |
| 11 A hook never blocks | the `omatty hook` client moves unchanged into `infra/hooks`; the server keeps drop-not-wait (`Offer`) |
| 12 Verdicts from exit status | `infra/gateexec` keeps exit-code verdicts and the LookPath pre-flight |

## Alternatives considered

- **UI-focused.** Restructure `internal/ui` and the cmd/ui seam only; keep
  the 26 other packages. It targets every pain point the audit ranked, at a
  fraction of the churn. Rejected: it leaves `registry`, `review` and
  `watcher` mixing domain and I/O, and nothing would mechanically prove the
  core UI-free. The maintainer wants both, and the second adapter needs the
  services this option does not create.
- **Minimal fixes.** Move the wiring to `cmd`, make the blocking calls
  asynchronous, add git timeouts. Rejected: the god object stays one type,
  only smaller.
- **Flat layers**, one package per layer. Rejected: each would hold 5–15k
  lines, breaking one-responsibility-per-package and the 500-line gate at
  once.
- **Vertical slices**, `internal/<feature>/{domain,service,infra}`.
  Rejected: adapters are shared across features (`vcs` serves sessions,
  review, discovery and stats), so slices would duplicate them or import each
  other sideways.
- **Adopting `help.Model` as well as `key.Binding`.** Rejected: it changes
  how help renders, which is user-visible.
- **A drop-oldest broker** (crush's choice). Rejected: a dropped verdict or
  status is a wrong screen. Only the hook server drops, as it does today.

## Consequences

- **Almost every import path changes.** Package moves are `git mv` with no
  edits, one PR each, so blame follows the file. Extracting types out of the
  mixed packages (`registry`, `review`, `gate`, `watcher`, `coverage`, `forge`)
  is separate work, in separate PRs, after the moves.
- **Behaviour is unchanged:** no flag, config key, keybinding, screen or
  `state.json` field changes. The one addition is the two `--json` commands.
- **One dependency is added**, `charm.land/bubbles/v2`, fenced to `tui/*`.
- **Documentation moves with the code.** `docs/ARCHITECTURE.md` and
  AGENTS.md's "Repository layout" and "Dependencies" sections are updated in
  the PR that changes what they describe.
- **M17 lands on the new shape.** The agent seam becomes `domain/agent`
  profiles plus the `service/status` Adapter port, so nine agents are nine
  profiles, not nine edits to `ui`. Work under way on M17 has to be sequenced
  against the migration, which is Phase 3's job.
- **The cost is time and review attention.** Phase 3 sequences the moves;
  this ADR fixes only where they end.

## What this ADR does not decide

- **The PR order, sizes, risk and rollback of each step.** That is Phase 3,
  `MIGRATION_PLAN.md`.
- **The characterization tests that pin behaviour before anything moves.**
  That is Phase 2.
- **Whether the existing key-spelling inconsistencies are fixed.** That is a
  separate issue. This migration keeps every spelling that works today.
