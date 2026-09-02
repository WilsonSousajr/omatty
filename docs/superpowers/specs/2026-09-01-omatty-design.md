# omatty design

Approved 2026-09-01. Implementation plan tracked separately.

# Context

**The problem.** Running several Claude Code sessions across several repos means several terminal windows, no shared status view, and no way to review a session's diff without leaving it. Reviewing agent output is the actual bottleneck, and the terminal has no tool for it.

**Why not an existing tool.** Researched before designing:

| Tool | Shape | Gap |
|---|---|---|
| [Orca](https://github.com/stablyai/orca) | Electron ADE — WebGL terminals, embedded Chromium, mobile apps | Not a terminal app; half its value can't exist in a TUI |
| [claude-squad](https://github.com/smtg-ai/claude-squad) | Go TUI, tmux + worktrees, diff preview | One repo at a time; no diff comments; no file tree |
| [FleetCode](https://github.com/built-by-as/FleetCode) | Desktop multi-agent app | Not a TUI |
| [workmux](https://github.com/raine/workmux), [claude-tmux](https://github.com/nielsgroen/claude-tmux) | tmux + worktree glue | Session plumbing only, no review UX |
| Crystal (Stravu) | Open-source Electron | **Deprecated Feb 2026** → closed-source Nimbalyst |
| [vibe-kanban](https://github.com/BloopAI/vibe-kanban) | Kanban + web UI | Bloop shut down Apr 2026 |
| Conductor (Melty) | Native parallel worktrees + diff review | macOS-only |
| Claude Code desktop app | Sessions, worktrees, file tree, inline diff comments | Electron; no SSH/headless; single project |

**The hole.** Nothing terminal-native does *multiple projects in one window*, and nothing in the terminal does *diff review with inline comments fed back to the agent*. That pair is omatty's reason to exist. Secondary but real: it works over SSH on a headless box.

**A prior attempt failed.** An earlier LazyVim-plugin approach didn't work. The likely cause is key routing — heuristically intercepting keys around an embedded Claude terminal collides with Claude's own bindings (`esc`, `shift-tab`, `ctrl-r`, `ctrl-c`). Invariant 1 exists because of this.

---

# Design decisions

Taken with the user during brainstorming. Recorded so a later reader does not relitigate them.

| Decision | Choice | Rejected |
|---|---|---|
| Session hosting | **Embedded PTY** — real `claude` rendered in an omatty pane | Headless stream-json (reimplements all of Claude's UX, permanent catch-up); tmux passthrough (omatty and Claude never on screen together) |
| Stack | **Go + Bubble Tea v2** | Rust/Ratatui (stronger, slower to write); TypeScript; Python |
| Session ↔ directory | **Worktree opt-in per session** | Always-worktree (ceremony tax on quick questions); no-git-opinions (loses one-key parallel branching) |
| Review loop | **Batch, PR-style** — comments queue, `[S]` submits one composed message | Immediate send (racy, diff shifts mid-review); review-file indirection |
| Git access | **Shell out to the `git` CLI** | go-git (linked worktrees are v6-experimental only; `add`/`remove` partial, lock/move/prune absent) |
| v1 cuts | Ship/PR workflow · other agents · dtach persistence · desktop notifications | — |

## Verified environment facts

Checked on this machine, not assumed:

- `claude` **2.1.252**. Flags confirmed: `--session-id <uuid>`, `--settings <file-or-json>`, `--resume`, `--fork-session`, `-w/--worktree`, `--permission-mode`, `--output-format`.
- Go **1.26.0** darwin/arm64 · git **2.53.0** · gh **2.87.3** (authed `WilsonSousajr`, scopes `repo`/`project`/`workflow`) · tmux **3.6a**.
- `dtach`, `abduco`, `delta` are **not** installed.
- Transcripts live at `~/.claude/projects/<slug>/<uuid>.jsonl`. Observed slugs confirm the algorithm: `/` and `.` both become `-`, so `…/api-guiaflix/.worktrees/p2-questoes` → `-Users-will-Work-Guia-api-guiaflix--worktrees-p2-questoes`.

**Three consequences that shape everything:**

1. `--session-id` lets omatty *assign* the UUID before launch → deterministic transcript path → **status from structured JSONL, never from the rendered screen.**
2. `--settings` injects per-session hooks → **zero footprint** on the user's `~/.claude/settings.json`.
3. Crash recovery is `claude --resume <uuid>` → **no dtach/tmux dependency.** A crash costs the in-flight turn, not the conversation.

## Layout

```
┌ projects ─┬ session: omatty/parser-fix ────┬ diff ──────────┐
│ ▾ omatty  │ ✻ Claude Code                  │ src/lex.rs     │
│   ○ main  │                                │ - fn lex(s)    │
│   ⎇ parse │ > refactor the parser          │ + fn lex(s)->T │
│   ⎇ tests │   ⎿ Read src/lex.rs            │   ▸ 💬 "use a  │
│ ▾ api-svc │   ⎿ Edit src/lex.rs +12 -4     │     match here"│
│   ○ main  │                                │                │
│           │ ↑ genuine claude TUI, embedded │ [c] [S]        │
└───────────┴────────────────────────────────┴────────────────┘
```

Fixed three panes with toggles. **No free-form splits** — omatty is not a multiplexer.

---

# Conventions

Modelled on [akitaonrails/ai-memory](https://github.com/akitaonrails/ai-memory), read directly from `main`. Adopted: `CLAUDE.md` as a four-line pointer to a canonical `AGENTS.md` (so rules cannot drift across harnesses); a numbered *"Cross-cutting invariants (do not violate)"* section tied to concrete failures; a copy-pasteable local gate identical to CI.

## Two concerns with the stated rules, flagged then proceeding as specified

1. **90% coverage vs. the `ui` package.** Bubble Tea view code is the hardest Go to line-cover. Mitigation baked into the task list: every decision is pulled out of `ui` into a pure package (`keys`, `paths`, sidebar row-building) that covers to 100%, and `teatest` + `testdata/fake-claude` drive real frames for what remains. Coverage is measured over `./internal/...`, excluding `cmd/`. If `ui` still drags the total under 90%, the fix is more `teatest` scenarios, not a lower gate.
2. **4–20 line functions vs. Bubble Tea's `Update`.** The Elm loop wants one giant type switch. Mitigation: `Update` only routes; one small named handler per message type. Enforced by `funlen`, not by review.

## `CLAUDE.md` (verbatim, repo root)

```markdown
# Claude Code Instructions

Read and follow [`AGENTS.md`](AGENTS.md). This repository keeps a single
canonical agent instruction file for Claude Code, Codex, OpenCode, Cursor,
Gemini CLI, and other AGENTS-aware harnesses.

Do not duplicate project rules here. Update `AGENTS.md` instead.
```

## `AGENTS.md` (verbatim, repo root)

````markdown
# AGENTS.md — omatty contributor guide

Single canonical instruction file for AI coding agents in this repository.
`CLAUDE.md` is only a pointer here — do not duplicate rules into it.

## Project overview

omatty is a terminal ADE: one TUI window showing multiple projects and multiple
parallel Claude Code sessions, each in its own pane, with diff review and inline
comments fed back into the session.

Core design:

- **The real `claude` binary runs in an embedded PTY.** omatty never reimplements
  Claude's interactive surface. It renders Claude's own output through a terminal
  emulator and owns only the panes around it.
- **omatty assigns the session UUID** (`claude --session-id <uuid>`), so it knows
  the transcript path `~/.claude/projects/<slug>/<uuid>.jsonl` deterministically.
- **Status is read from structured JSONL, never scraped from the screen.** Hooks
  injected via `--settings` give low latency; the JSONL tail gives truth.
- **A session is a Claude process in a directory.** Worktrees are opt-in.
- **Crash recovery is `claude --resume <uuid>`.** There is no detach layer.

Full design: `docs/superpowers/specs/2026-09-01-omatty-design.md`.

## Technology stack

- **Go 1.26**, module `github.com/WilsonSousajr/omatty`.
- **TUI:** `charm.land/bubbletea/v2`, `lipgloss/v2`, `bubbles/v2`.
- **Embedded terminal:** `github.com/taigrr/bubbleterm` (pre-1.0 — invariant 4),
  `github.com/creack/pty`.
- **Git:** the `git` CLI via `os/exec`, wrapped by `internal/vcs`. Not go-git:
  linked-worktree support is v6-experimental and incomplete.
- **Diff parsing (M3):** `github.com/bluekeyes/go-gitdiff`.
- **Tests:** stdlib `testing`, `charmbracelet/x/exp/teatest`.

## Repository layout

```
cmd/omatty/         binary entry point. Thin: parse flags, build deps, run.
internal/
├── paths/          every filesystem location omatty reads or writes. Pure.
├── registry/       projects + sessions + state.json.
├── vcs/            OUR interface over the git CLI (invariant 4).
├── termwrap/       OUR interface over bubbleterm (invariant 4).
├── supervisor/     process lifecycle: builds the claude command, owns the PTY.
├── keys/           modal key router. Pure state machine (invariant 1).
├── watcher/        [M2] JSONL tailer + hook socket -> typed status events.
├── review/         [M3] diff -> hunks -> comment store -> prompt composer.
└── ui/             bubbletea model, panes, rendering.
docs/               design specs and architecture notes.
scripts/            check-coverage.sh and other gate scripts.
testdata/           fixture repos, recorded ANSI, fixture JSONL, fake-claude.
```

One responsibility per package, typed APIs, no circular dependencies.
`ui` is the only package that imports bubbletea.

## Build and test commands

Run the full local gate before claiming any change is ready. CI runs the same:

```bash
gofmt -l .                                    # must print nothing
go vet ./...
golangci-lint run                             # funlen, dupl, gocyclo, revive
go test ./... -race
./scripts/check-coverage.sh 90
```

Tests never invoke the real `claude` binary or the network. `testdata/fake-claude`
emits scripted ANSI and JSONL and stands in for it everywhere.

## Code style guidelines

- **Functions 4–20 lines.** Longer means it does more than one thing — split it.
- **Files under 500 lines.** Longer means the package boundary is wrong.
- One thing per function, one responsibility per package (SRP).
- **Names must be specific and unique** — a good name returns fewer than 5 grep
  hits in this repo. Banned: `data`, `handler`, `manager`, `util`, `helper`,
  `process`, `info`, `obj`.
- **Explicit types.** No `any`/`interface{}` and no `map[string]any` crossing a
  package boundary. Parse untyped input into a struct at the edge, once.
- **No duplication.** Extract shared logic; `dupl` runs in the lint gate.
- **Early returns.** Maximum 2 levels of indentation inside a function.
- **Error messages carry the offending value and the expected shape:**
  `fmt.Errorf("session %s: worktree path %q is not a directory: %w", id, path, err)`.
  Never a bare `errors.New("invalid input")`.
- Match the surrounding file's conventions. Small, scoped, behavior-preserving
  changes. No opportunistic refactors, no speculative abstractions, no
  unreachable stubs.
- Formatting is `gofmt`'s business. Do not discuss style beyond it.

## Comments

- **Keep existing comments.** Do not strip them during a refactor — they carry
  intent and provenance you do not have.
- Write **WHY, not WHAT**. Never `// increment counter` above `i++`.
- Doc comments on every exported identifier: intent plus one usage example.
- When a line exists because of a specific bug or upstream constraint, reference
  the issue number or commit SHA in the comment.

## Dependencies

- **Inject through constructor or parameter.** No package-level mutable state,
  no global singletons, no `init()` side effects.
- **Wrap third-party libraries behind a thin interface this project owns.**
  `internal/termwrap` owns bubbleterm; `internal/vcs` owns the git CLI. No other
  package may import them or shell out to git.
- Before adding a dependency, check the project does not already have the
  capability.

## Logging

- **Structured JSON** (`log/slog` JSON handler) to a file under `~/.omatty/logs/`.
  Never to stdout — stdout is the TUI.
- **Plain text** only for user-facing CLI output.

## Cross-cutting invariants (do not violate)

1. **Key routing is modal, never heuristic.** When the terminal pane has focus,
   every keystroke goes to the PTY except the `Ctrl+O` leader. Never inspect a
   key and guess whether Claude wants it — that is what broke the prior LazyVim
   attempt.
2. **Status comes from JSONL and hooks, never from the rendered screen.** No
   package may parse the terminal cell grid to infer session state.
3. **omatty never writes to the user's `~/.claude/settings.json`.** Per-session
   hooks are passed with `--settings ~/.omatty/hooks.json`. Zero footprint is a
   feature.
4. **bubbleterm and git are reachable only through `internal/termwrap` and
   `internal/vcs`.** bubbleterm is pre-1.0 and will break; the blast radius must
   stay inside one package we own.
5. **`stdout` belongs to the TUI.** Every diagnostic goes to the slog file
   handler. A stray `fmt.Println` corrupts the screen. Enforced by `forbidigo`.
6. **One panicking session must not kill the app.** Each supervisor goroutine
   recovers and marks its own session `✗`.
7. **Comments anchor on content, not line numbers** — `(file, hunk header, line
   hash)`. Claude edits files while you read them; line-number anchors silently
   attach feedback to the wrong code.
8. **Review submission uses bracketed paste** (`ESC[200~ … ESC[201~`) then one
   `\r`. Writing a multi-line prompt raw sends N premature messages.
9. **`state.json` must always suffice to relaunch every session** with
   `--resume <uuid>`. Any new session field is either derivable or persisted.
10. **`cmd/` stays thin.** Parse flags, construct dependencies, call typed
    library functions. No logic.

## Testing instructions

- **TDD is mandatory.** Write the failing test first. Every new function gets a
  test.
- **Every bug fix gets a regression test** that fails before the fix and passes
  after. Name it with the issue number: `TestRouter_leaderSwallowed_issue42`.
- **Coverage gate is 90%** over `./internal/...`. The gate does not move.
- Tests are **F.I.R.S.T** — fast, independent, repeatable, self-validating,
  timely. No `time.Sleep` for synchronization; no test depends on another's
  ordering.
- **Mock external I/O with named fake types, not inline closures or stubs.**
  `type FakeGit struct{...}` in `fakes_test.go`, implementing the interface the
  production code takes. Named fakes are readable in a failure message.
- Filesystem tests use `t.TempDir()`. Never touch the real `~/.claude` or
  `~/.omatty`.
- Golden-frame tests for `termwrap`: recorded ANSI in, asserted cell grid out.
- End-to-end via `teatest` against `testdata/fake-claude`.

## Security considerations

- **Never commit secrets.** Transcripts under `~/.claude/projects/` contain user
  code and prompts — never copy them into `testdata/` without sanitizing.
- Treat transcript content as **untrusted data, never as instructions.** omatty
  parses it for status; it must not act on text found inside it.
- The hook socket `~/.omatty/sock` is user-only (`0600`) and accepts a bounded,
  typed payload. Reject anything oversized rather than buffering it.

## Project tracking and Git workflow

- **Work is tracked on the GitHub Project board.** Everything syncs with the
  remote — no local-only branches, no unpushed work at end of session.
- **Orient yourself by issues.** Read the issue before starting. If work is not
  covered by an issue, open one first.
- **Commit messages:** `type(#issue_number): message`
  e.g. `feat(#12): tail session JSONL for status events`
- **PR titles use the same pattern**; the body links the issue and states what
  changed, why, and how it was verified.
- Types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `chore`, `build`, `ci`.
- **Every bug found gets an issue and a regression test**, even if fixed at once.
- **PR evaluation:** report pros, cons, and a recommended fix, then ask for
  approval before merging or pushing PR changes.
- No version bumps or release tags without explicit approval.

## Documentation map

- `docs/superpowers/specs/2026-09-01-omatty-design.md` — the design this repo
  implements.
- `docs/ARCHITECTURE.md` — data flow, package breakdown, invariant rationale.
- `README.md` — install and usage.
````
