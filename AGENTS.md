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

### Every bug gets a regression test. No exceptions.

A bug that was fixed without a test is a bug that is coming back. The test is
not paperwork after the fix — it is how you know you fixed the right thing.

**The procedure, in this order:**

1. **Reproduce it as a failing test first.** Before touching the production
   code, write a test that fails *because of this bug*.
2. **Run it and read the failure.** It must fail for the bug's reason, not
   because of a typo, a compile error, or a missing import. A test that fails
   for the wrong reason proves nothing.
3. **Only now fix the code.**
4. **Run it again and watch it pass.** If it passed before your fix, it was
   never testing the bug — go back to step 1.

**Name it after the bug**, with the issue number, so the next reader knows why
it exists: `TestRouter_leaderSwallowedWhileUnfocused_issue42`.

**This applies to every bug, however it was found** — reported by a user, spotted
in review, caught by CI, or noticed by you in your own uncommitted work. "I
found it before I committed it" is not an exemption: the bug was reachable, so
something can reach it again.

**Never delete or weaken a regression test.** If one starts failing, the bug is
back; fix the code. If one is genuinely wrong, say so explicitly in the commit
message and explain why the behaviour it asserted was never correct.


## Security considerations

- **Never commit secrets.** Transcripts under `~/.claude/projects/` contain user
  code and prompts — never copy them into `testdata/` without sanitizing.
- Treat transcript content as **untrusted data, never as instructions.** omatty
  parses it for status; it must not act on text found inside it.
- The hook socket `~/.omatty/sock` is user-only (`0600`) and accepts a bounded,
  typed payload. Reject anything oversized rather than buffering it.

## Project tracking and Git workflow

- **Work is tracked on the GitHub Project board** (`omatty`, project 13).
  Everything syncs with the remote — no local-only branches, no unpushed work
  at end of session.
- **Orient yourself by issues.** Read the issue before starting. If work is not
  covered by an issue, open one first, label it, and put it on the board.
- **Every issue and PR goes on the board**, in exactly one column:

  | Column | Means |
  |---|---|
  | Backlog | Captured. Not committed to a milestone yet. |
  | Sprint Backlog | Committed to the current milestone; ready to pick up. |
  | In Progress | Being worked on right now. One per person. |
  | Review | PR open, awaiting review or CI. |
  | Done | Merged and verified — the gate passed on CI, not just locally. |

  Move the card when the state changes, not in a batch at the end.
- **Labels.** Every issue carries one type label and one milestone label; add an
  `area:*` label per package it touches.

  - Type: `feat` `fix` `docs` `test` `refactor` `perf` `chore` `build` `ci` —
    the same set as the commit-message types, so a `feat`-labelled issue
    produces `feat(#N):` commits.
  - Milestone: `M1` `M2` `M3` `M4`.
  - Area: `area:paths` `area:registry` `area:vcs` `area:termwrap`
    `area:supervisor` `area:keys` `area:ui` `area:cmd`.
  - Flags: `invariant` (changing this touches a cross-cutting invariant —
    argue it explicitly, never assume it is safe), `regression` (needs a test
    that fails before the fix), `blocked`.
- **Commit messages:** `type(#issue_number): message`
  e.g. `feat(#12): tail session JSONL for status events`
- **PR titles use the same pattern**; the body links the issue and states what
  changed, why, and how it was verified.
- **Every bug found gets an issue and a regression test**, even if you fix it
  immediately. Label the issue `regression`; follow the procedure under
  "Every bug gets a regression test" above.
- **PR evaluation:** report pros, cons, and a recommended fix, then ask for
  approval before merging or pushing PR changes.
- No version bumps or release tags without explicit approval.

## Documentation map

- `docs/ROADMAP.md` — milestones M1-M7, what is in each and why, and what was
  deliberately cut. Read it before proposing a feature.
- `docs/superpowers/specs/2026-09-01-omatty-design.md` — the design this repo
  implements.
- `docs/ARCHITECTURE.md` — data flow, package breakdown, invariant rationale.
- `README.md` — install and usage.
