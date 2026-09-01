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
go build -o omatty ./cmd/omatty
```

Requires Go 1.26, `git`, and `claude` on your PATH.

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

State lives in `~/.omatty/state.json`, worktrees in `~/.omatty/wt/`, logs in
`~/.omatty/logs/`. Your `~/.claude/settings.json` is never read or written.

## Contributing

Read [`AGENTS.md`](AGENTS.md). It is the canonical instruction file for both
people and coding agents.
