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

**The process model.** One omatty. One `claude` per session, started by
`internal/supervisor` in the session's directory (the main checkout or a
worktree omatty created) with the session UUID omatty assigned. Where the
`dtach` binary exists, the process runs under a dtach master that outlives
omatty, so quitting detaches instead of killing (#43). `~/.omatty/state.json`
is the registry of projects and sessions and is, by design, enough on its own
to relaunch every session with `--resume <uuid>` (invariant 9).

**Keys in.** A keystroke reaches the bubbletea model in `internal/ui`, which
asks `keys.Router` where it goes. The router is a two-state machine: with a
terminal pane focused, every key is forwarded to that pane's PTY through
`termwrap`, except the leader (`ctrl+o` unless configured), which arms the
next key as an omatty command. With no terminal focused - a modal is open, or
nothing is selected - every key is omatty's. That is the whole routing rule
(invariant 1).

```
keystroke -> keys.Router -> ui.Model -> termwrap.Terminal -> PTY -> claude
                  \-> ui command (n, x, /, d, ...) when armed or unfocused
```

**Status out.** Nothing reads the screen. Two structured sources feed
`internal/watcher`: the transcript JSONL Claude Code writes under
`~/.claude/projects/<slug>/<uuid>.jsonl`, tailed once a second, and hook
events - Claude runs `omatty hook` on each lifecycle event, which forwards the
payload over a unix socket to the watcher's listener. Hooks are fast and are
the only source that can tell "waiting for you" from "tool running"; the
transcript is the truth on attach, self-healing, and the only source of age
and tokens. Both go through the agent's `watcher.Adapter`, which turns one
agent's lines and payloads into omatty's neutral `Event` vocabulary. Events
arrive in the model as `ui.StatusMsg` and become the sidebar glyph, the
activity lane, the token meter and, when omatty is in the background, a
desktop notification.

```
claude --settings ~/.omatty/hooks.json
   |-- writes ~/.claude/projects/<slug>/<uuid>.jsonl --> watcher.Tail  --\
   \-- runs `omatty hook` --> ~/.omatty/sock --> watcher.Listen  --------+--> Event
                                                                         |
                          ui.StatusMsg <-- SessionState <-- watcher.Apply <-/
```

**Review round trip.** The review column asks for a diff through a typed
function `cmd/omatty` hands the model at startup; the function calls
`internal/vcs`, which is the one place git is run. `internal/review` splits
the diff into hunks, holds the operator's comments anchored on content -
file, hunk header, line hash - rather than line numbers, and composes the one
message that sends them back. `ui` writes that message into the session's PTY
as a single bracketed paste followed by one carriage return.

```
ctrl+o d -> ui -> DiffFunc (cmd wiring) -> vcs.CLI (git) -> review.ParseDiff
ctrl+o S -> review.Compose -> "\x1b[200~ ... \x1b[201~\r" -> PTY -> claude
```

**Wiring.** `cmd/omatty` parses flags, builds every dependency - the launcher,
the terminal factory, the registry store, the typed functions that reach git
and the store on `ui`'s behalf - and calls `ui.Run`. It holds no logic
(invariant 10). Subcommands (`add`, `rm`, `new`, `discover`, `adopt`) call
`registry` and `discover` directly and print plain text; the TUI is the only
thing that ever owns stdout.

## Package breakdown

One responsibility per package, typed APIs, no cycles. `ui` is the only
package that imports bubbletea.

| Package | Owns |
|---|---|
| `cmd/omatty` | The binary. Flags, dependency construction, `omatty hook`. Thin by rule. |
| `internal/agent` | What a coding agent is: a command template plus a status adapter. Claude is the only profile (#46). |
| `internal/config` | `~/.omatty/config.toml`. Every key optional; a missing file is every default. The only package that names a TOML library. |
| `internal/detach` | omatty's only route to `dtach`. Returns a no-op holder when the binary is absent. |
| `internal/discover` | Proposes repositories and sessions to register, read from Claude's own transcript store. Proposes only; never writes. |
| `internal/fuzzy` | Subsequence ranking for the session switcher, the pickers and the tree filter. Pure, so it is table-tested. |
| `internal/highlight` | omatty's only route to the syntax highlighter (chroma), with omatty's own colour style (#197). |
| `internal/hooks` | Renders `~/.omatty/hooks.json` and implements the `omatty hook` reporter. |
| `internal/keys` | The modal key router. A pure state machine with no bubbletea dependency. |
| `internal/notify` | Desktop notifications for a session that needs attention while omatty is blurred. |
| `internal/paste` | Bracketed-paste envelopes for text omatty types into a session on the operator's behalf. Invariant 8 lives here because review and gate both need it. |
| `internal/paths` | Every filesystem location omatty reads or writes. Pure; takes `home` explicitly so tests never touch the real one. |
| `internal/registry` | Projects, sessions, `state.json`, and the commands that edit them (add, remove, rename, adopt, create). |
| `internal/review` | Diff → hunks → content-anchored comments → the message sent back. |
| `internal/supervisor` | The `claude` process behind each session: fresh start vs resume, the PTY, the holder. |
| `internal/termwrap` | omatty's only route to the terminal emulator (bubbleterm). |
| `internal/ui` | The bubbletea model: sidebar, panes, modals, review column, rendering. |
| `internal/vcs` | omatty's only route to git, via the CLI. |
| `internal/watcher` | Transcript tailer + hook listener → typed status events, through an `Adapter`. |
| `testdata/` | `fake-claude`, `ptyrun`, `screen`, `dtachprobe`: the harness for the real-PTY smoke test the gate cannot replace. |

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

3. **omatty never writes `~/.claude/settings.json`.** Hooks are injected
   per-process with `--settings ~/.omatty/hooks.json`. Zero footprint is a
   feature: uninstalling omatty leaves Claude exactly as it was, and two
   omatty versions cannot fight over one file.

4. **bubbleterm, git and dtach are reachable only through packages omatty
   owns.** bubbleterm is pre-1.0 and will break; the blast radius must be one
   file (`termwrap/bubbleterm.go`). go-git's linked-worktree support is
   experimental and implements only add and remove, so git is the CLI, and
   only `vcs` runs it. `detach` is the same rule for dtach. See "The seams".

5. **`stdout` belongs to the TUI.** A stray `fmt.Println` lands inside the
   alternate screen and corrupts it. Every diagnostic is structured JSON to a
   file under `~/.omatty/logs/`; `forbidigo` in the lint gate rejects the
   print family outright.

6. **One panicking session must not kill the app.** The tailer goroutine and
   the terminal's read loop each recover (`watcher/guard.go`,
   `termwrap/guard.go`) and mark their own session `✗`. N sessions are the
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
    hand-copied project lookup in `cmd` drifted from the registry's until
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
and the tests substitute a named fake for it.

| Seam | Over | Why it is a seam |
|---|---|---|
| `termwrap` | bubbleterm | Pre-1.0. A breaking release touches one file. Golden-frame tests assert the cell grid, not the library. Since #192 it also owns the PTY: the child's bytes pass through a guard that keeps `0x9C` out of OSC and DCS payloads before the emulator parses them, and `Repaint` nudges a re-attached pane's size so claude redraws (#191). |
| `vcs` | the git CLI | go-git cannot do linked worktrees. A `FakeGit` records the commands the code would run. |
| `detach` | the dtach CLI | Optional at runtime; a `Plain` holder makes its absence a footer notice rather than a code path. `dtachprobe` exists because its unit tests assert the command line dtach is *given*, and a missing directory shipped green (#43). |
| `agent` | the coding agent | An agent is a command template plus a status adapter. A second agent is a new file here, not an edit to `supervisor`, `watcher`, `paths` and `cmd` at once (#46). |
| `highlight` | chroma | Not pre-1.0, but the blast-radius rule is the same: one package owns the lexers and the style, and the style is omatty's because every stock theme spends the accent and the diff hues on keywords (#197). |

**The adapter interface lives in the consumer.** `watcher.Adapter` is declared
in `watcher`, not in `agent`, and `agent` imports `watcher` to satisfy it -
never the reverse. `watcher` stays the neutral vocabulary (`Kind`, `Event`,
`SessionState`); `agent` knows what claude's JSONL looks like. The dependency
runs one way, which is what keeps a second agent from needing a change to the
package that reads every agent's status.

## Read next

- `docs/ROADMAP.md` - each milestone's "things worth remembering" is the bug
  behind a line of code here.
- `docs/superpowers/specs/2026-09-01-omatty-design.md` - the design this
  repository implements, and the two concerns it flagged before building.
- The package docs - every exported identifier carries its intent and an
  example; `internal/agent` and `internal/paths` are the two to read first.
