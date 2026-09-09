# omatty roadmap

Last revised 2026-09-08, when M7's close-out recorded what it did not close.
Every milestone is built; what is left is under "What is left".

omatty is a terminal ADE: several projects and several parallel Claude Code
sessions in one window, each session the real `claude` binary in an embedded
pane. It is built for one person first and opened up later, so every
milestone is ranked by how much daily friction it removes, and nothing on
this list exists only for strangers until M7.

Milestones are vertical slices. Each one produces software that is usable on
its own, and each ends with a smoke test of the real binary in a real PTY -
not only the coverage gate. See "Rules" at the end for why.

## Where things stand

| | Milestone | Status |
|---|---|---|
| M1 | Skeleton | **Done.** #36, #35, #15 closed; merged to develop. |
| M2 | Status | **Done.** Live glyphs, age, tokens, notifications; merged to develop. |
| M3 | Review | **Done.** #21-#24 merged to develop; diff, comments, submit, file tree. |
| M4 | Lifecycle | **Done.** PRs #98-#104 merged to develop 2026-09-05, review findings in #119. |
| M5 | File tree | **Planned** 2026-09-09; #24 shipped in M3, the rest is #194-#200 in Backlog. See the M5 section. |
| M6 | Persistence | **Done.** #43 and #122 merged as PRs #121 and #123 on 2026-09-05. |
| M7 | Reach | **Built** 2026-09-07 as PRs #135-#148. Leftovers still open; see "What is left". |
| M8 | Surface | **Built** 2026-09-09 as PRs #181-#186, stacked; see the M8 section and "What is left". |

The board at github.com/users/WilsonSousajr/projects/13 is the live view;
this document is the reasoning behind its order.

---

## M1 - Skeleton *(close-out)*

**Delivers:** projects and sessions registered, worktrees created on demand,
the real `claude` running inside an embedded pane, modal key routing so every
keystroke reaches Claude except the `ctrl+o` leader.

**Built:** #1-#13, then seven bugs found by actually running it (#28-#34),
each fixed with a failing test first.

**Still open, and blocking the merge:**

- **[done] #36 - restarting omatty killed every used session.**
  `StartTerminals` launches `claude --session-id <uuid>`; once a session has a
  transcript, claude refuses with "Session ID is already in use". There is no
  lock file - the transcript is the claim. The launcher must use
  `--resume <uuid>` when `paths.Transcript` exists and `--session-id` only
  when it does not. Invariant 9 promised exactly this; the code never did it.
- **[done] #35 - panes are now side by side, styled with lipgloss.** The
  sidebar renders above the terminal, so a growing session list pushes the
  thing you are reading down the screen. Fixing this rewrites `View`, so real
  styling (lipgloss borders, focused-pane highlight, status colours) lands in
  the same change rather than touching `View` twice.

**Done:** a used session survives quit-and-relaunch (#36), the sidebar sits
beside the terminal (#35), and ctrl+o r restarts a crashed or exited session
(#15). Verified with real claude in a sized PTY at 100x30 and 60x20.

## M2 - Status

**Delivers:** the sidebar tells you what every session is doing, across all
projects, without switching into any of them.

**Why next:** it is the single thing that stops omatty being the daily driver
over terminal tabs. Six sessions all showing `-` is six sessions you have to
click into.

**Contents:**

- #17 write real hooks into `~/.omatty/hooks.json` (`Notification`, `Stop`,
  `PreToolUse`). #31 stubbed this file as `{"hooks":{}}` so sessions could
  start; M2 fills it in. Invariant 3: omatty's own file, never the user's.
- #18 a unix socket the hooks report to - low latency.
- #19 tail `~/.claude/projects/<slug>/<uuid>.jsonl` - the truth. This is why
  omatty assigns the session UUID itself (invariant 2). Status is derived from
  structured data, never scraped from the rendered pane.
- #20 glyphs in the sidebar: `● thinking` `⚙ tool` `⏸ waiting for you`
  `✓ done` `✗ error`.
- **Last-activity age** - `parser-fix ⏸ 4m`. The thing that stops a waiting
  session being forgotten. One more column from the same tailer.
- **Desktop notification** when a session needs you while omatty is in the
  background. Fires from the hook event; deferred from v1 and now cheap.
- **Token / cost per session**, read from the JSONL, so you can see which
  session is spending your quota.

The last three ride on the tailer and are each about a day. They are in M2
rather than a follow-on because they are the same data shown three ways, and
status without "for how long" is half a feature.

**Done:** the sidebar shows a live glyph and age per session and the focused
box shows tokens; a permission prompt turns the glyph to `!` within a second,
and a backgrounded session that needs you fires a desktop notification. Hooks
plus a JSONL tailer feed it (invariant 2); a failed socket degrades to
tailer-only (#49). Verified with a cross-process e2e test of the real hook
binary.

## M3 - Review

**Delivers:** the reason the tool exists. Review a session's diff without
leaving omatty, comment on lines, send the whole batch back to Claude as one
message. Browse the session's worktree and preview the files it touched.

**Contents, exactly as approved and nothing more (file tree added
2026-09-02):**

- #21 diff pane: merge-base against the session's base branch, unioned with
  uncommitted changes - everything this session changed, committed or not.
  Parsed from git's unified output with `bluekeyes/go-gitdiff`, through
  `internal/vcs` (invariant 4).
- #22 comments anchor on `(file, hunk header, line hash)`, never a line
  number. Claude edits files while you read them; a line-number anchor
  silently attaches feedback to the wrong code (invariant 7). Orphans float to
  the top of the file marked moved.
- #23 `[S]` composes one message - `file:line`, the quoted line, your note,
  per comment - and writes it to the PTY as a bracketed paste followed by one
  `\r` (invariant 8). This is the `SendInput` path; keystrokes use `Update`.
- #24 file tree: browse each session's worktree, see which files that
  session touched, preview one without leaving omatty. Pulled out of M5 into
  M3 on 2026-09-02; built on 2026-09-03 as the review column's second view,
  which is what made it small enough to bring forward. `ctrl+o f` shows the
  tree, `*` marks a file the diff changed, `enter` folds a directory or
  previews a file, bounded at 256 KiB.

**Deliberately out:** asking Claude to self-review, commit/push/PR from
omatty, running N sessions on one task and comparing, broadcasting a prompt.
All considered; all cut. Review stays a person reading a diff and commenting.
Shipping stays in git.

**Done when:** you review a two-file change, leave three comments, press
`[S]`, and Claude receives them as one message and acts on all three; and
you open the file tree, see the two touched files, and preview one.

## M4 - Lifecycle

**Delivers:** managing projects and sessions once you have many of them.

**Why here:** by the end of M3 you will have ten or more sessions and no way
to get rid of one except editing `state.json`. None of this depends on M2 or
M3, so it could go anywhere; after the two big milestones is where the pain
peaks.

**Contents:**

- **Project discovery** (#91): `omatty add <dir>` registers one repository at
  a time, typed from memory. Claude already knows every project you have used
  it in - `~/.claude/projects/<slug>/<uuid>.jsonl` - so omatty proposes that
  list instead. It *proposes*: `omatty add` and manual session creation stay
  exactly as they are, and nothing enters `state.json` without you choosing
  it, so invariant 9 holds and the registry stays the single source of truth.

  The slug cannot be reversed (`/` and `.` both become `-`), but each
  transcript records its own `cwd`, which sidesteps the lossy mapping. Three
  filters turn a store into a list worth reading: the directory must still
  exist, it must be a git repository, and a linked worktree must resolve to
  its parent - `--show-toplevel` returns the worktree, so the main checkout
  comes from `--git-common-dir`. On a real store that is 34 slug dirs
  collapsing to 6 projects, which is the whole argument for doing it properly
  rather than listing the directory.

- **Kill / archive** (`ctrl+o x`): stop the process, optionally
  `git worktree remove`, drop it from the sidebar. A confirmation, because
  the worktree may hold uncommitted work.
- **Rename** (`ctrl+o R`): retitle in place. Title is display-only, so this
  is a `state.json` edit and a sidebar rebuild.
- **Restart a crashed session** (`ctrl+o r`): the crash frame has advertised
  this since #13 (issue #15). Relaunch with `--resume` in place. Small once
  #36 exists.
- **Fuzzy switcher** (`ctrl+o /`): type a few letters, jump to a session
  across all projects. `j/k` stops scaling past about eight sessions.

**Done when:** omatty proposes the repositories you have actually used Claude
in and you register them by choosing rather than typing; and a session can be
created, renamed, crashed, restarted, archived and its worktree removed, all
from inside omatty.

**Built** on 2026-09-04 as six PRs to `develop`, one issue each: #98 (#95, the
resize bug), #99 (#41 rename), #100 (#40 archive), #101 (#42 switcher), #102
(#91 discovery) and #104 (#103, the help modal). All six merged to `develop`
on 2026-09-05, followed by #119 for the review findings (#111). The plan is
`docs/superpowers/plans/2026-09-04-m4-lifecycle.md`.

Three things worth remembering from building it:

- **Three surfaces wanted the keyboard, so one modal layer carries all of
  them.** The prompt used to take it by making `focusedTerminal` return nil,
  which was load-bearing for key routing, rendering *and* the pane border.
  One `modal` field with a kind enum replaced the bool, and `keys.Router` and
  `focusTarget` did not change at all: invariant 1 survived four new surfaces
  without the router learning anything about them.
- **#95 was a class, not a path.** `focusedTerminal` answered "who owns the
  keyboard" and "which PTY fills the pane" with the same nil. Splitting them
  first meant the three surfaces after it inherited the fix rather than
  re-creating the bug, and the regression test is a table each one adds a row
  to.
- **The smoke test earned its place again.** The gate was green when
  `ctrl+o ?` then `ctrl+o q` typed a literal `q` into Claude: a modal makes the
  terminal unfocused, so the leader is never armed while one is open, and a
  help box that closed on any key swallowed it. Only a real PTY showed that.

**Deliberately out:** adopting *sessions* omatty did not create. Discovery
stops at projects. Reading a transcript to reconstruct a session and resume
it is the same code path as "reattach to my own session after a restart",
which is M6 - building it twice is the waste. Attaching to a `claude` already
running in another terminal is not on the roadmap at all: you cannot adopt a
PTY you do not own.

## M5 - File tree

**Delivers:** the tree grows from a listing into a surface. #24 - browse each
session's worktree, see which files that session touched, preview one without
leaving omatty - was pulled into M3 on 2026-09-02 and built there on
2026-09-03 as the second view of the review column, which is what made it
small enough to bring forward. What shipped is a `git ls-files` listing folded
into a tree, `*` on a file the diff changed, and a preview that is numbered
plain text. M5 was empty from then until 2026-09-09, when it was refilled
with what the tree still lacks. The docs issue is #193; the spec is
`docs/superpowers/specs/2026-09-09-m5-file-tree-design.md`.

The list was chosen by looking at what the terminal tools an operator would
otherwise reach for all do. yazi highlights previews with no external tool
and rolls git state up to the directory; superfile and the Bubble Tea file
explorers highlight through chroma; nvim-tree shows a glyph per git state,
sorts directories first, and binds a live filter to one key; the OpenCode
TUI highlights with the terminal's own palette and lets `@` reference a file
into the conversation, which no file manager can and which is the one that
fits an ADE. Those converge on seven issues, one PR each, in build order:

- **#194 sort.** Directories before files at every depth, then
  case-insensitive. A comparator over path components replaces
  `sort.Strings` in `review.NewTree`; the pre-order contiguity `Visible()`
  folds on must survive it.
- **#195 re-list on turn end.** `refreshReview` re-lists the worktree beside
  the diff when the column is open, and the reopen re-lists a stale one, so
  a file claude creates appears without `r`. `Tree.Relist` keeps the
  collapse state; the cursor stays on its path.
- **#196 change markers.** `M A D R` in the mark column instead of `*`,
  coloured amber, green, red and amber, the hues the diff already gives
  those states, so the colour rule holds. The kinds come from
  `review.File.Status`, which the diff already carries: no new git call.
  Deleted files, in the diff but not in `ls-files`, become rows for the
  first time.
- **#197 highlighting.** A new `internal/highlight` package owns chroma the
  way `termwrap` owns bubbleterm; nothing else imports it. The style is
  omatty's own, built from `style.go`'s indices, because every stock theme
  spends the accent and the diff hues on keywords and strings. Lines are
  highlighted once when the file is opened and stored beside the plain
  ones, which keep measuring width; panning goes through the ANSI-aware
  cut in `charmbracelet/x/ansi` so `l` never shows half an escape. Files
  over the budget draw plain with a note. The diff view is not highlighted
  by this issue.
- **#198 filter.** `/` opens a filter line; `fuzzy.Match` against the path
  narrows the listing live, ancestors kept, folded directories opened while
  the filter is on; `enter` keeps it, `esc` clears it. The leader router is
  untouched: `/` is a plain key in an already-focused column.
- **#199 attach.** `a` on a row writes `@<path> ` to the session inside
  paste brackets with no carriage return - `review.BracketedText`, the
  submitting `BracketedPaste`'s sibling - and hands focus back to claude's
  composer. Labelled `invariant` for 8. Whether claude's `@` picker takes a
  pasted path is a question for the real-PTY smoke run, and the PR records
  the answer.
- **#200 cross-links.** `o` on a hunk line opens the preview at that line's
  number; `o` in a preview lands the diff cursor on that file's header.
  The data already lines up: an entry carries its `Position`, a `Line` its
  `NewNo`.

**Cut**, and written down so it does not return sideways: Nerd Font
file-type icons and a second theme, both refused in M8; wrapping long lines
in the preview, when panning already reaches them; a markdown renderer for
the preview; watching the worktree with fsnotify, when the turn boundary is
the moment the operator looks. Highlighting the diff view is deferred, not
cut: it is one more caller of #197's package once that exists.

## M6 - Persistence

**Delivers:** sessions outlive omatty. Quit, close the terminal, reboot the
shell; every claude is still running mid-turn and reattaches on relaunch.

**How:** each `claude` runs under `dtach` (tiny, no UI of its own) rather
than tmux, so omatty inherits no prefix key, no status bar and no nested
multiplexer. On relaunch omatty attaches to the socket instead of starting a
new process. #36's `--resume` remains the fallback when the socket is gone.

**dtach is optional**, which was not obvious when this was planned. It is not
installed by default anywhere, so requiring it would have made omatty harder to
run in exchange for a feature you only notice when you quit. Absent, omatty
behaves exactly as it did before and says so once in the footer - the same
degradation as a hook socket that will not bind (#49).

**Why this late:** it was cut from v1 and you confirmed it is not the top
pain. It is the biggest technical risk left - nested terminal emulation on
top of a detached PTY - and it is fully isolated from everything above it. It
belongs after the tool has proven itself, not before.

**Session adoption belongs here**, not in M4's discovery. Reconstructing a
session from its transcript and resuming it, and reattaching to a session
omatty itself started, are the same problem seen from two sides; M6 is where
that machinery exists. M4 discovers projects only.

Built as #122: `ctrl+o A` lists the claude sessions in the project under the
cursor that omatty does not already hold, titled by their first typed prompt,
and adopting one registers it and starts it. `omatty adopt <project>` is the
CLI twin. An adopted session records `worktree: false`, because omatty did not
create that directory and archive must never offer to delete it (#40).

**Done when:** you quit omatty mid-turn, relaunch, and the turn finishes on
screen.

**Built** on 2026-09-05 as two PRs to `develop`, one issue each: #43
(persistence) and #122 (adoption), both merged to `develop` the same day. The
plan is `docs/superpowers/plans/2026-09-05-m6-persistence.md`.

Three things worth remembering from building it:

- **The unit tests could not have found either real bug.** `internal/detach`'s
  tests assert the command line dtach is *given*, so a missing `~/.omatty/s`
  shipped green and broke every session start; and the adoption key matched
  `shift+A`, a spelling no terminal sends, so it opened nothing on a real
  terminal while every test passed sending the legacy `A`. Both were found by
  running the binary. That is rule 2 twice in one milestone.
- **A test can pass for the wrong reason and look thorough.** `Stop`'s first
  test killed a stand-in process and asserted it died - but the test was the
  process's parent, so the zombie's pid still answered signal 0, `Stop` waited
  out its whole grace period, and the test was really proving the SIGKILL
  escalation. Reaping in a goroutine models production (dtach's master reaps
  claude) and turned 2s into 30ms. The timing *is* the assertion.
- **Quitting needed no code at all.** Closing the PTY ends the dtach client and
  leaves the master, so persistence arrived by adding nothing to the quit path -
  and the test that matters most is the one asserting quit ends *nothing*.

## M7 - Reach

**Delivers:** the open-source door, and the polish a week of real use turned
out to need before anyone walks through it.

M7 was three issues when this document was written. It is now twelve. Nine of
them arrived after M4 and M6 landed, because those two milestones put enough of
omatty in front of the operator for a day's real use to find what a coverage
gate never will: seven reports on 2026-09-05 and 2026-09-06, one more found
while reviewing the fix for the previous one, and one from auditing the branch
itself. They belong here rather than in a milestone of their own because they
are the same thing the original three are - the difference between software
that works and software a stranger would keep.

**Ordering.** Bugs first, because they make shipped features unusable rather
than merely awkward. Then navigation, which is the friction the operator hits
every day. Then the original three, in their existing order, since the config
file is what the last two need. Chrome last, because it touches every renderer
and the bugs live in those same files.

### Blocking bugs

- **#131 - the file tree hangs on "listing files..." forever** if you open the
  diff before the tree. `nil` from `treeRows()` is being asked to mean "not
  loaded yet", and two states that never resolve produce it too. This kills
  #24 - a whole M3 deliverable - on the path most people take to it.
- **#124 - the review column cannot be closed once you press `esc`.** `esc`
  drops keyboard focus but leaves the column on screen; `ctrl+o d` then takes
  focus back rather than closing. The reflex cycles forever and half the
  window stays gone.
- **#129 - the sidebar never scrolls.** Rows past the fold are never drawn,
  nothing indicates the list continues, and the cursor still moves onto them.
  One tall project is enough to hit it; seven make it the first thing you see.

### Navigation and naming

- **#130 - no way to move between projects.** The cursor skips project headers
  by design (`sidebar.go:47`), which is right for `j`/`k` and leaves nothing
  that jumps to project 2. The other half of #129's report.
- **#126 - `ctrl+o j`/`k` dead-end at both ends** instead of rotating, with no
  feedback saying why. On a two-session project that turns a leader key into
  counting rows and reversing at each end.
- **#127 - name a session and its worktree automatically.** `ctrl+o n` demands
  a title before the work it would describe exists, and under `N` that same
  buffer becomes the branch name. One headless call can name both from the
  first prompt. This is omatty doing its own housekeeping, not an agent acting
  on the work, which is what keeps it clear of the orchestration cut below.

### The original three

- **Config file** (`~/.omatty/config.toml`, #44): leader key, claude binary
  path, worktree root, default base branch. All hardcoded today. First of the
  three because the other two need it.
- **Mouse** (#45): click a session row, scroll the pane. Partly built already,
  and not as part of this milestone - #107 gave the session pane a wheel and
  #125 gave the review column a horizontal axis. What remains is the sidebar
  hit-testing, so that a click selects a row.
- **Other agents** (#46, Codex and opencode): an agent is a command template
  plus a status adapter. The seam has been kept thin on purpose since M1; this
  is the milestone that widens it. Needs #44 for the binary path.

### Carried in from review

- **#133 - `panReview` rebuilds every row of the view on each rightward
  notch**, so a wheel flick over a large preview blocks `Update` for ~180 ms.
  Found reviewing PR #132 and half-fixed there. `reviewMaxWidth()` walks and
  rebuilds rather than caching, a trade #94 made deliberately for one `l` press
  at human repeat rate. A wheel sends a burst, and the budget does not survive
  it.
- **#128 - give omatty a visual identity.** Four colours, six ASCII status
  glyphs and a rounded border today; nothing on screen is drawn rather than
  written. Activity lanes, meters and denser panel chrome, closer to `btop`.

### Release

- **#134 - `develop` has never been promoted to `main`.** Six milestones and
  164 commits sit on `develop`; `main` is the branch a stranger clones and it
  shows none of them. The door M7 opens leads to that branch, so the promotion
  rule - what it is and what gate it clears - has to exist before this
  milestone can claim to be done. No tag without approval; the decision comes
  first.

Nothing in M1-M6 is allowed to bake in a personal path or assumption that
M7 would have to undo. That is the cost of "open source later" and it is
paid continuously, not here.

**Built** on 2026-09-07 as thirteen PRs to `develop`, one issue each plus a
refactor that brought `model.go` and `main.go` under the 500-line limit first:
#135 (plan), #137 (#136 split), #138 (#131), #139 (#124), #140 (#133), #141
(#126), #142 (#129), #143 (#130), #144 (#44), #145 (#45), #146 (#127), #147
(#46) and #148 (#128). All merged the same day. The plan is
`docs/superpowers/plans/2026-09-07-m7-reach.md`. Scope decided that morning:
#127 shipped steps 1 and 2, #46 shipped the seam with claude as its only
profile, #128 shipped its first slice, and #134 was deferred.

Three things worth remembering from building it:

- **The smoke harness has two traps of its own.** A scratch HOME under a
  long path makes the hook socket exceed the 104-byte unix limit and `bind`
  fails silently into tailer-only mode; use a short one. And `ptyrun`'s
  default key is `ctrl+o q`, which leaves the captured final screen blank -
  pass a non-quitting key. A third: `esc` must end its key chunk, because
  `\x1b\x0f` in one write parses as alt+ctrl+o.
- **`claude -p` writes a transcript and waits for stdin.** A headless call
  leaves a session under `~/.claude/projects/<slug of cwd>/`, so the namer
  runs in a temp directory it removes on exit; and it waits three seconds for
  piped input unless stdin is closed. One call cost about $0.60 at list
  price, which is why `[naming] model` is off by default.
- **Every "one more row" is one constant.** `titleRows`, `sidebarHeaderRows`,
  `laneCells`: the caret, the wheel target, the click hit-test and the PTY
  height all derive from them, so #128 moved the title into the border by
  changing a 1 to a 0 and eight pinned tests moved with it. The tests that
  pin literals are the ones that catch a constant nobody meant to change.

**Done when** (revisited with #134): omatty reads its settings from a file
rather than from its own source - done; a click selects a session - done; a
second agent runs in a pane - the seam is built, the second agent is a
follow-up; the nine reports above are closed with regression tests - done;
and the branch a stranger clones is the software this repository has built -
deferred with #134.

## M8 - Surface

**Delivers:** the frame omatty's panes sit in, redrawn after the two
references the operator named - monocode's flatness and ade's card anatomy -
and the status palette made to mean one thing per hue.

The spec is `docs/superpowers/specs/2026-09-09-tui-redesign-design.md` (#172)
and the plan `docs/superpowers/plans/2026-09-09-m8-surface-redesign.md`. Three
decisions were made before the spec was written and are not reopened: of
three directions proposed, flat hairline columns with two-line cards won over
one-line rows and over a single joined frame; the colour rule replaces the
hues #128 shipped; and a per-session git poll for branch and diffstat is new
data omatty reads.

**What it is, in seven slices**, one issue and one PR each:

- **#174 frame (PR #181).** The three rounded boxes go. A header row, a rule
  with `┼` under each hairline, hairline-joined columns, the footer. Claude
  gains two columns at every width and keeps its rows. One focus rule for the
  screen: the accent hairline stands on the left edge of whatever owns the
  keyboard, decided once in `keyboardEdge`.
- **#175 colour rule (PR #182).** Eight indices; working states are text,
  amber is waiting and nothing else, green done, red error, muted
  not-actionable, accent 75 for focus alone. `○ ◐ ◆ ● ✓ ✕ ∅` replace the
  gear and pause signs. A test binds amber to waiting alone.
- **#176 cards (PR #183).** Two lines per session: glyph, title (eighteen
  columns, up from fifteen), age; then branch, diffstat, lane. The rail `▎`
  is the cursor on both lines. `rowHeight` is one function read by the
  scroll window and the click hit-test, so a click on either line is the
  card and the selected card is never split.
- **#178 footer (PR #184).** Keys left as before; `N sessions · K waiting`
  right, the waiting count amber, dropped whole before the keys.
- **#180 diffstat (PR #185).** `vcs.Shortstat` → `review.Source.Stat` →
  `ui.Deps.Stat`, polled every ten seconds, at once on done or waiting, and
  once at start; one poll in flight per session; a failure keeps the last
  stat and logs once. Tracked changes only, so a card can read lower than the
  review column, which is the truth when opened.
- **#177 header (PR #186).** The breadcrumb `project ▎ title · branch ·
  status age` with the meter and #170's counts right-aligned, collapsing
  counts, then meter, then branch; the status never. A modal names itself in
  the segment.
- **#179 close-out (this PR).** The smokes below, this section, `M8` in
  AGENTS.md.

**Built** on 2026-09-09 as six stacked PRs to `develop`, each targeting
`develop` and branched from the one below, so every diff shrinks to its own
slice as the stack merges bottom-up: #181, #182, #183, #184, #185, #186. #186
also carries a merge of #171 (#170's counts). The order departed from the
plan once: #177 was built after #180 because #171 had not merged, which let
the breadcrumb read the poll's branch directly instead of a stub.

Three things worth remembering from building it:

- **A mockup drawn by hand drifts; a mockup asserted by a script does not.**
  The spec's screen was regenerated by a script that measures every line at
  120 cells, East Asian width included, after two by-hand versions put the
  age against the hairline. The blank column before the hairline exists
  because of that.
- **`fitLine` now pads a cut that lands inside a wide rune.** The header row
  is exactly the frame's width only because of it; the box's rule used to
  measure the title itself.
- **Defining a style before its first use fails the gate.** `unused` runs
  on package-level vars, so `textStyle`, `accentStyle` and `amberStyle` are
  declared in the slice that first renders with them, not in the palette
  slice the plan assigned them to.

**Cut**, and written down so it does not return sideways: a second theme;
background fills for any column; Nerd Font or provider glyphs; a
context-window gauge like ade's (usage is known, the window size is not);
plan, subagent or "changes" panes; blank margins between cards; tabs; a
composer; a sidebar width other than 28. The gear and pause glyphs are not
coming back.

**Done when:** every PR above is merged; the three real-PTY smokes at 120x32
(review column open, and a modal open), 80x24 and 60x20 are read by a person
and attached to #179; and one run with the real `claude` binary shows its own
prompt box in the pane with no omatty frame around it.

## What is left

Everything M7 opened and did not close, plus what running the merged result
has found since, in the order it should be picked up.
Each has an issue, labelled `M7` for the milestone it came out of and parked
in Backlog until someone is actually on it. The label says where the work
belongs; the column says whether anyone has picked it up. A bullet marked
done stays here, naming its PR, so the list is still the whole account of
what M7 left.

- **#158 - a project with no sessions cannot be selected.** Done 2026-09-08,
  PR #161. The cursor rests on an empty project's header; `Selected()` is
  `ok=false` there, so `]`, `j`/`k`, a click and `ctrl+o n` all reach a
  project discovery just added. #130's skip was the bug; its tests were
  redefined, not weakened.
- **#159 - a registered project can never be removed.** Done 2026-09-08,
  PR #164. `registry.RemoveProject` refuses while sessions exist and never
  touches the repository; `omatty rm <project>`; `ctrl+o x` on an empty
  project's header.
- **#134 - promote `develop` to `main`.** Deferred on 2026-09-07. `develop`
  carries all seven milestones; `main` still holds `ca3952b`, the bootstrap
  commit of 2026-09-01, and has no protection. Decide the shape (merge commit
  per milestone, fast-forward, or a PR), write the gate it must clear - CI
  plus the real-PTY smoke test rule 2 already requires - into this document
  and AGENTS.md, then promote. Nothing ships to a stranger until this is done,
  and no tag without approval.
- **#151 - name the worktree branch (#127 step 3).** `ctrl+o N` still demands
  a name because `git worktree add -b` bakes it into a directory and into
  `state.json`. Either a placeholder branch (`omatty/<date>-<n>`) renamed only
  while it has no commits, or keep asking for this one string. It must use
  `registry.Slug`, the same filter step 2 applies to model output, never a
  looser one.
- **#152 - a second agent profile, Codex first.** #46 built the seam with
  claude as its only entry; the roadmap's original promise was Codex and
  opencode. Each is one file in `internal/agent`: a command template, a
  transcript location, hook events (or none, degrading to transcript-only
  status), and a `watcher.Adapter` for its transcript shape. Two known costs,
  written down in #46's PR: `paths.HooksFile` is one file for the whole app,
  so an agent with a different settings schema needs a file per profile; and
  discovery and adoption read claude's store only, so adopting another agent's
  sessions is its own issue.
- **#153 and #154 - the rest of #128.** Both done 2026-09-08. The token
  meter (PR #163) is in the focused pane's rule, the operator's call on
  screen: `▰▰▰▰▰▰▱▱ 80% cached`, cache-read over everything the prompt was
  fed. The counts beside it say that same total, not `input_tokens`, since
  #170: claude reports that field as the uncached remainder alone, so a
  well-cached session read `154 in / 62.6k out` as though it had sent
  nothing. Truecolor (PR #166) went the other way from the 256 rule for two ramps
  only - the lane fades with age, the newest cell kept at full colour, and
  the meter warms amber to green - because bubbletea detects the profile and
  quantises; a test asserts each ramp still reads at 256 and 16 colours. The
  reasoning is in `style.go`.
- **#155 - the lane's title budget, judged on screen.** Done 2026-09-08,
  PR #165: `laneCells` 8 → 6, `SidebarWidth` stays 28, fifteen title columns.
  Width 32 was judged against it and lost: it takes four columns from the
  session pane at every width. The comparison is in `laneCells`' comment.
- **#156 - `docs/ARCHITECTURE.md`.** Done 2026-09-09. Data flow, the
  package table, the eleven invariants each with the failure behind it, and
  the four seams; AGENTS.md's documentation map points at it again.

---

## Not on the roadmap

Considered and cut, so they do not creep back in through the side door:

- Claude self-reviewing its own diff
- Commit / push / PR from inside omatty
- Running N sessions on one task and comparing the results
- Broadcasting one prompt to several sessions
- SSH / remote sessions
- Attaching to a `claude` already running in another terminal. omatty renders
  a PTY it owns; there is no supported way to adopt one it does not. M6's
  dtach sockets cover the case that matters - omatty's own sessions surviving
  a quit. Not to be confused with M4's project discovery (#91), which reads
  the transcript store and registers nothing by itself.
- Themes beyond one, or keybinding customisation beyond the leader
- Any orchestration, planning board, or agent-to-agent messaging. omatty is a
  window, not an orchestrator.

## Rules

1. **Every bug gets a failing test before the fix.** Procedure in AGENTS.md.
2. **Every milestone ends with a real-binary smoke test in a sized PTY.**
   M1's three worst bugs - sessions dying at start (#31), terminals never
   pumped (#33), sessions dying on restart (#36) - all passed a 92% coverage
   gate, because every test substituted a fake for claude. Coverage measures
   units; these were failures of the wiring between them. `testdata/`
   carries a PTY harness for this; using it is part of "done".
3. **A milestone is not done while a blocker is open.** M1 is the example.
4. **Invariants are argued, never assumed.** Anything touching the ten in
   AGENTS.md says so in its commit message.
