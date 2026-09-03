# omatty

A terminal ADE: multiple projects and multiple parallel Claude Code sessions
in one window.

Every other tool in this space is either a desktop app or scoped to a single
repository. omatty is terminal-native — it works over SSH on a headless box —
and shows sessions from *several* repositories side by side.

## Status

**M3: review.** Projects and sessions are registered, worktrees are created,
and the real `claude` binary runs inside an embedded terminal pane (M1). The
sidebar shows live per-session status (M2). A diff pane reviews what a session
changed and sends your comments back as one message (M3). The file tree is
next.

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
| `ctrl+o d` | open or close the diff pane |
| `ctrl+o f` | open or close the file tree |
| `ctrl+o r` | restart a crashed session |
| `ctrl+o q` | quit |

`esc`, `shift+tab`, `ctrl+r` and `ctrl+c` all reach Claude untouched.

## Review

`ctrl+o d` opens a diff of everything the session changed: its commits since
it branched, its uncommitted edits, and the files it created, all in one view.
The pane takes the keys while it is open.

| Key | Action |
|---|---|
| `j` / `k` | move through the diff |
| `c` | comment on the line under the cursor |
| `d` | delete the comment under the cursor |
| `r` | reload the diff |
| `S` | send every comment to Claude as one message |
| `esc` | give the keys back to Claude, leaving the pane open |

`esc` and `ctrl+o d` are a round trip: `esc` hands the keys back to Claude with
the pane still on screen, and `ctrl+o d` takes them back. Only a press while the
pane already has the keys closes it.

Comments are anchored to the *content* of a line, not its number, so they stay
put while Claude edits the file underneath you. A comment whose line disappears
floats to the top of its file marked `(moved)` rather than silently attaching
itself to the wrong code.

`S` sends the whole batch as a single prompt — `file:line`, the quoted line and
your note, numbered — so Claude answers them together instead of one at a time.
Pending comments live in memory: quitting omatty drops them.

## File tree

`ctrl+o f` shows the same column as the session's worktree instead of its diff.
A `*` marks every file the session changed, and a directory holding one, so you
can see the shape of a change before reading it. The two views share one column:
`ctrl+o d` and `ctrl+o f` switch between them, and either key closes the column
when it already shows that view.

| Key | Action |
|---|---|
| `j` / `k` | move through the tree, or scroll a preview |
| `enter` | fold or unfold a directory, or preview a file |
| `r` | re-list the worktree |
| `esc` | from a preview back to the tree; from the tree back to Claude |

The listing is tracked plus untracked files with `.gitignore` honoured — what
`git` thinks the worktree contains, not what is on disk. A preview reads at most
256 KiB and says so when it stops; a binary file says it is binary rather than
spraying the pane.

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
