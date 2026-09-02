# omatty

A terminal ADE: multiple projects and multiple parallel Claude Code sessions
in one window.

Every other tool in this space is either a desktop app or scoped to a single
repository. omatty is terminal-native — it works over SSH on a headless box —
and shows sessions from *several* repositories side by side.

## Status

**M1: the skeleton.** Projects and sessions are registered, worktrees are
created, and the real `claude` binary runs inside an embedded terminal pane.
Session status, diff review with inline comments, and the file tree are M2–M4.

## Install

```bash
go install ./cmd/omatty
```

Requires Go 1.26, `git`, and `claude` on your PATH, with `$(go env GOPATH)/bin`
on your PATH too.

Install rather than `go build -o omatty`: a binary left in the working tree
goes stale the moment you rebuild anywhere else, and `./omatty` will happily
run last week's code while the tests pass on this week's.

## Use

```bash
omatty add ~/Projects/my-app          # register a repository
omatty new my-app main                # a session on the main checkout
omatty new my-app parser-fix parser-fix   # a session on a fresh worktree
omatty                                # run the TUI
```

Inside the TUI every keystroke goes to Claude except the `ctrl+o` leader:

| Key | Action |
|---|---|
| `ctrl+o j` / `ctrl+o k` | move between sessions |
| `ctrl+o n` | new session on the main checkout |
| `ctrl+o N` | new session on a fresh worktree |
| `ctrl+o q` | quit |

`esc`, `shift+tab`, `ctrl+r` and `ctrl+c` all reach Claude untouched.

## Session status

Each sidebar row shows what its session is doing and how long it has been in
that state, and the focused session's header shows its cumulative token usage.

| Glyph | Meaning |
|---|---|
| `-` | idle |
| `*` | thinking |
| `@` | running a tool |
| `!` | **waiting on you** (a permission prompt) |
| `+` | turn finished |
| `∅` | claude exited (`ctrl+o r` restarts it) |

When a session starts waiting on you or finishes a turn while omatty is in the
background, you get a desktop notification. On macOS you may need to allow
notifications from your terminal app in System Settings once.

Status comes from Claude Code hooks (written to `~/.omatty/hooks.json`, never
your own `~/.claude/settings.json`) and from each session's transcript. If the
hook socket cannot be created, omatty still shows status from the transcript.

State lives in `~/.omatty/state.json`, worktrees in `~/.omatty/wt/`, logs in
`~/.omatty/logs/`. Your `~/.claude/settings.json` is never read or written.

## Contributing

Read [`AGENTS.md`](AGENTS.md). It is the canonical instruction file for both
people and coding agents.
