# omatty

A terminal ADE: multiple projects and multiple parallel Claude Code sessions
in one window.

Every other tool in this space is either a desktop app or scoped to a single
repository. omatty is terminal-native — it works over SSH on a headless box —
and shows sessions from *several* repositories side by side.

## Status

**v0.1.0 — the first release.** Eight milestones, all built:

| Milestone | Delivers |
|---|---|
| **M1** Skeleton | Projects and sessions registered, worktrees created on demand, the real `claude` binary running inside an embedded terminal pane, and modal key routing. |
| **M2** Status | Live per-session glyphs, age and token usage in the sidebar, from Claude Code hooks and each session's transcript — never scraped from the screen. |
| **M3** Review | A diff pane over everything a session changed, with comments anchored to line *content*, sent back as one message. |
| **M4** Lifecycle | Rename, archive, jump by name, and discover the projects claude already knows you use. |
| **M5** File tree | The session's worktree beside its diff, with change markers and syntax-highlighted previews. |
| **M6** Persistence | With `dtach`, quitting detaches rather than ends; relaunching reattaches. Sessions claude already has can be adopted. |
| **M7** Reach | A config file, mouse support, the agent seam, and a visual identity. |
| **M8** Surface | The frame, colour rule, cards, header, footer and diffstat that the panes are drawn in. |

Pre-1.0 deliberately: the embedded terminal library underneath is itself
pre-1.0, and the key table, `config.toml` keys and `state.json` schema are
not yet frozen. `docs/ROADMAP.md` has the reasoning and what was cut;
`CHANGELOG.md` has what each release changed.

## Install

```bash
go install ./cmd/omatty
```

Requires Go 1.26, `git`, and `claude` on your PATH, with `$(go env GOPATH)/bin`
on your PATH too.

Optionally, `dtach`:

```bash
brew install dtach     # or: apt install dtach
```

With it, quitting omatty *detaches* from your sessions instead of ending them,
and relaunching reattaches to the same running `claude` — a turn in flight
keeps going while omatty is closed. Without it omatty works exactly as before
and says so once at startup; quitting ends each session, and relaunching
resumes the conversation from its transcript rather than the turn.

Install rather than `go build -o omatty`: a binary left in the working tree
goes stale the moment you rebuild anywhere else, and `./omatty` will happily
run last week's code while the tests pass on this week's.

## Use

```bash
omatty discover                       # pick from the repos claude already knows
omatty adopt my-app                   # pick from the claude sessions already in it
omatty add ~/Projects/my-app          # or register one by hand
omatty rm my-app                      # forget a project (the repository stays)
omatty new my-app main                # a session on the main checkout
omatty new my-app parser-fix parser-fix   # a session on a fresh worktree
omatty                                # run the TUI
omatty --version                      # which build is this
```

`omatty discover` reads Claude Code's own transcript store and offers the
repositories you have actually used it in, most recent first — the ones still
on disk, that are still git repositories, with worktrees folded into the
repository they came from. On a well-used machine that is 34 directories in
the store collapsing to 6 worth listing. It only ever proposes: nothing is
registered until you pick it.

Inside the TUI every keystroke goes to Claude except the `ctrl+o` leader:

| Key | Action |
|---|---|
| `ctrl+o j` / `ctrl+o k` | move between sessions |
| `ctrl+o ]` / `ctrl+o [` | move between projects, including one with no sessions yet |
| `ctrl+o n` | new session on the main checkout |
| `ctrl+o N` | new session on a fresh worktree |
| `ctrl+o d` | open or close the diff pane |
| `ctrl+o f` | open or close the file tree |
| `ctrl+o r` | restart a crashed session |
| `ctrl+o R` | rename the selected session |
| `ctrl+o x` | archive the selected session, or forget an empty project |
| `ctrl+o /` | jump to a session by typing part of its name |
| `ctrl+o a` | register a project claude already knows you use |
| `ctrl+o A` | adopt a claude session already in this project |
| `ctrl+o q` | quit |

`ctrl+o R` opens the session's title for editing, pre-filled, so correcting a
typo is a small edit. `enter` confirms, `esc` cancels. The title is
display-only, so a rename never disturbs the session itself.

### Mouse

The wheel scrolls whatever is under it: Claude's transcript in the pane, the
diff, tree or preview in the review column. A click on a sidebar row selects
that session. A click on a row of the review column puts the cursor there and
gives the column the keys, and a click on the `×` at the right end of the
column's rule closes it. The column's rule reads `─ review ─────×` so it is
told apart from a diff Claude draws inside its own pane, which is Claude's to
open and close. Clicks inside the pane go to Claude.

### Paste and copy

A paste goes to the pane that has the keys: Claude's prompt, or the note and
filter lines when one of those is open. It arrives as pasted text, so a
multi-line paste does not submit on every line and nothing is sent until you
press `enter`.

When the program in the pane copies something itself - Claude's own copy
affordances, `tmux`'s `set-clipboard`, neovim's clipboard provider - it lands
on your clipboard. omatty lifts the `OSC 52` the program writes out of the
pane's output and hands it to your terminal, which the embedded emulator
would otherwise have dropped. Nothing is asked first, exactly as in any other
terminal. The read direction is not bridged: a program in a pane cannot ask
for your clipboard's contents.

Taking text off the screen *yourself* is still your terminal's job, and
because omatty asks it for the mouse (for the wheel and for clicks), a plain
drag is a scroll rather than a selection. Hold the modifier your terminal
bypasses reporting with - `shift` on Ghostty, kitty, xterm and Alacritty,
`option` on Apple Terminal and iTerm2 - and drag. The selection is the
composed screen, so keep it inside one pane. Text that has scrolled out of
the pane is in Claude's transcript, not on screen; `pgup` reaches it.

On an empty project's header, `ctrl+o x` forgets the project instead - the
same thing `omatty rm <project>` does from the shell. The repository is never
touched; a project still holding sessions is refused until they are archived.

`ctrl+o x` asks before it does anything. Archiving stops the session and drops
it from the sidebar, but the transcript stays on disk, so nothing is lost that
`claude --resume` could have found. A session on a worktree gets a second
answer, on its own key, that also runs `git worktree remove` — that one
discards uncommitted work, which is why it is never the key your hand reaches
for. A session on the main checkout is never offered it: omatty did not create
that directory, so it will not delete it.

`ctrl+o /` filters every session in every project as you type, matching on the
title and the project name, and `enter` jumps to the one under the cursor. `j`
and `k` are filter text here rather than movement — `ctrl+j` and `ctrl+k` move
— because a switcher you cannot type "jk" into is not a switcher.

`ctrl+o a` is `omatty discover` inside the TUI: the same proposed list, `tab`
to mark several, `enter` to register them all at once.

`ctrl+o A` is the same thing one level down: the claude sessions already in the
project under the cursor that omatty does not yet track, titled by the first
thing you typed in each. Adopting one registers it and opens it in a pane,
resumed where it left off. It is how a session you started in a plain terminal,
or one lost from `state.json`, gets back on the sidebar. `omatty adopt
<project>` does the same from the command line. An adopted session is never
treated as a worktree omatty owns, so archiving it will not offer to delete its
directory.

`esc`, `shift+tab`, `ctrl+r` and `ctrl+c` all reach Claude untouched.

## Configuration

omatty reads `~/.omatty/config.toml` at startup. Every key is optional; with
no file at all these are the values in force:

```toml
leader = "ctrl+o"          # the one key omatty intercepts; bubbletea spelling ("ctrl+a", not "C-a")
claude_bin = "claude"      # the binary each session runs, resolved on PATH or absolute
worktree_root = "~/.omatty/wt"   # where `omatty new ... <branch>` and ctrl+o N put worktrees
base_branch = ""           # fork worktrees from this branch; empty means the checkout's current one

[naming]
model = false              # let a headless claude call improve auto-derived session titles
```

A project that has no sessions yet is selectable too: `ctrl+o ]` reaches
it, the pane says which project is empty, and `ctrl+o n` creates its first
session there. Archiving a project's last session leaves the cursor on that
project for the same reason.

A session created with `ctrl+o n` and a blank title is named by the first
prompt you type into it. With `naming.model = true`, a second, headless
`claude -p --model haiku` call then turns that prompt into a short slug such
as `diff-horizontal-scroll-fix`. It is off by default because it spends your
quota: the call carries the CLI's own system prompt, which cost about $0.60 at
list price when measured. It never blocks, never touches the footer, and a
title you set with `ctrl+o R` always wins.

A malformed file, a wrong type or a key omatty does not know is an error at
startup naming the file and the key, printed before the TUI opens. A blank
leader is refused: with a session focused `ctrl+c` belongs to Claude, so a
leader that never arrives would leave no way to quit.

## Review

`ctrl+o d` opens a diff of everything the session changed: its commits since
it branched, its uncommitted edits, and the files it created, all in one view.
The pane takes the keys while it is open.

| Key | Action |
|---|---|
| `j` / `k` | move through the diff |
| `h` / `l` | pan left and right along a line too wide for the column |
| `0` | jump back to the left edge |
| `c` | comment on the line under the cursor |
| `d` | delete the comment under the cursor |
| `r` | reload the diff |
| `o` | open the preview of this file with the line under the cursor on top |
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
A letter marks every file the session changed - `M` modified, `A` added, `D`
deleted, `R` renamed - in the colour the diff gives that state, and a directory
holding one reads as `M`, so you can see the shape of a change before reading
it. A deleted file keeps its row, in red, until the next listing without it. The two views share one column:
`ctrl+o d` and `ctrl+o f` switch between them, and either key closes the column
when it already shows that view.

| Key | Action |
|---|---|
| `j` / `k` | move through the tree, or scroll a preview |
| `enter` | fold or unfold a directory, or preview a file |
| `h` / `l` | pan left and right along a line too wide for the column |
| `0` | jump back to the left edge |
| `r` | re-list the worktree |
| `/` | filter the tree as you type; `enter` keeps the filter, `esc` clears it |
| `a` | attach the row, or the previewed file, to the prompt as `@path` and go back to typing |
| `o` | from a preview, jump to the diff at that line; from a diff line, `o` opens the preview there |
| `esc` | from a preview back to the tree; from the tree, lift the filter, then back to Claude |

A preview is syntax-highlighted when the file's type is known, in colours
kept apart from the ones the diff uses for added and removed lines, so a
coloured preview never reads as a diff. Files over 64 KiB draw plain and say
so under the last line.

The column is narrow, so a long line runs off its right edge. `h` and `l` scroll
it sideways eight columns at a time and the title shows how far, as `· +24`. The
same keys work in the diff.

The listing is tracked plus untracked files with `.gitignore` honoured — what
`git` thinks the worktree contains, not what is on disk. A preview reads at most
256 KiB and says so when it stops; a binary file says it is binary rather than
spraying the pane.

## Session status

Each sidebar row shows what its session is doing and how long it has been in
that state, and the focused session's header shows its cumulative token usage:
a bar and a percentage for how much of the input claude read back from cache
(`▰▰▰▰▰▰▱▱ 80% cached`), then the in/out counts. Cache reads are what a
long session's prompts mostly are, and the meter says at a glance whether that
is holding.

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
