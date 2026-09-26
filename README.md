# omatty

A terminal ADE: multiple projects and multiple parallel Claude Code sessions
in one window.

omatty is terminal-native — it works over SSH on a headless box — and shows
sessions from *several* repositories side by side. Other terminal managers do
the second part; the desktop apps do neither. What none of them do, and what
Claude Code's own `claude agents` does not do either, is run the project's own
check line in each session's worktree and put the verdict on the card.

`docs/comparison.md` is the fair version of that claim, with the places other
tools are ahead.

## What omatty is

There are a great many tools for running coding agents in parallel, and they
almost all optimise the same thing: how much agent-work you can have in flight
at once. Fleets, coordinators, queues, boards.

omatty optimises the other thing: **how quickly you can tell whether what came
back is any good.**

So a project here is not a container for delegated tasks. It is a repository
and the sessions running against it. omatty starts the real `claude` binary —
it does not reimplement Claude's interface — and shows you what each session
changed, with your comments anchored to the *content* of the lines rather than
their numbers, sent back as one message.

A project also carries its **gate**: the `fmt`/`vet`/`lint`/`test`/coverage
line that decides whether the work is sound. omatty runs it in the session's
own directory, shows the result on the session's card, and sends the failures
back into the session that caused them — one keystroke, or on its own when a
turn ends, if you ask for that.

It does not delegate, plan, schedule, or decide on your behalf. You are not a
bottleneck in the loop; you are the part of it that catches things. omatty's job
is to get you to the point of catching them sooner.

`docs/ROADMAP.md` lists what that rules out, and why.
`docs/comparison.md` says how it compares to everything else in the field.

## Status

**v0.5.0**, 2026-09-26 — the forge, read from inside the window. `ctrl+o i`
shows a project's open issues and open pull requests, with the counts on every
project's sidebar header; `enter` reads one in full; `n` turns the one you picked
into a worktree session named and branched after it, and `a` types its reference
into a running session's prompt without sending it. It reads the forge through
your own `gh` and never writes to it — no create, no comment, no close. Install
with `brew install WilsonSousajr/tap/omatty` on macOS or a release archive on
Linux; `CHANGELOG.md` has the whole list.

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
| **M9** The Gate | A project carries the check line that says whether work in it is sound. omatty runs it per session, shows the verdict on the card, and sends the failures back into the session. |
| **M10** Coverage on the diff | Of the lines a session added, the ones no test covers, marked in the diff with a count per file — and a word on the title when a change brought no tests with it. |
| **M11** The Harness | Nothing an operator sees: invariant 4's import boundaries, module hygiene, per-function C.R.A.P. and the package dependency structure become steps of this repository's gate that fail. |
| **M12** The Field | What the rest of the field ships and why omatty declines most of it, researched at code level; then the verification core that came out of it — review scoped to the last turn, a session's pull request and CI on its card, and the files a new worktree needs. |
| **M13** Memory and idle CPU | Idle CPU cut by about 55%, and a per-session leak on archive fixed. |
| **M14** The Tracker | A project's open issues and pull requests in the review column, read through your own `gh` and never written to: counts on every header, one item's body and comments on `enter`, and a worktree session named after the issue you picked. |

Pre-1.0 deliberately: the embedded terminal library underneath is itself
pre-1.0, and the key table, `config.toml` keys and `state.json` schema are
not yet frozen. `docs/ROADMAP.md` has the reasoning and what was cut;
`CHANGELOG.md` has what each release changed.

## Install

On macOS, with Homebrew:

```bash
brew install WilsonSousajr/tap/omatty
```

On Linux, or anywhere without Homebrew, take the archive for your platform
(darwin or linux, amd64 or arm64) from the
[latest release](https://github.com/WilsonSousajr/omatty/releases/latest),
check it against `checksums.txt`, and put `omatty` on your PATH. No Go needed
either way.

From source, with Go 1.26:

```bash
go install github.com/WilsonSousajr/omatty/cmd/omatty@latest
```

Or `@vX.Y.Z` for a given release rather than the tip of `main`; from a clone,
`go install ./cmd/omatty` does the same thing, with `$(go env GOPATH)/bin` on
your PATH.

Every way needs `git` and `claude` on your PATH. `omatty --version` says which
build you ended up with.

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
omatty gate my-app                    # show the gate, or propose one and confirm
omatty gate my-app --detect           # print the proposal, write nothing
omatty carry my-app .env certs        # files every new worktree of it carries
omatty carry my-app                   # show that list
omatty                                # run the TUI
omatty --version                      # which build is this
```

`omatty carry` is what makes a fresh worktree able to run the project. A
worktree is a clean checkout, so nothing git ignores is in it: `.env`, local
certificates, generated config. Name those paths once per project and every
worktree omatty creates afterwards gets a copy, before Claude starts in it —
so a gate step fails because the code is wrong, not because `.env` was
missing. Paths are relative to the main checkout; directories are copied
recursively, modes are preserved, symlinks are copied as links, a file the
worktree already has is left alone, and a path that is not there yet is
skipped with a note rather than treated as an error. The list lives in
`~/.omatty/state.json`, per project, not in the repository — a clone does not
get to choose which of your files are copied.

`omatty discover` reads Claude Code's own transcript store and offers the
repositories you have actually used it in, most recent first — the ones still
on disk, that are still git repositories, with worktrees folded into the
repository they came from. On a well-used machine that is 34 directories in
the store collapsing to 6 worth listing. It only ever proposes: nothing is
registered until you pick it.

`omatty gate` reads the repository and proposes the check line it already
uses — `gofmt`, `go vet`, `golangci-lint`, `go test -race`, a coverage script;
`cargo fmt --check`, `cargo clippy`, `cargo test`; `ruff` and `pytest`; or the
`lint` and `test` scripts a `package.json` defines. Like `discover`, it only
proposes: nothing is written, and nothing is ever run, until you confirm it.

A coverage step also declares the **profile** it writes, and the listing you
confirm says so:

```
  cov   $ ./scripts/check-coverage.sh   [coverage]  -> cover.out
```

That path is read out of the session's own directory when its gate finishes,
and it is what puts the uncovered markers on the diff. Go profiles and lcov are
both understood, told apart by content rather than by file name. A project that
writes its profile somewhere else names it in `~/.omatty/state.json`; a project
that declares none simply gets no overlay.

Inside the TUI every keystroke goes to Claude except the `ctrl+o` leader:

| Key | Action |
|---|---|
| `ctrl+o j` / `ctrl+o k` | move between sessions |
| `ctrl+o ]` / `ctrl+o [` | move between projects, including one with no sessions yet |
| `ctrl+o n` | new session on the main checkout |
| `ctrl+o N` | new session on a fresh worktree |
| `ctrl+o d` | open or close the diff pane |
| `ctrl+o f` | open or close the file tree |
| `ctrl+o g` | open or close the gate pane, and run the gate |
| `ctrl+o i` | open or close this project's issues and pull requests |
| `ctrl+o m` | hand the mouse back to your terminal, or take it back |
| `ctrl+o r` | restart a crashed session |
| `ctrl+o s` | stop the selected session's claude, keeping the session; `enter` resumes it |
| `ctrl+o u` | put the session's worktree back to the start of its last turn, after a confirmation |
| `ctrl+o B` | rename a worktree session's branch |
| `ctrl+o R` | rename the selected session |
| `ctrl+o x` | archive the selected session, or forget an empty project |
| `ctrl+o /` | jump to a session by typing part of its name |
| `ctrl+o a` | register a project claude already knows you use |
| `ctrl+o A` | adopt a claude session already in this project |
| `ctrl+o q` | quit |

`ctrl+o s` ends a session's `claude` process and frees its memory (a few
hundred MB each) without forgetting the session: its card keeps its status and
age, and its queued review comments stay. The pane then says it is stopped, and
`enter` starts it again with `--resume`, so nothing is lost but a turn that was
in flight. Unlike `ctrl+o x` there is no confirmation, because nothing is lost
that `enter` cannot bring back.

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
open and close.

A drag inside a session pane selects that pane's text and copies it on release.
The run is highlighted while the button is down, and it is clipped to the pane:
the sidebar and the review column share those screen rows, and the host
terminal's own selection takes every column on them, so a copy that wrapped a
line used to arrive with their text mixed in (#360). Two limits are worth
knowing. A line Claude wrapped for you arrives with a newline in it, because
the emulator does not record where a soft wrap happened. And only what is on
screen can be selected, because the pane has no scrollback of its own.
`shift`/`opt`+drag still hands the drag to your terminal, which is how you
select *across* omatty's panes rather than inside one.

All of that costs the one thing a terminal normally does with a pointer:
while omatty is asking the host for mouse events, the host will not make a
selection of its own. `ctrl+o m` hands the mouse back, and the header says
`mouse off` while it is handed back. The terminal then selects, copies on
select and opens its context menu exactly as it does everywhere else; the
wheel, the sidebar's clicks and the review column's stop working until you
press `ctrl+o m` again. The keyboard is untouched either way.

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
drag is a scroll rather than a selection. For one selection, hold the modifier
your terminal bypasses reporting with - `shift` on Ghostty, kitty, xterm and
Alacritty, `option` on Apple Terminal and iTerm2 - and drag. For anything
longer, `ctrl+o m` gives the mouse back until you ask for it again. The
selection is the composed screen, so keep it inside one pane. Text that has scrolled out of
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

[gate]
max_parallel = 2           # how many gates may run at once
auto = false               # run a session's gate when its turn ends

[sessions]
lazy_start = true          # at boot, start only sessions dtach still holds; enter starts the rest
idle_stop = "0"            # stop a session quiet this long, keeping it; "0" is off

[ui]
icons = "plain"            # "nerd" draws every state mark with a Nerd Font's icons
```

`ui.icons` is `"plain"` unless you ask: a Nerd Font glyph in a terminal without
one is a box, which is worse than no icon. Set `"nerd"` if your terminal font is
a Nerd Font and every state mark - a gate step's verdict, a card's gate strip, a
pull request's CI - switches to its icon at once. Any other value is refused at
startup, naming the key.

`sessions.lazy_start` is on because every `claude` costs a few hundred MB
before its first turn, and a boot that started all of them paid that for
sessions nobody opened. With it on, omatty reattaches the sessions dtach is
still holding and leaves the rest stopped: their pane says so, and `enter`
starts one with `--resume`. Without dtach nothing survives a quit, so a lazy
boot starts nothing and each session starts the first time you use it. Set it
to `false` to start every session at boot, as before.

Lazy start stops the fleet re-forming; it does not disband one. Sessions dtach
is already holding are reattached, not stopped, so quitting and relaunching
omatty with eleven held sessions still leaves eleven running. They go away on a
reboot, with `ctrl+o s`, or with `sessions.idle_stop`.

`sessions.idle_stop` takes a duration such as `"90m"` or `"72h"` (there is no
`d` unit), and stops a session that has been quiet that long exactly as `ctrl+o s` would: its
process ends, its row, transcript and comments stay, and `enter` resumes it.
Quiet means that its last turn in the transcript, the moment omatty last started
it, and the last key you typed into its pane are all older than the threshold.
A session you are looking at, one in the middle of a turn, and one waiting on
your answer are never stopped. It is off by default, because ending a process
you did not ask to end costs a turn if omatty is wrong about quiet.

`gate.auto` is off because a test suite on every idle costs real time. With it
on, a session that finishes a turn is gated immediately and a red result
notifies you when omatty is not the window you are looking at. Either way, only
a gate you confirmed is ever run.

A project that has no sessions yet is selectable too: `ctrl+o ]` reaches
it, the pane says which project is empty, and `ctrl+o n` creates its first
session there. Archiving a project's last session leaves the cursor on that
project for the same reason.

A session created with `ctrl+o n` and a blank title is named by the first
prompt you type into it. `ctrl+o N` asks for nothing either: the worktree is
created on a placeholder branch named after the session - `omatty-2501d6b4` -
and the same first prompt renames it to `fix-the-horizontal-wheel-pan`, but
only while the branch has nothing committed to it. After the first commit the
name is in a history you may already have pushed, so it stays and `ctrl+o B`
is how you change it. A branch you type at creation is never renamed, and the
worktree's *directory* keeps its original name whatever the branch is called:
Claude is running in it. With `naming.model = true`, a second, headless
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
| `g` / `G` | the first row, or the last - the same on every face of the column |
| `ctrl+d` / `ctrl+u` | half a page down, or up |
| `h` / `l` | pan left and right along a line too wide for the column |
| `0` | jump back to the left edge |
| `c` | comment on the line under the cursor |
| `C` | comment on *part* of the line: type the words, then the note |
| `d` | delete the comment under the cursor |
| `r` | reload the diff |
| `t` | switch between the whole session and only this turn |
| `o` | open the preview of this file with the line under the cursor on top |
| `S` | send every pending comment to Claude as one message |
| `esc` | give the keys back to Claude, leaving the pane open |

`esc` and `ctrl+o d` are a round trip: `esc` hands the keys back to Claude with
the pane still on screen, and `ctrl+o d` takes them back. Only a press while the
pane already has the keys closes it.

Comments are anchored to the *content* of a line, not its number, so they stay
put while Claude edits the file underneath you. A comment whose line disappears
floats to the top of its file marked `(moved)` rather than silently attaching
itself to the wrong code.

You can leave **several comments on one line** - they are all shown under it,
all numbered in the message, and `d` takes the one under the cursor. And `C`
comments on *part* of a line: type or paste the words you mean, then the note,
and the message tells Claude `about: "do(ctx, timeout)"` rather than pointing at
the whole line. The fragment is checked against the line before it is queued, so
a typo is caught while the line is still in front of you. It is stored as text
rather than as a column range, because a range into a line Claude has since
rewritten points at whatever now happens to sit there - and if the line survives
an edit that removes those words, the note still travels and says the fragment is
no longer in it.

`ctrl+o u` puts the worktree back to where that baseline was taken - the start
of the session's last turn. It asks first, and the question names how many files
will be discarded. Files the turn created go with it; a gitignored file that
existed before the turn - the `.env` the session needs to run - is in neither
snapshot and is never touched, and your index, HEAD and stash are not moved
either. It refuses mid-turn, and when no baseline has been taken yet. omatty
does not tell the session what happened: sending anything is `S`'s job and needs
you.

`t` narrows the diff to what changed since you last sent the session a
prompt, and back. omatty takes the baseline when the prompt hook fires, as a
git tree under `refs/omatty/turn/<session>` built through a temporary index,
so your staging, HEAD and stash are never touched; archiving the session
deletes it. Comments work the same in both views: one outside this turn is
hidden rather than shown as moved, and `S` still sends it.

`S` sends the whole batch as a single prompt — `file:line`, the quoted line and
your note, numbered — so Claude answers them together instead of one at a time.
What it sent stays on its line, muted and marked `(sent 14:36)`, so the next
turn's diff can be read against what you asked; it is never sent again, and it
goes once its line does. The title's count is what the next `S` would send.
Comments live in memory: quitting omatty drops them.

## File tree

`ctrl+o f` shows the same column as the session's worktree instead of its diff.
A letter marks every file the session changed - `M` modified, `A` added, `D`
deleted, `R` renamed - in the colour the diff gives that state, and a directory
holding one reads as `M`, so you can see the shape of a change before reading
it. A deleted file keeps its row, in red, until the next listing without it.

Files nobody wrote are **folded out of the listing**: lockfiles, protobuf
output, `build/`, `dist/`, `coverage/`, `vendor/`, `node_modules/`, and anything
the repository's own `.gitattributes` marks `linguist-generated`. A lockfile does
not belong in the queue beside source, and a generated file has no test and never
will - so the coverage markers leave it alone too, rather than making it look
worse than it is. The title says how many are folded (`⊞3`) and `.` brings them
back: they are folded, not hidden. Detection is the repository's declaration
first, then the name, then the Go `// Code generated ... DO NOT EDIT.` header.

`v` marks the file under the cursor as read. It keeps a `✓` and goes quiet
until its diff changes, at which point it reads `~` - changed since you read
it. On a long session that is the difference between reviewing the turn and
reviewing everything again from the top. What is compared is the file's diff
*content*, never its modification time: the agent rewrites files while you are
reading them, so a timestamp only says that something was written. The marks
are one session's, they are not written to disk, and quitting omatty drops
them. The two views share one column:
`ctrl+o d` and `ctrl+o f` switch between them, and either key closes the column
when it already shows that view.

| Key | Action |
|---|---|
| `j` / `k` | move through the tree, or scroll a preview |
| `g` / `G` | the first row, or the last - the same on every face of the column |
| `ctrl+d` / `ctrl+u` | half a page down, or up |
| `enter` | fold or unfold a directory, or preview a file |
| `h` / `l` | pan left and right along a line too wide for the column |
| `0` | jump back to the left edge |
| `r` | re-list the worktree |
| `/` | filter the tree as you type; `enter` keeps the filter, `esc` clears it |
| `a` | attach the row, or the previewed file, to the prompt as `@path` and go back to typing |
| `v` | mark the file read: `✓` while its diff is unchanged, `~` once the session changes it again |
| `.` | show the generated files the tree folded away, or fold them again |
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

## Issues and pull requests

`ctrl+o i` turns the same column into the project's tracker: its open issues,
then its open pull requests under a rule, each row naming the number, one label
or the pull request's CI mark, the title and how long since it last moved. The
counts are in the title, and on every project's header in the sidebar - `13i 2p`
- so "how many are open" needs no keypress at all.

```
 projects · 2              │ omatty ▎ review loop · develop  │ tracker · omatty · 13 issues · 2 prs
───────────────────────────┼─────────────────────────────────┼─ review ──────────────────────────×
 omatty              13i 2p│                                 │#399  feat      filter the tracker
▎● review loop             │                                 │#397  feat      read one issue or p
▎  399-filter-the-t        │                                 │#315            Upstream: 4 MiB par
 other-app             4i 1p                                 │── pull requests ──────────────────
                           │                                 │#405  ✓         work from an issue
```

It is the one view that belongs to a project rather than a session, so it works
on a project you have registered and never started a session in - which is when
its issues matter most.

| Key | Action |
|---|---|
| `j` / `k` | move through the list, or scroll an open item |
| `g` / `G` | the first row, or the last - the same on every face of the column |
| `ctrl+d` / `ctrl+u` | half a page down, or up |
| `enter` | read the item under the cursor: its body and its comments |
| `/` | filter by number, title or label as you type; `enter` keeps it, `esc` clears it |
| `n` | start a worktree session named and branched from the issue |
| `a` | type the item's reference into the selected session's prompt, unsent |
| `b` | open the item in your browser |
| `r` | read both lists again now |
| `h` / `l` / `0` | pan along a row too wide for the column |
| `esc` | from an item back to the list; from the list, lift the filter, then back to Claude |

`n` is the point of showing you issues at all: it creates the worktree, names
the branch after the issue (`399-filter-the-tracker`) and titles the session
`#399 …`, then hands you the keyboard so the next thing you type is a prompt.
`a` is the smaller version for a session you already have: it pastes
`issue #399 ` into the composer **and stops** - no carriage return, so nothing
is sent until you send it. omatty never submits a turn on your behalf.

Everything here is read through your own `gh`, with your own authentication,
and omatty writes nothing to the forge: there is no create, no comment, no
close. The board stays GitHub's; this is a window onto it.

The cost, on top of the two `gh pr list` calls the cards already make: one
`gh issue list` per project every five minutes, one more when you open the
tracker or press `r`, and one `gh issue view` or `gh pr view` for an item you
open, cached until you press `r` on it. Nothing at all while omatty is in the
background, never more than once in thirty seconds for one project, and nothing
after `gh` is found missing. An item is read to 64 KiB and says so if there was
more. Without `gh`, or for a repository that is not on GitHub, the column says
which and the sidebar headers stay as they were.

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

A card's second line names the session's branch and its diffstat. Once that
branch has a pull request on GitHub, the branch becomes the pull request and
one mark for it, read through your own `gh`:

| Line two | Meaning |
|---|---|
| `#349 ✓` | checks passing |
| `#349 ◍` | checks still running |
| `#349 ✗` | a check failed |
| `#349 ⚠` | conflicts, or behind its base |
| `#349` | no checks |
| `#349 merged` / `#349 closed` | the pull request is done |
| `#349 ?` | the last read failed; the old verdict is not shown as current |

omatty reads each project's pull requests with two `gh pr list` calls - the
open ones, then the thirty most recently finished - when omatty regains focus,
when a session finishes a turn, and every minute while it has focus - never
while it is in the background, never more than once in thirty seconds for one
project, and never for longer than thirty seconds. Without `gh`, or for a repository that is not on GitHub, the card shows
the branch as before and says so once in the log. A pull request from a fork is
never taken for the session's, whatever its branch is called; a session on the
main checkout shows open pull requests only, and a merged or closed one shows
only while the checkout is still at its head commit.

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

## License

MIT - see [`LICENSE`](LICENSE).
