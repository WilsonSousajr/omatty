# Changelog

Notable changes to omatty. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[semantic versioning](https://semver.org/spec/v2.0.0.html), with the pre-1.0
caveat that the key table, `~/.omatty/config.toml` keys and the `state.json`
schema are not yet frozen.

Each entry names the issues behind it. `docs/ROADMAP.md` carries the reasoning
for each milestone and what was deliberately cut.

## [Unreleased]

## [v0.6.0] — 2026-09-26

M12 finishes what its research asked for, and the review column starts
remembering things. It knows which files you have read and says when one changes
under you; it folds away the files nobody wrote; it takes more than one comment
on a line, and comments on part of one. A session can be put back to the start
of its last turn, and a green one can be shipped — pushed, opened as a pull
request, and merged when both verdicts are already green — from the card you read
it on. M15's first eleven slices come with it.

### Added

- **Per-file reviewed, and changed since reviewed** (#337). `v` in the tree
  marks a file read: it keeps a `✓` and goes quiet until its diff changes, then
  reads `~`. The identity compared is the file's diff *content* — the agent
  rewrites files while you read them, so a timestamp only says something was
  written.
- **Generated files folded out of the review queue** (#338). Lockfiles,
  `coverage/`, protobuf stubs and anything the repository's own `.gitattributes`
  marks `linguist-generated`, detected in that order. `g` brings them back, the
  title says how many are withheld, and the coverage markers leave them alone —
  a generated file has no test and never will.
- **Several comments on one line, and comments on part of a line** (#339). All
  of them are shown, numbered and sent; `C` collects the words the note is about
  and the message says `about: "…"`. The fragment is text, not a column range,
  and it is not part of the anchor — invariant 7 keeps the line's identity to
  itself.
- **Revert a session to the start of its last turn** (#334). `ctrl+o u`, after a
  confirmation naming how many files go. It reuses #311's baseline, so it adds a
  restore and no new state, and it leaves your index, HEAD and stash alone. A
  gitignored file that existed before the turn — the `.env` the session needs —
  is never touched.
- **Ship a green session** (#331). `ctrl+o p` pushes the branch and opens its
  pull request, or merges one whose local *and* remote verdicts are already
  green. One keypress, one session, every time. Never when checks merely *go*
  green, never a force-push, never a branch deletion, and never into a protected
  branch.
- **Lead time and first-pass gate rate** (#332). `omatty gate <project> --stats`:
  session start to pull request merged, and the share of turn-following gate runs
  that passed. Two numbers, local only, nothing leaves the machine.
- **A recipe for an LLM audit as a gate step** (#333). `docs/llm-audit-gate.md`
  and a working `docs/examples/audit.sh`, honest about non-determinism, cost and
  the fact that the diff it reads is untrusted input.
- **M15's first eleven** (#420–#429, #447): one list window for every face of the
  column, one state vocabulary, chrome that names the face it is showing,
  `ctrl+o z` to zoom, a gate face that reads like a CI check page, and `r` to
  re-run it.
- Issue templates and an announcement draft (#415, part of #330).

### Changed

- **A release happens at every milestone close** (#329), written into AGENTS.md
  and the roadmap. A floor, not a ceiling: a release may still happen without
  one. The checks did not move.
- **The working glyph spins** and the activity lane is gone (#410, #412); the
  branch gets the seven columns it cost.
- `internal/forge` is no longer read-only. It reads on a timer and writes only on
  a keypress — the three calls `ctrl+o p` makes, and nothing else.
- **No session URL** in a commit, a pull request, an issue or a comment (#474).

### Fixed

- **The ship key shipped without a green gate** (#471). It guarded on a
  predicate that checks three facts, none of them the gate: a project with no
  gate configured, or one whose gate had failed, was pushed anyway. Found by the
  real-binary smoke test, not by the suite.
- **`git push` prompted for credentials over the TUI** (#472). git asks on the
  controlling terminal, which is the one omatty draws on; it now fails into the
  footer instead.
- The gate view could not scroll or pan (#421, #447), the help modal omitted
  whole faces' keys (#422), and the tracker's age was not at the right edge
  (#423).

### Schema

`state.json` stays at version 1. `Session.Started` and a project's two gate
counters are new and optional, with derivable empty values — the argument
`Base`, `Agent` and `Gate` already make.

## [v0.5.0] — 2026-09-26

The forge, read from inside the window. omatty could already tell you that this
session's branch had pull request #349 and that its checks were green; it could
not tell you how many issues the project had, what they were about, or which one
to start next. That was the last daily-friction item that needed no new
subsystem, and M14 is it.

`ctrl+o i` turns the review column into the project's tracker: its open issues,
then its open pull requests under a rule, with the counts on every project's
sidebar header so "how many are open" needs no keypress at all. `enter` reads one
in full. `n` turns the one you picked into a worktree session named and branched
after it.

**It reads the forge and never writes to it.** No create, no comment, no close,
no label. `docs/ROADMAP.md` refuses "a planning board inside the TUI" for a
stated reason - two sources of truth - and this is not one: nothing is stored, no
`state.json` field is added, and the rows are gone when omatty exits. It is a
window onto GitHub's board, which is the distinction #310 already won for pull
requests. The keys that act - `n`, `a`, `b` - act on this machine: a worktree, a
composer, a browser.

A minor bump, per the pre-1.0 rule: a new leader key, two new column views, three
new keys inside them and a third `gh` call. **`state.json` does not change at
all**, so there is no migration and `Version` stays 1. Nothing was renamed or
removed.

### Added

- **`ctrl+o i` - a project's open issues and open pull requests.** A row names
  the number, one label or the pull request's CI mark, the title and how long
  since it last moved. It is the one view that belongs to a *project* rather than
  a session, so it opens on a project you have registered and never started a
  session in - which is when its issues matter most - and it keeps its list and
  its cursor as you move between two sessions of one project. `j`/`k` walk,
  `h`/`l`/`0` pan, `r` reads both lists again. (#396)
- **Counts on every project's sidebar header** - `13i 2p`, right-aligned and
  muted. Unknown is never drawn as zero: without `gh`, for a repository that is
  not on GitHub, or before the first poll answers, the header is exactly what it
  was before. A project holding no session is never asked for pull requests, so
  it shows its issue count and says nothing about the other. (#395)
- **`enter` reads one issue or pull request in full** - its body and its
  comments, wrapped to the column and scrolled with `j`/`k`. One `gh` call on the
  keypress, cached until `r` asks again, bounded at 64 KiB with whole comments
  dropped rather than cut, and it says when there was more. (#397)
- **`n` starts a worktree session on the issue under the cursor**, named `#399 …`
  and branched `399-filter-the-tracker`, then hands you the keyboard so the next
  thing you type is a prompt. On a pull request row it refuses and says why: that
  branch already exists. (#398)
- **`a` types the item's reference into the selected session's composer** -
  `issue #399 ` - bracketed and **with no carriage return**, so nothing is sent
  until you send it. omatty never submits a turn on your behalf. `b` opens the
  item in your browser through your own `gh`. (#398)
- **`/` filters the list as you type**, over the number, the title and every
  label - including the ones the row has no room for, because `M14` is how a
  milestone is asked for. `enter` keeps the query, `esc` lifts it, and the marker
  never leaves the title: a filtered list is short *because* a filter is in
  force. (#399)
- **`internal/forge` reads issues as well as pull requests**: `ListIssues` is one
  `gh issue list --state open` call per project, and an open pull request now
  carries its title, whether it is a draft, and an age. Polled every five minutes
  where the pull requests are polled every one - CI changes in minutes, an issue
  list changes in days - and never while omatty is in the background, never more
  than once in thirty seconds for one project, and not at all once `gh` has been
  found missing. (#393, #394)

### Changed

- The review column has five faces rather than three. `ctrl+o d`, `ctrl+o f`,
  `ctrl+o g` and `ctrl+o i` switch between them and each closes the column when it
  already shows its own view. (#396)
- `docs/ARCHITECTURE.md` gains the tracker's data flow and `internal/forge`'s row
  in the package table, which #310 never added. (#399)

### Fixed

- The rule between the two lists panned away with the rows, leaving the issues
  and the pull requests merged with nothing to say where one stopped. Found by
  the real-PTY smoke run, not by a test. (#396)
- An open item's footer was the file tree's, offering `o diff` in a view with no
  diff. (#397)

### Known limitations

- `brew install` prints `Calling postflight is deprecated`. The install works;
  the warning comes from GoReleaser's generated cask and is blocked on
  goreleaser#6873. (#369)

## [v0.4.0] — 2026-09-25

A worktree you can actually run, a pane you can copy out of, and a hook that
survives being reinstalled. M12's close-out: the P1/P2 work it had cut, taken
back (#379), plus the defects v0.3.0's own release surfaced.

The headline is #309. A worktree is a clean checkout, so nothing git ignores is
in it - `.env`, local certificates, generated config - and a gate step that
fails for that reason is the gate being wrong about the code. `omatty carry`
names those paths once per project and every worktree created afterwards gets
them, before claude starts in it.

A minor bump, per the pre-1.0 rule: `state.json` gains one optional key
(`Project.Carry`) whose nil value means "carry nothing", so no migration and
`Version` stays 1. A new subcommand, `omatty carry`, and a new gesture inside a
pane. Nothing existing was renamed or removed.

### Added

- **`omatty carry <project> <path>...` - the files a new worktree needs.**
  Paths are relative to the main checkout; directories are copied recursively,
  modes are preserved so an executable still runs, and a symlink is copied as a
  link rather than followed. A file the worktree already has is left alone,
  because git checked that one out. A listed path that is not there yet is
  skipped with a note instead of failing the session, and a carry that does
  fail takes the worktree with it. The list lives in `state.json`, per project,
  not in the repository - a clone does not get to choose which of your files
  are copied. The cost of that choice, stated rather than hidden: the list is
  not shared with a team. (#309)
- **A drag inside a session pane copies that pane.** The run is highlighted
  while the button is held and clipped to the pane, and `shift`/`option`+drag
  still hands the selection to your terminal for selecting across omatty's own
  panes. (#360)

### Fixed

- **Selecting text across a line break copied the sidebar and file tree with
  it.** The selection was the host terminal's, and the host sees one grid of
  columns and rows - omatty draws the sidebar, the pane and the review column
  on the same rows, so anything that wrapped took all three. Only a
  single-line selection was ever clean. omatty now answers the drag itself.
  (#360)
- **Reinstalling omatty broke every session that was already running.** claude
  reads its hooks file once, at startup, so a session holds the path of the
  binary that launched it for as long as it lives; moving that binary - `go
  install` to Homebrew, a `brew upgrade` to a new Cellar path, `go clean` -
  made every hook event fail with `No such file or directory`, on every tool
  call. An invariant 11 break: a hook must never fail claude. (#380)
- **The gate scored a stale coverage profile**, reporting a covered function at
  0% and a C.R.A.P. of 30. Adding a test changes coverage without changing any
  source file, and the staleness guard only compared the profile against
  source. A confident wrong number that reads exactly like a real finding.
  (#385)
- `S` during a gate run sent the *previous* run's failures back into the
  session. It now says the run is in flight and sends nothing. (#345)
- A turn snapshot finishing after its session was archived recreated the ref
  the archive had just removed. (#350)
- Turn-view failure notices cut off the cause; they now show what git said,
  wrapped. (#351)
- An out-of-order turn load could paint a two-turn diff over a newer one. Loads
  are numbered and a late answer is dropped. (#352)
- A stalled `gh` left the previous pull-request verdict on the card instead of
  reporting that the read failed. (#356)
- A four-digit pull request number hid the card's diffstat. (#357)
- An open pull request older than a repository's newest hundred dropped off its
  card. (#358)
- A release's notes reached the GitHub release again; GoReleaser's changelog
  pipe had been switched off. (#367)
- `docs/comparison.md` listed three capabilities as missing that v0.3.0 had
  shipped - #310's pull-request state, #311's per-turn review scope and #335's
  sent comments. A page that undersells is as wrong as one that oversells.
  (#384)

### Changed

- **Acting on a pull request is decided, and written down.** omatty may push a
  branch, open its pull request, and merge one whose local *and* remote
  verdicts are already green - one keypress, one session, after a person read
  the verdict. Merging when checks *go* green is refused by name, beside the
  CI-triggered agent runs it resembles: it acts because a check changed, with
  nobody reading. A protected branch is never a merge target, so a ship key
  cannot route around omatty's own release gate. The build is #331; this is the
  boundary it asked for. (#331, #379)
- `docs/ROADMAP.md` sources M9's claim about the size of the field, and records
  what M12 took back. (#340, #379)
- The #311 design note matches the code it describes. (#353, #354)
- `#310`'s follow-ups: pull requests default on, invariant 4 names `gh`, and a
  path guard. (#359)

### Known limitations

- `brew install` prints `Calling postflight is deprecated`. The install works
  and the binary runs; GoReleaser cannot yet emit Homebrew's replacement
  stanza, and the fix is blocked on goreleaser#6873. (#369)
- The agent seam still has one profile, claude. Codex is a follow-up. (#152)
- Scrollback is not preserved across a detach and reattach. (#336)
- A wrapped line copied out of a pane arrives with a newline in it: the
  emulator does not record where a soft wrap happened. Only what is on screen
  can be selected. (#360)
- Homebrew installs the cask on macOS; on Linux, use the release archive.

## [v0.3.0] — 2026-09-25

M12's verification core - review scoped to the last turn, a session's pull
request on its card, sent comments that stay put - and the first release the
pipeline builds: binaries for macOS and Linux, checksums and a Homebrew cask,
so installing omatty no longer needs Go (#364).

A minor bump, per the pre-1.0 rule: the review column gains the `t` key and the
card gains the pull request; `state.json` and `config.toml` are unchanged.

### Added

- **omatty is MIT-licensed** (`LICENSE`), and the release archives carry the
  license beside the README and changelog. (#362)
- **A tag is a release.** Pushing a `v*` tag re-runs the whole gate and then
  publishes, through GoReleaser: binaries for macOS and Linux on amd64 and
  arm64, `checksums.txt`, a Homebrew cask (`brew install
  WilsonSousajr/tap/omatty`), and the GitHub release with the tag's own
  changelog section as its notes. Installing no longer needs Go. Every pull
  request checks the release configuration and builds it as a snapshot. (#327)
- **A session's pull request on its card.** Once a session's branch has a pull
  request on GitHub, line two names it with one mark for its CI - `#349 ✓`,
  `◍` running, `✗` failing, `⚠` conflict or behind, `merged`, `closed`, and `?`
  when the last read failed rather than the old verdict. One `gh pr list` per
  project, on focus, at the end of a turn and every minute while focused, at
  most once in thirty seconds and never in the background; `internal/forge` is
  the one package that runs `gh`. A fork's pull request is never matched, and a
  finished one only while the checkout is at its head commit.
  Without `gh`, or off GitHub, the card is as before. (#310)
- **`t` in the review column shows only this turn.** When a prompt is
  submitted, omatty snapshots the session's working tree as a git tree under
  `refs/omatty/turn/<session>`, built through a temporary index, so staging,
  HEAD and the stash are never touched; `t` diffs the tree now against it.
  Only the prompt hook takes the baseline, never the transcript tailer, which
  also reports tool results. A failed snapshot is shown instead of a diff that
  would span two turns; archiving deletes the ref. (#311)
- **A sent review comment stays on its line**, muted and marked
  `(sent 14:36)`, is never sent again, and goes once its line does; the gate
  pane's `S` asks before resending the same failures. (#335)

### Fixed

- The gate pane's rows keep one command column, pending or with a verdict,
  whatever the length of the step names. (#342)

### Known limitations

- The agent seam still has one profile, claude. Codex is a follow-up. (#152)
- Scrollback is not preserved across a detach and reattach. (#336)
- Homebrew installs the cask on macOS; on Linux, use the release archive.
- A card reads the newest 100 pull requests of its repository, so an open one
  older than that shows the branch. (#358)

## [v0.2.0] — 2026-09-22

M9, M10, M11 and M13, the session lifecycle, and M12's research, built on
`develop` between 2026-09-10 and 2026-09-22 and promoted together (#328). The
headline is the gate: v0.1.0 did not have it.

A minor bump, per the pre-1.0 rule: `state.json` gains optional fields
(`Gate`, `Conversation`) whose empty value is derivable, and `config.toml`
gains the `[gate]` and `[sessions]` tables. Nothing existing was renamed or
removed.

### Added

- **M9 — The Gate.** A project is now a repository, the gate that says whether
  work in it is sound, and the sessions running against it. omatty runs the
  project's own check line in a session's directory, shows the verdict on the
  session's card, and sends the failures back into the session that caused
  them. (#222–#234)
  - `omatty gate <project>` shows, proposes, sets or clears a gate. Detection
    reads the checkout — Go, Cargo, Node, Python — and only ever *proposes*;
    nothing runs a gate you have not confirmed. (#226, #228)
  - `ctrl+o g` opens the gate in the review column and runs it. `enter` folds a
    step's output open, `S` sends the failures into the session as one
    bracketed paste. (#231, #232)
  - A session card's third line carries the gate: one mark per step in order,
    the failing step named, coverage, and `READY` when the gate is green, there
    is a diff to show for it and the session is at rest. (#230, #225)
  - `[gate] auto` runs a session's gate when its turn ends, off by default. A
    red gate notifies only while omatty is blurred. `[gate] max_parallel`
    bounds how many run at once, default 2. (#233, #229)
- **Invariant 12** — gate verdicts come from exit status, never from output
  text. A `kind = "coverage"` step's percentage is parsed for display only, and
  a tool that is not installed reports `Missing` rather than a failure. (#222)
- **M10 — Coverage on the diff.** Of the lines a session just added, which are
  not exercised by anything? omatty reads the profile the project's own gate
  wrote and answers it in the review column. (#251–#258)
  - A coverage step **declares the profile it writes** —
    `cov $ ./scripts/check-coverage.sh [coverage] -> cover.out`. `omatty gate`
    proposes the ecosystem's convention and shows it in the listing you confirm;
    `state.json` stays at version 1, since an absent profile means "no overlay".
    (#253)
  - **Added lines no test covers are marked** in the diff, with the count on the
    file header — `internal/ui/model.go +2 -1  3 uncovered`. A line the profile
    does not mention is not a statement and gets no marker. (#251, #252, #255)
  - The profile is read **when the session's gate finishes**, out of that
    session's own directory, so two sessions never read each other's numbers.
    One that will not parse leaves the previous overlay rather than blanking it.
    (#254)
  - **`⚠ no tests`** on the diff title when a change touched source and no
    tests. Rust's in-file `#[cfg(test)]` counts as tests, so a Rust change that
    tested itself does not raise it. A remark, not a gate: nothing is blocked.
    (#256, #257)
  - A review column too narrow for the whole title now **gives up whole parts**
    in a stated order — a zero comment count, then the file count — rather than
    being cut from the right, which took the flag out at the default window
    size. `diff · 2 files · ⚠ no tests`, then `diff · ⚠ no tests`. (#283)
  - The tree title fits itself the same way, by its own rule: the **filter
    marker never goes**, because a listing is short *because* a filter is in
    force and a cut marker leaves a filtered tree looking complete. The session
    name shortens — `files · a-l…name /internal` — and past the point where a
    name identifies anything, goes. (#285)
  - The preview title shortens **from the front**, since a path's filename is
    what says which file is on screen and its directories are context:
    `…/lifecycle/restart.go`. The `…/` is part of the answer — `lifecycle/…`
    alone would read as a complete repo-relative path. (#287)
  - The **diff's file header fits itself too**, by a fourth rule, because its
    parts rank differently again: the `N uncovered` count is never given up —
    it is the finding, and it was the first thing a narrow column cut — the
    path shortens from the front as the preview title does, and `+13 -0` goes
    last, only when keeping it would cost the filename. At the default window
    `internal/paths/scratch.go +13 -0  5 uncovered` reads `scratch.go  5
    uncovered`. A rename shortens both names and a binary keeps its
    `(binary)`. The header no longer pans with the body, since it now fits
    where it is. (#291)

- **`ctrl+o N` stops asking for a branch name.** It was the last prompt that
  demanded a string before anything could start, because `git worktree add -b`
  runs at creation and bakes the name into a directory and into `state.json`.
  The worktree is now created on a placeholder named after the session —
  `omatty-2501d6b4` — and the session's first prompt renames it, the way that
  prompt already names the session: `fix-the-horizontal-wheel-pan`. Only while
  the branch has nothing committed to it; after the first commit the name is in
  a history you may have pushed, and `ctrl+o B` renames it by hand. The
  worktree's *directory* keeps its original name whichever way — Claude is
  running in it, and its transcript path is derived from it. A branch you do
  type is now passed through the same `registry.Slug` filter model output has
  always had, which it never was. (#151, #127)

- **`ctrl+o m` hands the mouse back to your terminal.** #107 asked the host for
  mouse reporting and never added a way to stop asking, so `?1002h` was held
  from a session's first frame to its last and the terminal could never make a
  selection of its own — a plain drag scrolled instead. Released, the host owns
  the pointer again: native selection, copy-on-select and context menu, no
  crosshair; the wheel, the sidebar's clicks and the review column's stop until
  you press it again. The header says `mouse off` while it is released, in the
  one piece of chrome that is drawn whole at every width. The keyboard is
  untouched either way, and the modifier-drag stays the quick answer for a
  single selection. (#217)

- **M11 — The Harness.** The gate stops trusting prose. Three rules this
  repository had written down and nothing checked are now steps that fail, and
  the package structure underneath them is a number printed on every run.
  Nothing here changes what omatty does for an operator; it changes what can
  reach `develop`. (#260–#263)
  - **depguard** enforces invariant 4's import boundaries — bubbleterm and
    `creack/pty` to `internal/termwrap`, bubbletea to `ui` and `termwrap`,
    chroma to `internal/highlight`, go-gitdiff to `internal/review`, `os/exec`
    to the six packages that run one. Measuring the real importers is what
    found `AGENTS.md`'s claim that `ui` was the only bubbletea importer to be
    false. (#260)
  - **`go mod tidy -diff` and `govulncheck`**, the latter pinned by
    `GOVULN_VERSION`. The first steps of the gate that need the network. (#261)
  - **A C.R.A.P. gate**, scored per function rather than per repository, because
    a repo-wide coverage average is where an exported function at 0% hides. It
    shipped at 15 — the lowest value green at the time, held there by the two
    untested functions the gate had just found — and ratcheted to **12** once
    they were tested, with the worst score in the tree then 8.2. (#262, #267)
  - **`scripts/check-deps.sh`** reports afferent and efferent coupling,
    instability, abstractness and distance per package, and fails on an import
    cycle through the *test* graph — which the compiler permits and nothing
    else looks at — and, since #269, on an import that runs against the
    direction of stability. The Stable Dependencies Principle landed behind
    `--sdp` on purpose and was enforced once the margin had been watched across
    eight merges without moving. (#263, #269)

- **`ctrl+o s` stops a session without forgetting it.** The `claude` process
  ends and frees its memory; the row, transcript, queued review comments and
  card stay. The pane says the session is stopped, and `enter` resumes it
  with `--resume` on its current conversation. Before this the only way to
  free one was `ctrl+o x`, which archives it. (#318)
- **`[sessions] idle_stop`** stops a session that has been quiet this long —
  `"90m"`, `"72h"` — exactly as `ctrl+o s` would. Off by default (`"0"`): ending
  a process you did not ask to end costs a turn if omatty is wrong about
  "quiet". The selected session and one that is thinking, running a tool or
  waiting on you are never stopped. (#319)
- **M12 — The Field.** `docs/research/` surveys the tools omatty ships among —
  a landscape, four code-level deep dives, four issue-tracker minings, a
  prior-art ledger and a self-critical parity audit — and `docs/comparison.md`
  is the public version. The method is captured as
  `.claude/skills/market-research/`. (#295–#302)

### Fixed

- Cancelling a gate step killed only the `sh` it ran under, leaving a
  grandchild holding the output pipe; a cancelled run took its full duration
  and left the work running. The whole process group is killed now. (#238)
- `revealHeader` could push the selected card out of a short sidebar while
  making room for its project header. Latent since #129, reachable in v0.1.0 by
  making the terminal short enough. (#244)
- A shell builtin with no binary — `exit`, `return`, `local` — was reported as
  a missing tool, which also stopped the gate. omatty now asks the shell
  (`command -v`) instead of looking for a binary. (#248)
- A gate step written relative to its repository —
  `./scripts/check-coverage.sh`, which `omatty gate` itself proposes — was
  reported not on `PATH` and never run, because the pre-flight looked for it
  in omatty's working directory rather than the step's. (#289)
- `/clear` gave claude a new conversation id and omatty never noticed: the
  tailer watched a transcript that had stopped growing, hook events for the
  new id were dropped, and the next start resumed the conversation as it was
  before the clear. The session row now follows the clear (and a compact)
  onto the new conversation, and keeps its own id. (#316)
- A typo under `[gate]` in `config.toml` was answered with a list of known
  keys that did not contain the `[gate]` ones. The list is now derived from
  the config's own fields. (#321)
- Archiving a session left its per-line coverage map in memory for the life
  of the process. (#314)

### Changed

- `internal/paste` owns invariant 8; it moved out of `internal/review` so the
  gate and the review loop can both reach it. (#223)
- A session card is three lines rather than two. (#230)
- **Boot starts only the sessions something is still holding.** `[sessions]
  lazy_start`, on by default: with `dtach`, omatty reattaches the sessions
  whose `claude` survived the quit and leaves every other one stopped until
  you press `enter` in its pane. Measured on eleven registered sessions, the
  old boot spawned 2.99 GB of `claude` for ten sessions idle three to thirteen
  days. One failed start no longer aborts the whole boot. `lazy_start = false`
  restores the old behaviour. (#317)
- **M13 — idle CPU cut by about 55%.** A frame measured each line's width
  three times over, and every checkout was polled while omatty was blurred;
  both stopped. Eight chatty sessions through a real PTY went from 35–41% of a
  core to 16–18%. (#314)
- `README.md` opens with what omatty does that the field does not — the gate,
  and review comments that survive the file changing — replacing a claim
  about the field that M12 found to be false. (#301)

### Known limitations

- The agent seam still has one profile, claude. Codex is a follow-up. (#152)
- Scrollback is not preserved across a detach and reattach. (#336)
- Each embedded terminal holds a 4 MiB parser buffer and two 10,000-line
  scrollbacks, which is most of omatty's retained memory and lives upstream.
  (#315)

## [v0.1.0] — 2026-09-10

First release. Eight milestones, built on `develop` between 2026-09-01 and
2026-09-10, promoted to `main` together (#134).

### Added

- **M1 — Skeleton.** Projects and sessions in `~/.omatty/state.json`,
  worktrees created on demand, the real `claude` binary running inside an
  embedded PTY pane, and modal key routing where every keystroke reaches
  Claude except the `ctrl+o` leader. (#1–#13)
- **M2 — Status.** Per-session glyphs, age and cumulative token usage in the
  sidebar, derived from Claude Code hooks over a unix socket and from each
  session's transcript JSONL — never scraped from the rendered screen.
  Desktop notifications when a session starts waiting on you. (#17–#20)
- **M3 — Review.** `ctrl+o d` opens a diff of everything a session changed —
  commits since it branched, uncommitted edits and new files in one view.
  Comments anchor to line *content*, not line numbers, so they survive Claude
  editing the file underneath you, and `S` sends the whole batch as a single
  bracketed-paste prompt. (#21–#24)
- **M4 — Lifecycle.** Rename (`ctrl+o R`), archive (`ctrl+o x`), jump by name
  (`ctrl+o /`), and project discovery from the repositories claude already
  knows you use (`ctrl+o a`). (#91, #40–#42)
- **M5 — File tree.** `ctrl+o f` shows the session's worktree in the same
  column as the diff, with per-file change markers in the diff's own colours,
  a filter, `@path` attach, and syntax-highlighted previews kept in colours
  distinct from the diff's. (#194–#200)
- **M6 — Persistence.** With `dtach` installed, quitting detaches from
  sessions rather than ending them and relaunching reattaches, so a turn in
  flight survives. Optional: without the binary omatty behaves as before and
  says so once. Sessions claude already holds can be adopted. (#43, #122)
- **M7 — Reach.** `~/.omatty/config.toml`, mouse support, the agent seam in
  `internal/agent`, automatic session naming, and a visual identity — activity
  lanes, a cache-hit token meter and denser panel chrome. (#44–#46, #127,
  #128, #130, #133, #153–#155)
- **M8 — Surface.** The frame, colour rule, session cards, header, footer and
  sidebar diffstat the panes are drawn in. (#174–#180)
- `omatty --version` reports the release the binary was built from, read from
  the linker's `-X main.version` or, failing that, the module version the go
  tool stamps. (#134)
- `docs/ARCHITECTURE.md` — data flow, the package table, the eleven invariants
  each with the failure behind it, and the four seams. (#156)

### Fixed

Bugs found by running the merged result, each with a regression test named
after its issue:

- Claude's window title drawn into the pane: `x/ansi` read the byte `0x9C` as
  a string terminator inside an OSC payload even in UTF-8, and every Dingbat
  is `E2 9C xx`. (#192)
- Every pane blank after a restart — dtach clears on attach and re-signals at
  the same size, which the kernel does not deliver. (#191)
- Paste never reached claude: a paste is a `PasteMsg`, not keystrokes, and
  nothing routed it. (#190)
- A copy made inside a pane now reaches the host clipboard: omatty lifts the
  `OSC 52` the program writes and hands it to your terminal. (#212)
- The review column ignored the mouse and was indistinguishable from the diff
  claude draws in its own pane. (#168)
- A project with no sessions could not be selected, making discovery unusable.
  (#158)
- A registered project could never be removed. (#159)
- Token counts read as near-zero on a well-cached session: claude reports
  `input_tokens` as the uncached remainder alone. (#170)

### Known limitations

- Dragging to select text scrolls instead, because omatty asks the terminal
  for the mouse. Hold `shift` (Ghostty, kitty, xterm, Alacritty) or `option`
  (Apple Terminal, iTerm2) and drag. (#217)
- `ctrl+o N` still asks for a worktree branch name. (#151)
- The agent seam has one profile, claude. Codex is a follow-up. (#152)
- Scrollback is not preserved across a detach and reattach.

[Unreleased]: https://github.com/WilsonSousajr/omatty/compare/v0.5.0...HEAD
[v0.5.0]: https://github.com/WilsonSousajr/omatty/releases/tag/v0.5.0
[v0.4.0]: https://github.com/WilsonSousajr/omatty/releases/tag/v0.4.0
[v0.3.0]: https://github.com/WilsonSousajr/omatty/releases/tag/v0.3.0
[v0.2.0]: https://github.com/WilsonSousajr/omatty/releases/tag/v0.2.0
[v0.1.0]: https://github.com/WilsonSousajr/omatty/releases/tag/v0.1.0
