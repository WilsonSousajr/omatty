# omatty architecture

The one-page version: how data moves, what each package is for, why each
invariant is a rule, and where the seams are. AGENTS.md states the rules; this
says what would break without them. The design that produced the shape is
`docs/superpowers/specs/2026-09-01-omatty-design.md`; the order things were
built in, and what was cut, is `docs/ROADMAP.md`.

omatty is a terminal window around N real `claude` processes. It never
reimplements Claude's interface: each session is the actual binary in its own
PTY, rendered through a terminal emulator, and omatty owns only the panes
around it - sidebar, status, review column, footer.

## Data flow

**The process model.** One omatty. One `claude` per session. `internal/service/sessions`'
`Launcher` decides how a session starts - fresh with the UUID omatty assigned, or
`--resume <uuid>` - and hands back a `session.Launch` (argv, env, directory: the
main checkout or a worktree omatty created), which `internal/tui/terminal` runs
in its own PTY. Where the `dtach` binary exists, the `Holder` port wraps that
command line in a dtach master that outlives omatty, so quitting detaches
instead of killing (#43). `~/.omatty/state.json` is the registry of projects
and sessions and is, by design, enough on its own to relaunch every session
with `--resume <uuid>` (invariant 9).

**Keys in.** A keystroke reaches the bubbletea model in `internal/tui/app`,
which asks `keys.Router` where it goes. The router is a two-state machine: with
a terminal pane focused, every key is forwarded to that pane's PTY through
`internal/tui/terminal`, except the leader (`ctrl+o` unless configured), which
arms the next key as an omatty command. With no terminal focused - a modal is
open, or nothing is selected - every key is omatty's. That is the whole routing
rule (invariant 1). Inside the review column each face matches keys through
`key.Binding`s, and the help modal is built from the same bindings, so a key
cannot be handled without being documented (#422, #653).

```
keystroke -> keys.Router -> app.Model -> terminal.Terminal -> PTY -> claude
                  \-> omatty command (n, x, /, d, ...) when armed or unfocused
```

**Status out.** Nothing reads the screen. Two structured sources feed
`internal/service/status`:
- the transcript JSONL Claude Code writes under
  `~/.claude/projects/<slug>/<uuid>.jsonl`, read as it grows by
  `internal/infra/transcript` and tailed once a second;
- hook events: Claude runs `omatty hook` on each lifecycle event, which
  forwards the payload over a unix socket that `internal/infra/hookserver`
  listens on.

Hooks are fast, and they are the only source that can tell "waiting for you"
from "tool running". The transcript is the truth on attach, heals itself, and
is the only source of age and tokens. Both go through the agent's
`status.Adapter`, which turns one agent's lines and payloads into omatty's
neutral `Event` vocabulary in `internal/domain/status`. The service publishes
events through a `pubsub.Broker`. The TUI subscribes, and each event arrives
as a `StatusMsg` that the TUI folds into the session's `SessionState` with
`status.Apply`. That state drives:
- the sidebar glyph, a spinner while the session thinks or runs a tool (#410);
- the token meter;
- a desktop notification, when omatty is in the background.

`omatty status --json` reads the same transcript once, through the same
code (`status.Read`), with no timer and no socket.

```
claude --settings ~/.omatty/hooks.json
   |-- writes ~/.claude/projects/<slug>/<uuid>.jsonl --> transcript.Reader --> status.Tail ---\
   \-- runs `omatty hook` --> ~/.omatty/sock --> hookserver.Listen --> status (Adapter) ------+--> Event
                                                                                              |
   sidebar <-- SessionState <-- status.Apply <-- app.StatusMsg <-- Broker.Subscribe <----------/
```

**Review round trip.** The review column asks for a diff through a typed
function `cmd/omatty` hands the model at startup. That function is
`internal/service/review`'s `Source`, which reads through its own `Git` port,
implemented by `internal/infra/vcs`, the one place git is run.
`internal/infra/gitdiff` parses the unified diff into `internal/domain/review`'s
files, hunks and lines. The domain model holds the operator's comments anchored
on content (file, hunk header, line hash) rather than line numbers, and
composes the one message that sends them back. The TUI writes that message into
the session's PTY as a single bracketed paste followed by one carriage return.

```
ctrl+o d -> app -> DiffFunc (cmd wiring) -> review.Source -> vcs.CLI (git) -> gitdiff.ParseDiff
ctrl+o S -> review.Compose -> "\x1b[200~ ... \x1b[201~\r" -> PTY -> claude
```

**The tracker.** `ctrl+o i` turns the review column into the project's tracker:
its open issues and open pull requests, read through `internal/infra/forge`, the one
place any forge is reached. Since M16 that is every forge omatty reads -
GitHub, GitLab, Gitea/Forgejo/Codeberg, Bitbucket Cloud and Data Center, Azure
DevOps - behind one `forge.Router`. It names a project's forge from its
`origin` remote and reads it through that forge's own CLI (`gh`, `glab`, `tea`,
`az`) on the operator's own authentication, or, without it, through the
forge's REST API with a token borrowed from the environment for the one call
(#452, #453). Three reads, none of them while omatty is in the background: the
pull requests every minute (#310, #358), the issues every five, and one item
when `enter` opens it. Nothing is stored — the rows are derived at render time
from the last poll, so `state.json` gains nothing and invariant 9 is untouched —
and the tracker writes nothing to the forge. `n` on an issue goes the other way, through
the same `CreateFunc` the new-session prompt uses, and `a` types the item's
reference into a session's composer as a bracketed paste with no carriage return
(invariant 8).

```
ctrl+o i -> app -> IssueListFunc/PRListFunc (cmd wiring) -> forge.Router -> backend (CLI or REST) -> fold
enter    -> app -> ItemFunc                              -> forge.Router -> backend (CLI or REST) -> fold
n        -> app -> CreateFunc -> sessions.Creator -> vcs.CLI (git worktree add -b)
a        -> paste.BracketedText("issue #399 ") -> PTY -> claude's composer, unsent
```

**Wiring.** `cmd/omatty` is the one composition root (`cmd/omatty/tui.go`,
since migration step 5.10). It:
- parses flags;
- builds every adapter: the store, git, the holder, the hook server, the
  highlighter, the forge Router;
- builds the services over those adapters;
- starts the status service and the gate Runner;
- runs the model.

It holds no logic (invariant 10). The subcommands print plain text, and the
TUI is the only thing that ever owns stdout:
- `add`, `new`, `discover`, `gate` and `carry` call the services directly;
- `adopt`, `rm`, `gate --stats`, `sessions --json` and `status --json` run
  through `internal/cli`, the second driving adapter (ADR 0001).

## Package breakdown

The packages sit in the layers ADR 0001 lays out
(`docs/adr/0001-architecture.md`). Each layer may import only those below it:

| Layer | Path | May import |
|---|---|---|
| domain | `internal/domain/*` | other domain packages and the standard library. Nothing else: no I/O, no clock it was not handed. |
| service | `internal/service/*` | domain and `pubsub`. Never infra, `os/exec`, `net/http` or a UI library: each use case declares the ports it consumes, and `cmd` plugs infra into them. |
| infra | `internal/infra/*` | domain and third-party libraries. Never a service, the TUI, the CLI or pubsub: an adapter implements a port, it does not call a use case. |
| pubsub | `internal/pubsub` | nothing of omatty's. |
| tui | `internal/tui/*` | domain, service, pubsub, and bubbletea and its kin. Never infra: the TUI reaches git, chroma or a file through a function or port `cmd` hands it. |
| cli | `internal/cli` | domain, service and pubsub. No UI library anywhere beneath it, which is what proves the core is UI-free (`TestCLI_importsNoUILibrary_issue653`). |
| cmd | `cmd/omatty` | everything. It is the only place that does. |

`./scripts/check-layers.sh -enforce` holds the table in CI. The migration
moved the code into this shape one step at a time (`docs/MIGRATION_PLAN.md`),
and the check ran report-only until the last step left it with no finding
(#620, #653).

Within a layer the rule is one responsibility per package, typed APIs, and no
cycles. `internal/tui/app`, `internal/tui/terminal` and `internal/tui/theme`
are the only packages that import bubbletea or lipgloss; `terminal` because
bubbleterm is itself a bubbletea component, so `terminal.Terminal` returns
`tea.Cmd`. `depguard` enforces this (#260).

**domain** - pure.

| Package | Owns |
|---|---|
| `internal/domain/agent` | What a coding agent is: a command template plus a status adapter. Claude is the only profile (#46), composed in `cmd/omatty/agents.go` with the implementations it carries. |
| `internal/domain/coverage` | A coverage profile as per-line verdicts. Three states: covered, uncovered, and no verdict at all for a line that is not a statement. It parses a reader. |
| `internal/domain/crap` | Per-function complexity × coverage → a C.R.A.P. score, for the gate tool that holds the limit (#262). |
| `internal/domain/depgraph` | The internal import graph → Ca, Ce, instability and the SDP check `check-deps.sh` runs (#263, #269). |
| `internal/domain/forge` | A forge's work, independent of the forge: `PR` and its state, CI and review, `Issue`, `Detail` with its `Comment`s and `Check`s, and `Label`, how a forge names things; and the ways a forge says no that the TUI turns into a note - a missing tool, a refused or unsendable token, no forge, no tracker. |
| `internal/domain/fuzzy` | Subsequence ranking for the session switcher, the pickers and the tree filter. Table-tested. |
| `internal/domain/gate` | The gate's vocabulary - `Step`, `Verdict`, `StepResult`, `Report` - the output caps and the prompt `Compose` writes. |
| `internal/domain/paste` | Bracketed-paste envelopes for text omatty types into a session on the operator's behalf. Invariant 8 lives here because review and gate both need it. |
| `internal/domain/review` | The review model: a diff as files, hunks and lines; comments anchored on content, not line numbers (invariant 7); where they land after the diff moves; the file tree; the prompt `Compose` writes. |
| `internal/domain/session` | `Project`, `Session`, `State` - what `state.json` holds, whose JSON tags are invariant 9 - the placeholder title and branch a new session starts with, and `Launch`, the command line a session starts from. |
| `internal/domain/status` | A session's status vocabulary - `Kind`, `Status`, `Event`, `Tokens`, `SessionState`, `HookPayload`, the transcript `Entry` and the agent's `Adapter` port - and `Apply`, which folds an event into a state. |
| `internal/domain/tally` | A project's gate counters and its pull requests → lead time and first-pass rate (#332). |

**service** - the use cases, each declaring the ports it consumes.

| Package | Owns |
|---|---|
| `internal/service/discovery` | Proposes repositories and sessions to register, read from the agent's own transcript store; which bodies are typed prompts arrives as a `PromptText` port (#46, #653). Proposes only; never writes. |
| `internal/service/gate` | The `Runner`: gates many sessions at once, bounded, superseding a session's stale run, one session's panic kept to it (invariant 6). Reports are published through a `pubsub.Broker`; running a step and reading a coverage profile are injected ports. |
| `internal/service/review` | A session's diff, stat, turn baseline, revert and what can ship, read through its own `Git` port and an injected diff parser; the model it hands back is `domain/review`'s. |
| `internal/service/sessions` | The commands that edit projects and sessions (add, remove, rename, adopt, create, gate, carry), over a `StateStore` port and narrow git ports (`Worktrees`, `RepoRooter`, `SessionBrancher`, `BranchRenamer`); where a worktree goes arrives from `cmd` as a function. Creating a worktree also carries the project's gitignored paths into it, before the session is registered (#309). The `Launcher` builds a session's `Launch` - fresh start or resume, wrapped by the `Holder` port. |
| `internal/service/status` | Transcript lines and hook payloads → typed status events through the agent's `Adapter`, published through a `pubsub.Broker`; `Read` is the one-shot form `status --json` uses. |

**infra** - the driven adapters, each the only route to the thing it wraps.

| Package | Owns |
|---|---|
| `internal/infra/agentcli` | The agent's binary run headless for a one-shot answer - a session's name from its first prompt (#127) - under a timeout, never through the holder. |
| `internal/infra/config` | `~/.omatty/config.toml`. Every key optional; a missing file is every default. The only package that names a TOML library. |
| `internal/infra/detach` | omatty's only route to `dtach`. Returns a no-op holder when the binary is absent. |
| `internal/infra/forge` | omatty's only route to any forge, and its only HTTP client: a `Router` that names each project's forge from its remote and reads it through the forge's own CLI (`gh`, `glab`, `tea`, `az`) or its REST API - a project's pull requests, its open issues, one item in full, browse - on a timer, and the three writes the ship key makes, only on a keypress (#310, #394, #397, #331, #452-#465). A token is borrowed per call, sent only to the instance it is for, and stored nowhere (#453). |
| `internal/infra/fsread` | Files omatty only reads: a coverage profile, sniffed as Go or lcov, keyed against the module in the go.mod beside it (`service/gate`'s `ProfileReader`); a file's preview for the tree; and the head of a file the generated-file sniff reads (#338). |
| `internal/infra/gateexec` | A project's own verification commands, run under `sh` in a session's directory, and `Detect`, which proposes them. Verdicts come from exit status only (invariant 12). |
| `internal/infra/gitdiff` | The only package that imports go-gitdiff: parses git's unified output into `domain/review.Diff`, numbering every line on both sides. |
| `internal/infra/golist` | omatty's interface over `go list`, for the gate tools (#263). |
| `internal/infra/highlight` | omatty's only route to the syntax highlighter (chroma), with omatty's own colour style (#197); the TUI reaches it through its `Highlighter` port. |
| `internal/infra/hooks` | Renders and installs `~/.omatty/hooks.json`, written atomically and never through a link, and implements the `omatty hook` reporter. |
| `internal/infra/hookserver` | omatty's end of the hook socket: user-only, bounded connections and payloads, a read deadline; each payload is offered on and dropped rather than waited on (invariant 11). What a payload means is `internal/service/status`'s. |
| `internal/infra/notify` | Desktop notifications for a session that needs attention while omatty is blurred. |
| `internal/infra/paths` | Every filesystem location omatty reads or writes. Pure; takes `home` explicitly so tests never touch the real one. |
| `internal/infra/store` | `state.json`: loaded, and saved atomically so a crash cannot strand a session (invariant 9). Also carry's copy of a project's gitignored files into a new worktree (#309). |
| `internal/infra/transcript` | Reads an agent's JSONL transcript as it grows: the complete lines appended since the last poll, a truncation flag, the line cap. What the lines mean is `internal/service/status`'s. |
| `internal/infra/vcs` | omatty's only route to git, via the CLI. |

**pubsub, the driving adapters, and the root.**

| Package | Owns |
|---|---|
| `internal/pubsub` | `Broker[T]`: how a service tells its subscribers something changed. `Publish` waits for room and never drops (the tailer, the gate runner); `Offer` never waits and counts what it drops (the hook server, invariant 11). Stdlib only. |
| `internal/tui/app` | The bubbletea model: sidebar, panes, modals, review column, rendering. Every write to `state.json` goes through one FIFO writer, off the Update goroutine. |
| `internal/tui/keys` | The modal key router. A pure state machine with no bubbletea dependency. |
| `internal/tui/terminal` | omatty's only route to the terminal emulator (bubbleterm), and the PTY a session's `Launch` runs in. |
| `internal/tui/theme` | The palette and every style. No other TUI package builds a colour or a style of its own. |
| `internal/cli` | The second driving adapter: `sessions --json`, `status --json`, and the flows behind `adopt`, `rm` and `gate --stats`. |
| `cmd/omatty` | The binary and the one composition root. Flags, dependency construction, `omatty hook`. Thin by rule. |
| `tools/`, `scripts/` | The gate's own tools - `crapcheck`, `depcheck`, `layercheck` - and the scripts that run them. Outside the hexagon. |
| `testdata/` | `fake-claude`, `ptyrun`, `screen`, `dtachprobe`, `gateprobe`, `forgeprobe`: the harness for the real-PTY smoke test the gate cannot replace. |

## The twelve invariants, and why

AGENTS.md lists them as rules. Each one is here with the failure it prevents.

1. **Key routing is modal, never heuristic.** An earlier attempt as a LazyVim
   plugin intercepted keys around an embedded Claude by guessing which ones
   Claude wanted; Claude binds `esc`, `shift+tab`, `ctrl+r` and `ctrl+c`, and
   the guesses collided. The router therefore has no opinion about any key
   but the leader. Four modal surfaces were added in M4 without the router
   learning anything about them (ROADMAP, M4).

2. **Status comes from JSONL and hooks, never from the screen.** A cell grid
   is a rendering, not a fact: it changes with width, theme and scrollback.
   The structured sources are exact, which is also why `paths.TranscriptSlug`
   must match Claude's own directory transform character for character - #60
   was a path with a space the old mapping missed, which left the tailer
   blind and every restart starting a fresh session instead of resuming.

   `terminal.Text` reads that grid, and does not breach this. The invariant
   forbids *inferring session state* from a rendering, because the JSONL
   already reports it exactly. Copying the cells a drag selected is not an
   inference about anything: the text is the product, nothing derived from it
   reaches `service/status`, and no status is read off it (#360). The test of the
   rule is whether a wrong answer would mislead omatty about a session - a
   clipboard cannot.

3. **omatty never writes `~/.claude/settings.json`.** Hooks are injected
   per-process with `--settings ~/.omatty/hooks.json`. Zero footprint is a
   feature: uninstalling omatty leaves Claude exactly as it was, and two
   omatty versions cannot fight over one file.

4. **bubbleterm, git and dtach are reachable only through packages omatty
   owns.** bubbleterm is pre-1.0 and will break; the blast radius must be one
   file (`tui/terminal/bubbleterm.go`). go-git's linked-worktree support is
   experimental and implements only add and remove, so git is the CLI, and
   only `infra/vcs` runs it. `infra/detach` is the same rule for dtach. See "The seams".

5. **`stdout` belongs to the TUI.** A stray `fmt.Println` lands inside the
   alternate screen and corrupts it. Every diagnostic is structured JSON to a
   file under `~/.omatty/logs/`; `forbidigo` in the lint gate rejects the
   print family outright.

6. **One panicking session must not kill the app.** The tailer goroutine and
   the terminal's read loop each recover (`service/status/guard.go`,
   `tui/terminal/guard.go`) and mark their own session `✗`. N sessions are the
   point; one bad transcript line must not take the other N-1 down.

7. **Comments anchor on content, not line numbers.** Claude edits files while
   you read them. A line-number anchor silently attaches feedback to
   whatever code has since moved there; `(file, hunk header, line hash)`
   either finds the same line or reports that it is gone.

8. **Review submission is one bracketed paste, then one `\r`.** Writing a
   multi-line prompt raw sends N premature messages, one per newline.
   `ESC[200~ … ESC[201~` tells Claude the whole block is a single input
   (`paste/paste.go`, its own package since #223 so review and gate can both
   reach it without importing each other).

9. **`state.json` must always suffice to relaunch every session.** Crash
   recovery is `claude --resume <uuid>` (#36), and that only works if
   everything a relaunch needs is either persisted or derivable. A new
   session field is one or the other - `Agent` is empty for claude precisely
   so pre-#46 files stay at schema version 1.

10. **`cmd/` stays thin.** Logic in `main` is logic without tests: the
    coverage gate measures `./internal/...` only, which is how a
    hand-copied project lookup in `cmd` drifted from the session service's until
    #122 moved it. Parse flags, construct dependencies, call typed functions.

11. **A hook must never block or fail claude.** Claude runs `omatty hook` on
    every lifecycle event with a 5 s timeout, whether or not omatty is
    running. The hook therefore reads bounded stdin (`maxPayload` in
    `hooks/report.go`), dials the socket with a short timeout, and exits 0
    in every case, writing nothing - a hook that hung or errored would stall
    every claude session on the machine. `main` dispatches to it before
    opening the log or reading config, so nothing that can fail sits in its
    path (#54).

12. **[M9] Gate verdicts come from exit status, never from output text.**
    Invariant 2 applied to the gate, and the same argument: a step's output is
    a rendering — it moves with tool version, `-v`, locale and colour — while
    its exit code is the fact the tool is asserting. So no package greps stdout
    to decide pass or fail. A `kind = "coverage"` step has its percentage
    parsed for display only, and an unparseable one yields zero rather than
    failing a run that the command itself said had passed.

    The corollary that matters in practice is `Missing` vs `Fail`. Steps run
    under `sh -c` — gate lines carry pipes, arguments and script paths — so
    `cmd.Err` never fires the way it would for a direct exec: `sh` is present
    even when the tool is not, and an absent tool comes back as the shell's
    exit 127. That is a convention rather than a guarantee, and a real command
    may exit 127 for its own reasons, so the gate does not read it as one.
    Instead it resolves the step's leading word with `exec.LookPath` before
    running anything. Reporting an uninstalled `golangci-lint` as a failing
    lint step would send a session off to fix code that was never broken.

## The seams

A seam is a package omatty owns that stands between the rest of the code and
something omatty does not control. Each has the same shape: one package
imports the foreign thing, exports a small interface in omatty's own terms,
and the tests substitute a named fake for it. In ADR 0001's terms these are
the infra adapters and the ports they implement.

| Seam | Over | Why it is a seam |
|---|---|---|
| `tui/terminal` | bubbleterm | Pre-1.0. A breaking release touches one file. Golden-frame tests assert the cell grid, not the library. Since #192 it also owns the PTY: the child's bytes pass through a guard that keeps `0x9C` out of OSC and DCS payloads before the emulator parses them, and `Repaint` nudges a re-attached pane's size so claude redraws (#191). |
| `infra/vcs` | the git CLI | go-git cannot do linked worktrees. A `FakeGit` records the commands the code would run. |
| `infra/detach` | the dtach CLI | Optional at runtime; a `Plain` holder makes its absence a footer notice rather than a code path. `dtachprobe` exists because its unit tests assert the command line dtach is *given*, and a missing directory shipped green (#43). |
| `domain/agent` | the coding agent | An agent is a command template plus a status adapter. A second agent is a new profile, not an edit to the launcher, the status service, discovery, `paths` and `cmd` at once (#46). |
| `infra/highlight` | chroma | Not pre-1.0, but the blast-radius rule is the same: one package owns the lexers and the style, and the style is omatty's because every stock theme spends the accent and the diff hues on keywords (#197). |
| `infra/forge` | every forge's CLI | A forge is a backend file in one package, not a package per forge (#452): a `Router` reads each project's origin once, names its forge from the host (`[forge.hosts]` first, an ssh alias through `ssh -G`, #576), and dispatches to that forge's unexported backend. The UI depends on its own func types, never on the Router, and a second forge adds no `os/exec` importer. |

Every seam above is an enforced import rule rather than a convention:
`depguard` in `.golangci.yml` fences bubbleterm and the PTY to
`tui/terminal`, bubbletea to `tui/*`, chroma to `infra/highlight`, go-gitdiff
to `infra/gitdiff`, `os/exec` to the eight packages that shell out and
`net/http` to `infra/forge`. The git seam is the exception depguard cannot
see - git is reached by a string literal, not an import - so
`TestNoGitOutsideVcs` covers it instead (#260); `TestNoGhOutsideForge` does
the same for the forge CLIs.

### The seams as numbers

Robert Martin's package metrics over `./internal/...`, printed by
`./scripts/check-deps.sh` (#263). Ca is how many packages import this one, Ce
how many it imports, and `I = Ce/(Ca+Ce)` — 0 is a stable leaf, 1 is a package
nothing depends on. `cmd` is outside the count, which is what makes the
hexagon visible in it.

Paths are relative to `internal/`; the figures are `./scripts/check-deps.sh`'s
after the migration (#653).

| Package | Ca | Ce | I |
|---|---|---|---|
| `tui/app` | 0 | 15 | 1.00 |
| `cli` | 0 | 6 | 1.00 |
| `service/status` | 0 | 3 | 1.00 |
| every infra adapter but `forge`, `golist`, `highlight`, `notify`, `paths`, `transcript` | 0 | 1–3 | 1.00 |
| `domain/crap` | 0 | 1 | 1.00 |
| `service/gate` | 1 | 3 | 0.75 |
| `domain/tally`, `service/review` | 1 | 2 | 0.67 |
| `service/sessions` | 2 | 3 | 0.60 |
| `infra/forge`, `tui/terminal`, `tui/theme` | 1 | 1 | 0.50 |
| `domain/agent` | 2 | 1 | 0.33 |
| `domain/review` | 5 | 1 | 0.17 |
| `domain/session` | 9 | 1 | 0.10 |
| `domain/status`, `domain/gate`, `domain/forge`, `domain/coverage` | 7, 5, 4, 4 | 0 | 0.00 |
| `pubsub`, `infra/paths`, `domain/fuzzy`, `service/discovery`, `domain/paste`, `tui/keys` | 1–3 | 0 | 0.00 |
| `infra/golist`, `infra/highlight`, `infra/notify`, `infra/transcript`, `domain/depgraph` | 0 | 0 | 0.00 |

The shape is the one ADR 0001 drew. The driving side - `tui/app`, `cli` - sits
at I=1.00, and the domain at the stable bottom. The infra adapters are leaves
that nothing in `internal/` imports: only `cmd` plugs them into ports, so
none of them pins anything. The Stable Dependencies Principle holds with
**0 violations over 55 edges**, the tightest being
`domain/session → domain/gate` at **+0.100**. The migration had to keep it
that way at every step: the import of `service/discovery` by `cli` would have
broken it (−0.100 on `discovery → service/status`) until discovery took its
prompt parser as a port (step 7.2a).

`internal/domain/tally` (#332) is a leaf nothing but the CLI depends on, importing
`domain/session` and `domain/forge` to turn a project's counters and its pull
requests into two numbers. It is its own package for a reason worth recording:
in the session service it would have pulled forge into it and taken its
instability up, tightening every edge into it; in `domain/gate` it would have
given a deliberate stable leaf its first outward import.

**That is a gate, not an observation** (#269). It landed report-only on purpose:
`I` is a ratio of small integers and moves in jumps — a package at Ca=1 Ce=1
goes from 0.50 to 0.33 with one new importer — so a gate failing on a margin
nobody had watched move would be one people learn to `--no-verify` past. The
margin was watched instead, and across every merge from #263 to #278 the
tightest edge stayed `watcher → registry` at exactly +0.071, through a change
that took the graph from 35 edges to 36. A violation now fails
`./scripts/check-deps.sh`, which CI runs before the test suite. The layer
check followed the same path, report-only from #620 to the end of the
migration, enforced since (#653).

**Distance from the main sequence is deliberately not measured here.** Martin
pairs instability with abstractness and calls a stable, concrete package the
"Zone of Pain". By that reading most of the domain scores the maximum
distance — and it is the part this architecture is proudest of. The reason is
that Go declares interfaces at the *consumer*, and usually unexported:
`status.Adapter` lives in `domain/status` precisely so `agent` can satisfy it, which
is the paragraph below. `internal/infra/paths` is Ce=0, pure, and has no
exported interface because nothing needs one. Gating on distance would demand
exactly the speculative interfaces AGENTS.md bans. Do not "fix" these numbers.

**The adapter interface lives beside the vocabulary.** `status.Adapter` is
declared in `internal/domain/status`, not in `agent`, and `agent` imports it to
satisfy it - never the reverse. `domain/status` is the neutral vocabulary
(`Kind`, `Event`, `SessionState`, `Entry`); the profile knows what claude's
JSONL looks like. The dependency runs one way, which is what keeps a second agent
from needing a change to the package that reads every agent's status. It was
declared in the watcher, its consumer, until migration Amendment 7 (#653)
moved it down: `domain/agent` profiles carry an `Adapter`, and domain may not
import the service that reads status.

## Read next

- `docs/ROADMAP.md` - each milestone's "things worth remembering" is the bug
  behind a line of code here.
- `docs/superpowers/specs/2026-09-01-omatty-design.md` - the design this
  repository implements, and the two concerns it flagged before building.
- The package docs - every exported identifier carries its intent and an
  example; `internal/domain/agent` and `internal/infra/paths` are the two to read first.
