# omatty roadmap

Last revised 2026-09-10, when v0.1.0 promoted `develop` to `main` (#134).
Every milestone is built; what is left is under "What is left", and how a
release reaches `main` is under "Releases".

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
| M5 | File tree | **Built** 2026-09-09 as PRs #202-#208, one per issue #194-#200; #199's real-PTY answer is still owed. See the M5 section. |
| M6 | Persistence | **Done.** #43 and #122 merged as PRs #121 and #123 on 2026-09-05. |
| M7 | Reach | **Built** 2026-09-07 as PRs #135-#148; the four terminal bugs a day of use found were fixed 2026-09-09 as PRs #210-#214. Leftovers still open; see "What is left". |
| M8 | Surface | **Built** 2026-09-09 as PRs #181-#186, stacked; see the M8 section and "What is left". |
| — | **Released** | **v0.1.0**, 2026-09-10. All eight promoted to `main` (#134). See "Releases". |
| M9 | The Gate | **Done.** Thirteen slices built 2026-09-12/13 as PRs #235-#249, closed out in #250. Released in v0.2.0. |
| M10 | Coverage on the diff | **Done.** Seven slices #251-#257 built 2026-09-14/16 as PRs #259, #270, #273-#277; closed out in #258. Released in v0.2.0. |
| M11 | The Harness | **Done.** #260-#263 merged 2026-09-14 as PRs #264-#268; the two follow-ups it deliberately left, #267 and #269, merged 2026-09-16 as PRs #279 and #280. Released in v0.2.0. |
| M12 | The Field | **In progress.** The research half is #295-#302, eight slices, captured 2026-09-18 into `docs/research/` and `docs/comparison.md`. Of what it produced, #310 and #311 are built and released in v0.3.0. On 2026-09-25 it took back the P1/P2 issues it had cut (#379); what is open is the milestone's open issues on the board, not a count here. |
| M13 | Memory and idle CPU | **Done.** PR #314, merged 2026-09-22. Released in v0.2.0. |
| — | **Released** | **v0.2.0**, 2026-09-22. M9-M11, M13 and the session lifecycle promoted to `main` (#328). See "Releases". |
| — | **Released** | **v0.3.0**, 2026-09-25. M12's verification core (#311, #310, #335, #342), the release pipeline (#327) and the MIT license (#362). See "Releases". |
| — | **Released** | **v0.4.0**, 2026-09-25. M12's close-out promoted to `main` (#390). See "Releases". |
| M14 | The Tracker | **Done.** Seven slices #393-#399 built 2026-09-25 as PRs #400-#406. Released in v0.5.0. |
| — | **Released** | **v0.5.0**, 2026-09-26. M14 promoted to `main` (#407). See "Releases". |
| M15 | The Polish | **Done.** Nineteen issues #421-#439 and two bugs found building them (#447, #483), merged 2026-09-26 as PRs #444-#491; closed out in #492. See the M15 section. |
| M16 | The Forges | **Planned** 2026-09-26: GitLab, Azure DevOps, Gitea/Forgejo/Codeberg and Bitbucket at GitHub's parity, #449-#465, in Backlog. See the M16 section. |

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

**Deliberately out:** asking Claude to self-review, running N sessions on one
task and comparing, broadcasting a prompt. All considered; all cut. Review
stays a person reading a diff and commenting.

Commit, push and PR from omatty were cut here too, and that is **superseded**:
the decision was reserved for #331 when #310 landed, and taken on 2026-09-25
under "Acting on a pull request" below. M3's reason - that shipping is git's
job and not a review pane's - still holds for M3, which had no gate verdict to
act on and no remote verdict to read. What changed is that both now exist.

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
with what the tree still lacks and built the same day: seven PRs, #202 to
#208, one per issue, each merged to `develop` once its gate, real-PTY smoke
line and CI were green. The docs issue is #193; the spec is
`docs/superpowers/specs/2026-09-09-m5-file-tree-design.md`.

The list was chosen by looking at what the terminal tools an operator would
otherwise reach for all do. yazi highlights previews with no external tool
and rolls git state up to the directory; superfile and the Bubble Tea file
explorers highlight through chroma; nvim-tree shows a glyph per git state,
sorts directories first, and binds a live filter to one key; the OpenCode
TUI highlights with the terminal's own palette and lets `@` reference a file
into the conversation, which no file manager can and which is the one that
fits an ADE. Those converge on seven issues, one PR each, in build order:

- **#194 sort.** Done, PR #202. Directories before files at every depth, then
  case-insensitive. A comparator over path components replaces
  `sort.Strings` in `review.NewTree`; the pre-order contiguity `Visible()`
  folds on must survive it.
- **#195 re-list on turn end.** Done, PR #203. `refreshReview` re-lists the worktree beside
  the diff when the column is open, and the reopen re-lists a stale one, so
  a file claude creates appears without `r`. `Tree.Relist` keeps the
  collapse state; the cursor stays on its path.
- **#196 change markers.** Done, PR #204. `M A D R` in the mark column instead of `*`,
  coloured amber, green, red and amber, the hues the diff already gives
  those states, so the colour rule holds. The kinds come from
  `review.File.Status`, which the diff already carries: no new git call.
  Deleted files, in the diff but not in `ls-files`, become rows for the
  first time.
- **#197 highlighting.** Done, PR #205 (+4.4 MB of binary, chroma's
  lexers; 40 ms to highlight 64 KiB, the budget). A new `internal/highlight` package owns chroma the
  way `termwrap` owns bubbleterm; nothing else imports it. The style is
  omatty's own, built from `style.go`'s indices, because every stock theme
  spends the accent and the diff hues on keywords and strings. Lines are
  highlighted once when the file is opened and stored beside the plain
  ones, which keep measuring width; panning goes through the ANSI-aware
  cut in `charmbracelet/x/ansi` so `l` never shows half an escape. Files
  over the budget draw plain with a note. The diff view is not highlighted
  by this issue.
- **#198 filter.** Done, PR #206, which also re-budgeted both review
  footers under 80 columns: `j/k` and `r` moved to the help modal. `/` opens a filter line; `fuzzy.Match` against the path
  narrows the listing live, ancestors kept, folded directories opened while
  the filter is on; `enter` keeps it, `esc` clears it. The leader router is
  untouched: `/` is a plain key in an already-focused column.
- **#199 attach.** Done, PR #207, with one thing owed: whether claude's
  `@` picker takes the pasted path was not observed on the real binary (the
  scratch-HOME harness lands on claude's onboarding screen); the issue holds
  the three possible answers and what each means. `a` on a row writes `@<path> ` to the session inside
  paste brackets with no carriage return - `review.BracketedText`, the
  submitting `BracketedPaste`'s sibling - and hands focus back to claude's
  composer. Labelled `invariant` for 8. Whether claude's `@` picker takes a
  pasted path is a question for the real-PTY smoke run, and the PR records
  the answer.
- **#200 cross-links.** Done, PR #208. `o` on a hunk line opens the preview at that line's
  number; `o` in a preview lands the diff cursor on that file's header.
  The data already lines up: an entry carries its `Position`, a `Line` its
  `NewNo`.

**Cut**, and written down so it does not return sideways: Nerd Font
file-type icons and a second theme, both refused in M8 (icons came back
opt-in in M15, #425/#431; the second theme is still cut); wrapping long lines
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

- **#134 - `develop` has never been promoted to `main`.** Done 2026-09-10 as
  v0.1.0. Six milestones and 164 commits sat on `develop` when this was
  written and 307 by the time it was done; `main` still held `ca3952b`, the
  bootstrap commit. The promotion rule the issue asked for is under
  "Releases" below and in AGENTS.md's Git workflow section, and `main` is now
  protected. The decision came first, as the issue demanded, and the tag came
  with approval.

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
done 2026-09-10, when #134 promoted all eight milestones to `main` as v0.1.0.

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
background fills for any column; Nerd Font or provider glyphs (Nerd Font
came back opt-in in M15, #425; provider glyphs did not); a
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
- **#134 - promote `develop` to `main`.** Done 2026-09-10, released as
  v0.1.0. Deferred on 2026-09-07, when it was the one thing standing between
  eight built milestones and the branch a stranger clones. The shape decided:
  a pull request from `develop` to `main`, merged with a merge commit, never
  a fast-forward. The gate: the full CI gate on both runners plus the
  real-PTY smoke test rule 2 already required, read by a person. Written into
  "Releases" below and AGENTS.md; `main` is protected; the merge commit is
  tagged. `omatty --version` came with it, so the tag names something the
  binary can report.
- **#151 - name the worktree branch (#127 step 3).** Done 2026-09-18. The
  placeholder shape, not the keep-asking one. `ctrl+o N` creates the worktree
  on `omatty-<first 8 of the uuid>` and the session's first prompt renames it,
  only while the branch has nothing committed to it; `ctrl+o B` renames one by
  hand. Two deliberate departures from the sketch above, both written up in the
  PR. The placeholder carries no slash and no date counter: `registry.Slug`
  forbids `/` and `paths.WorktreeDir` joins the branch into a path, and a
  counter would collide with a branch an archived session left behind, while
  the uuid is unique by construction and - like `PlaceholderTitle` -
  recomputable from `state.json` alone, which is what lets a relaunched session
  still know its branch is provisional (invariant 9). And the worktree
  *directory* does not move: `git worktree move` would change the cwd of a
  running claude and the transcript path derived from it, which is #60. Dir and
  Branch have always been stored separately, so they were never required to
  agree. A typed branch now passes `registry.Slug` too, which it never did.
- **#152 - a second agent profile, Codex first.** #46 built the seam with
  claude as its only entry; the roadmap's original promise was Codex and
  opencode. Each is one file in `internal/agent`: a command template, a
  transcript location, hook events (or none, degrading to transcript-only
  status), and a `watcher.Adapter` for its transcript shape. Two known costs,
  written down in #46's PR: `paths.HooksFile` is one file for the whole app,
  so an agent with a different settings schema needs a file per profile; and
  discovery and adoption read claude's store only, so adopting another agent's
  sessions is its own issue.

  Half-spiked 2026-09-18 against 55 real rollouts under `~/.codex/sessions/`,
  recorded on the issue so nobody repeats it. The transcript is
  `~/.codex/sessions/<YYYY>/<MM>/<DD>/rollout-<ISO8601>-<uuid>.jsonl`, whose
  `session_meta.payload.id` is the uuid - but the date directories and the
  timestamp prefix are not known in advance, so `Profile.TranscriptPath` cannot
  be the pure join `paths.Transcript` is and the seam needs a scan. The
  envelope is `{timestamp, type, payload}`; `event_msg/task_started` and
  `task_complete` pair exactly and are the busy/idle signal, `user_message`
  carries the prompt and `token_count` the meter. One functional gap to write
  down before building: `DeriveKind` can never report `PermissionRequested`
  from a transcript, and Codex has no hook mechanism, so a Codex session can
  never show "waiting for you". What is still unknown is the half that blocks
  it - whether `codex` accepts an externally assigned session id at all, which
  omatty's whole architecture rests on, and which flag resumes one. That needs
  the real binary, which was not on the machine.

- **#217 - the mouse is captured for the whole session.** Done 2026-09-18.
  #107 asked the host for mouse reporting and never added a way to stop
  asking, so `?1002h` was held from a session's first frame to its last and the
  terminal could never select text. `ctrl+o m` hands it back, the header says
  `mouse off` while it is handed back, and the modifier-drag stays the quick
  answer for one selection.
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
- **#192 - claude's window title drawn into the pane.** Done 2026-09-09,
  PR #210. `x/ansi` takes the byte `0x9C` for the 8-bit string terminator
  inside an OSC or DCS payload even in UTF-8, and every Dingbat is `E2 9C xx`,
  so `✳️ Claude Code` ended at the ✳ and the rest landed on the grid. No
  upstream release fixes it; `termwrap` now opens the PTY itself and feeds
  the emulator through a reader that rewrites that byte inside a payload and
  touches nothing in ground. Owning the PTY cost the window size, the close
  and TERM, each with a test. SOS/PM/APC are not guarded, and the code says
  why.
- **#191 - every pane blank after a restart.** Done 2026-09-09, PR #211.
  dtach clears the pane on attach and signals at the same size, which claude
  ignores and the kernel does not even deliver. `Terminal.Repaint` changes
  the PTY's size to `h-1` and back with a pause before and between (two
  ioctls close together coalesce into one signal at the original size);
  `detach.Holder.Held` tells a held session from a fresh one before the
  terminals start, and only those panes are nudged. `dtachprobe` now stands
  in a silent child and proves dtach forwards a later size change, which its
  manual does not say. Scrollback is still gone; persisting the grid would be
  its own issue.
- **#190 - paste never reached claude.** Done 2026-09-09, PR #213. A paste
  is a `PasteMsg`, not keystrokes, and nothing routed it. It now follows the
  key table: the note and filter lines as text, the focused terminal
  re-bracketed with no carriage return (invariant 8), the column and a modal
  drop it. Copy is documented, not built: forwarding OSC 52 needs a callback
  `x/vt` does not have, and was **#212**, shipped 2026-09-10 in PR #216:
  bubbletea already had SetClipboard, and the lift runs before the C1 guard.
- **#168 - the review column ignored the mouse and looked like claude's
  diff.** Done 2026-09-09, PR #214. A click on a row moves that view's cursor
  and takes the keys; a `×` on the column's rule closes it; the rule reads
  `─ review ─────×` where the pane's stays plain dashes, so omatty's diff and
  the one claude draws inside its pane are told apart. Clicks in the pane
  stay claude's (#107).

---

## M9 - The Gate

**Delivers:** a project stops being a path and becomes a repository, the gate
that says whether work in it is sound, and the sessions running against it.
omatty runs that gate in a session's own directory, shows the verdict on the
session's card, and sends the failures back into the session that caused them.

**Why this and not something else.** v0.1.0 shipped into a field of hundreds
of agent orchestrators (sized in `docs/research/2026-landscape.md` §2, probed
2026-09-18: 184 to 1,300 repositories depending on the query, none filtered
for a maintained tool; #340), and every one of them optimises the same
variable: how much agent-work can be in flight at once. omatty bets on the
other one - how fast a person can tell whether what came back is any good - and
that bet had already decided M3's review loop and everything the roadmap
refused. M9 makes it explicit and builds the rest of it. "Not on the roadmap"
above now carries the refusals with their reasons.

**What it is, in thirteen slices**, one issue and one PR each:

- **#222 thesis (PR #235).** README says what omatty is; "Not on the roadmap"
  leads with the anti-orchestrator entry and gives each refusal a reason;
  AGENTS.md and ARCHITECTURE.md gain **invariant 12**.
- **#223 `internal/paste` (PR #236).** Invariant 8 moves out of `review` into
  a package of its own, so `review` and `gate` can both reach it without
  importing each other.
- **#224 `internal/gate` (PR #237).** Steps, exit-status verdicts, bounded
  output. A gate stops at the first step that does not pass; the rest report
  `Pending`, not `Pass`. Output is capped at capture - 200 lines or 16 KiB,
  keeping the tail, where a test runner puts the summary.
- **#238 process groups (PR #237).** Found by CI: macOS passed and ubuntu did
  not, because killing the `sh` a step runs under is enough only when it
  exec'd the command into itself. Cancelling now kills the group.
- **#225 coverage (PR #239).** A `kind = "coverage"` step has its percentage
  read for display. The parser picks a *line* before it picks a number, and
  its fixtures are recorded from real tools.
- **#226 detect (PR #241).** omatty proposes a gate by reading the checkout,
  and only ever proposes. The Node branch reads `package.json` for *whether* a
  script exists and never what it contains.
- **#227 `Project.Gate` (PR #240).** `state.json` stays at version 1: a nil
  gate is "not configured yet", and the empty value is derivable
  (invariant 9), the argument `Agent` and `Base` already carry.
- **#228 `omatty gate` (PR #243).** Show, propose, set or clear. The plain
  form is the confirm-once flow `discover` and `adopt` already use.
- **#229 the Runner (PR #242).** Bounded parallelism, supersede, panic
  recovery. `testdata/gateprobe` is its real-binary half.
- **#230 the card strip (PR #245).** A third card line, always drawn.
  `READY` is derived from a green gate, a non-empty diff and a resting
  session - never stored.
- **#244 revealHeader (PR #245).** Found by an existing test when the card
  grew: showing a project header could push the selected card out of a short
  pane. Latent since #129.
- **#231 the gate pane (PR #246).** The review column's fourth mode, and
  running the gate from it. A mode rather than a fourth pane.
- **#232 feedback (PR #247).** `S` sends the failures into the session as one
  bracketed paste (invariant 8).
- **#233 auto-run (PR #249).** `[gate] auto`, off by default, gating a session
  when its turn ends. A red gate notifies only while omatty is blurred.
- **#248 builtins (PR #249).** Found by #233's own smoke test: `exit` is a
  shell builtin with no binary, so the `LookPath` pre-flight called a working
  step `Missing` and stopped the gate. The hand-written list of builtins is
  deleted in favour of asking the shell - `command -v` - because a list of
  builtins cannot be completed across shells.

**Deliberately cut, so they do not return sideways:**

- **Coverage overlaid on the diff** - marking added lines no test covers - and
  **test-file pairing flags**, which flag a diff that touches source and no
  tests. Cut from M9 because both needed its coverage parsing first, and built
  immediately after as **M10**; the entry stays here because the deferral is
  the reason M10 exists. See the M10 section for what it did with them.
- **Reading the gate out of `AGENTS.md`'s fenced block.** One source of truth
  for human, agent and environment is the right long-term answer, but parsing
  prose is fragile; M9 uses explicit config and the docs say to keep the two
  in sync.
- **A gate that blocks anything.** `READY` is a badge, not a permission.
  Nothing in omatty refuses an action because a gate is red - the operator
  decides, and a tool that argued with them about it would be the thing this
  milestone exists not to build.
- **Sending anything without being asked.** #233 runs the gate for you;
  nothing sends a prompt on your behalf. That line is where a verification
  tool becomes an orchestrator.

---

## M10 - Coverage on the diff

**Delivers:** the milestone in one glance - of the lines this session added,
which are not exercised by anything? omatty reads the profile the project's own
gate just wrote, marks the added lines no test covers, and says a word when a
change brought no tests with it at all.

**Why this, and why it needed M9 first.** M9 put a gate beside each session and
a percentage on its card. A percentage is a number about a repository; it does
not say which of *these* lines, the ones that came back five minutes ago, are
untested. That is the question a reviewer actually has, and answering it is the
whole bet of this tool: not how much agent-work can be in flight, but how fast
a person can tell whether what came back is any good. Both halves were cut from
M9 explicitly and for the same reason - they needed a coverage profile parsed
at line granularity, which is what #251 and #252 built.

**What it is, in seven slices**, one issue and one PR each:

- **#251 `internal/coverage` (PR #259).** A Go profile as per-line verdicts.
  Three states, and the third earns the package its shape: covered, uncovered,
  and *no verdict at all* for a line that is not a statement. An absent line is
  silence, never a claim, because marking braces and declarations would be
  noise that trains the eye to ignore the marker.
- **#252 lcov and sniffing (PR #270).** The other format, and picking the
  parser by looking at the file rather than at its name: `.info`, `.lcov`,
  `.out` and `.txt` are each in use for *both* formats. A file that is neither
  is an error, not an empty profile - an empty profile would quietly mean
  "nothing here is uncovered", which is a lie the operator cannot notice.
- **#253 `Step.Profile` (PR #273).** A coverage step *declares* the path it
  writes. `omitempty`, so `state.json` stays at version 1 (invariant 9).
  `Detect` proposes the ecosystem's convention - `cover.out`, `lcov.info`,
  `coverage/lcov.info` - which is a fine thing to propose and confirm, and a
  poor thing to assume at read time, which is exactly what #248 punished.
- **#254 loading it (PR #274).** When a gate finishes, its profile is read out
  of the session's own directory - a worktree session reads its own worktree,
  so two sessions never read each other's numbers. Display-only, never
  persisted. A profile that will not parse **leaves the previous overlay**
  rather than blanking it: blanking would quietly claim nothing is uncovered.
- **#255 the markers (PR #275).** An added line the overlay says never ran
  draws `!` where its `+` would be, and the file header carries the count -
  `internal/ui/model.go +2 -1  3 uncovered` - so a long diff says where to look
  without being scrolled. The marker takes the sign's cell rather than adding a
  gutter column, because a new column would shift every row of every diff,
  including files the profile says nothing about.
- **#256 `review.Pair` (PR #276).** The cheaper half, needing no coverage data:
  four outcomes, not two. **Rust is the interesting case** - it puts tests in
  the same file behind `#[cfg(test)]`, so a `.rs` hunk that adds one is paired
  and one that does not is *Unknown*, never Unpaired. A flag that cried wolf on
  every Rust change would teach the eye to skip it, and then it would be worth
  nothing on the languages where it was right.
- **#257 the flag (PR #277).** `diff · 3 files · 0 comments · ⚠ no tests`, and
  only for Unpaired. Three quarters of the outcomes are silence, which is what
  keeps the fourth worth reading.

**Deliberately cut, so they do not return sideways:**

- **A gate on any of it.** Nothing is blocked, nothing turns red, `S` sends no
  more than it did. This is M9's line held one milestone later: the overlay and
  the flag are remarks, and a tool that argued with the operator about them
  would be the thing the roadmap exists not to build. A flag is exactly the
  kind of thing that grows teeth later, so #257 ships with a test whose only
  job is that it has none.
- **Python's Cobertura XML.** `Detect` proposes *no* profile for a Python
  project, because `internal/coverage` reads Go profiles and lcov and nothing
  else. "No overlay" is legible; a path that never parses is an overlay that
  never arrives and never says why.
- **Typing a profile path at the command line.** `omatty gate --set` writes the
  proposal; a project whose profile lives somewhere else is a `state.json`
  edit. The confirm-once flow is about agreeing to what was *proposed*, and a
  free-text path is a different act.
- **An overlay that refreshes itself.** It describes the tree as the gate found
  it and goes stale the moment the session edits again - which is exactly the
  freshness the diff and the diffstat beside it already have. Watching the file
  would buy a fresher wrong answer, since the profile is only true just after
  the run that wrote it.
- **Marking context and removed lines**, and a total in the title. A context
  line's coverage is not this change's business, a removed line is not in the
  tree the profile describes, and a per-file count already says where to look.
- **Branch coverage.** Both formats carry it and neither carries it the same
  way; line verdicts are what a diff can draw, and the second number would have
  to be explained every time it disagreed with the first.

---

## M11 - The Harness

**Delivers:** the gate stops trusting prose. Three rules this repository had
written down and nothing checked - invariant 4's import boundaries, the module's
own hygiene, and where untested code hides behind a repo-wide coverage average -
become steps that fail, and the package structure underneath them becomes a
number that is printed on every run.

**Why this, immediately after M9.** M9 built the machinery for running *a
project's* gate and showing the verdict beside the session that caused it. M11
turns the same instrument on omatty itself, and it did so because the
measurement kept finding the documents wrong. `AGENTS.md` said `ui` was the only
package importing bubbletea; `internal/termwrap` had imported it in four files
for weeks. The coverage gate was green at 90% with an *exported* function at 0%.
A rule nobody measures is a rule that has already drifted - that is the whole
argument of the milestone, and each slice is an instance of it.

**What it is, in four slices**, one issue and one PR each:

- **#260 depguard (PR #264).** Invariant 4's import half, enforced by a linter
  that already ships inside the pinned golangci-lint - zero new tooling, zero
  new CI steps. Four rules: bubbleterm and `creack/pty` to `internal/termwrap`;
  bubbletea, bubbles and lipgloss to `ui` and `termwrap`; chroma to
  `internal/highlight` and go-gitdiff to `internal/review`; `os/exec` to the six
  packages that genuinely run one, with `!$test`. Every importer set was
  *measured* rather than assumed, which is how the `termwrap` exception was
  found and `AGENTS.md` corrected. Invariant 4's git half is a string literal,
  not an import, so depguard structurally cannot see it and it stays a grep test
  (`TestNoGitOutsideVcs`).
- **#261 module hygiene (PR #265).** `go mod tidy -diff` after `vet`, and
  `govulncheck` pinned by `GOVULN_VERSION` the way the linter already is. These
  are the first steps of the gate that need the network, and the gate block in
  `AGENTS.md` says so, because somebody will run it on a plane. The pin earns
  itself immediately: `govulncheck` reports against the toolchain doing the
  analysis rather than against `go.mod`, so the same tree was red locally and
  green on CI until the Go version was pinned exactly.
- **#262 C.R.A.P. (PR #266).** `CC² × (1 − cov)³ + CC`, scored per *function*,
  because a repo-wide coverage average is exactly the place untested code hides:
  `watcher.PromptText` is exported and at 0% inside a package measuring 92.2%.
  The threshold is **12, not the canonical 30** - `gocyclo` is capped at 10 here
  and a CC=10 function at the 90% floor scores 10.1, so a CRAP-30 gate would be
  vacuous rather than merely slack. It shipped at 15 and ratcheted to 12 in
  #267, below. Coverage blocks are attributed to
  `*ast.FuncDecl` extents rather than joined against `go tool cover -func` text,
  which prints methods without receivers (`Close` appears nine times) and
  reports zero-statement functions as 0.0%.
- **#263 the dependency structure (PR #268).** Ca, Ce, instability
  `I = Ce/(Ca+Ce)`, abstractness and distance per package, printed on every run.
  Two things are *enforced*: cycles through the **test** graph - the compiler
  already refuses production cycles, but `a_test -> b -> a` compiles happily and
  couples two packages in a direction their production code never admits to -
  and the Stable Dependencies Principle, which landed behind `--sdp` and was
  enforced in #269 below. The universe is `./internal/...` only; adding
  `cmd/omatty` was measured and found *worse*, raising Ca on thirteen packages
  and putting two edges at exactly zero margin.

**Deliberately left, each with a reason rather than an omission:**

- **The C.R.A.P. ratchet to 12** was left to #267 and **done 2026-09-16**
  (PR #279). The gate shipped at 15 because that was the lowest value green at
  the time, and the two functions holding it there - `PromptText` and
  `typedText`, both CC=3 at 0%, scoring exactly 12.0 - had to be *tested*
  before the threshold could move, or the ratchet would have been a red CI and
  nothing else. `PromptText` is the one copy of what "the operator typed this"
  means, read by both the tailer and discover, and it was exported for that
  reason and then never tested: precisely the hole #262 built the gate to find.
  With both covered the worst score in the tree is 8.2, so 12 landed with a
  margin of nearly four.
- **SDP as a failure rather than a report** was left to #269 and **done
  2026-09-16** (PR #280). `I` is a ratio of small integers and moves in jumps -
  `internal/config` is Ca=1 Ce=1, and a single new importer would take it from
  0.50 to 0.33 - so a gate failing on a margin nobody had watched move would be
  one people learn to `--no-verify` past. The margin was watched instead: across
  every merge from #263 to #278 the tightest edge stayed `watcher -> registry`
  at exactly +0.071, through M10's `ui -> coverage` edge taking the graph from
  35 to 36. The flag is gone and a violation fails the run. Had the margin
  oscillated, that would have been a finding rather than a failure - it did not.
- **Gating on distance from the main sequence.** Eight packages sit at D = 1.00,
  and that is what Go looks like rather than a defect: interfaces are declared
  at the consumer and often unexported, so a stable pure leaf like
  `internal/paths` (Ca=7, Ce=0 - exactly as designed) scores maximum distance.
  Gating on D would demand precisely the speculative interfaces `AGENTS.md`
  bans. The table is in `docs/ARCHITECTURE.md` with the paragraph explaining
  why, so nobody "fixes" it later.
- **Mutation testing and module size limits.** Named in the same argument that
  produced this milestone and cut from it: a mutation run costs minutes per
  package, and nothing yet says what omatty would do with the score. Cut for
  cost, not for principle.

---

## Releases

`main` is the branch a stranger clones; `develop` is where milestones land.
For the first nine days of this repository those were different pieces of
software - eight milestones on `develop`, the bootstrap commit on `main` -
because no milestone ever said what happens after "merged to develop". #134
was that gap, and this section is the answer to it.

**A release is a promotion, and a promotion is a pull request** from
`develop` to `main`, merged with a merge commit. Not a fast-forward: the
merge commit is the record of what was promoted and when, and it is what the
tag points at. Never a force-push.

**The gate is the milestone gate, once more.** The full CI gate green on both
runners, plus the real-binary smoke test rule 2 requires - a scratch `HOME`,
`testdata/fake-claude` on PATH, read by a person. Nothing new is asked of a
release, deliberately: the promotion is the last cheap point to catch a
wiring failure the coverage gate cannot see, which is the whole argument of
rule 2, and inventing a separate release checklist would only be a second
thing to let rot.

**Before the merge** the PR updates `CHANGELOG.md` and README's Status
section. **After it** the merge commit is tagged `vMAJOR.MINOR.PATCH`, and the
tag is the release: `release.yml` re-runs the gate on it, then GoReleaser
publishes binaries, checksums, the Homebrew cask and the notes from the tag's
own CHANGELOG section (#327). A pull request already checks the release
configuration and builds it as a snapshot, so a tag cannot be the first thing
to find it broken.

**When a release happens: at every milestone close** (#329). This section and
AGENTS.md said *how* for three weeks and never *when*, and with "no version
bump and no release tag without explicit approval" standing over them, the
default was that nothing shipped - v0.1.0 took nine days of finished
milestones to arrive, and twelve days after it five more were waiting on
`develop` (#328). Merges already ran on a standing approval granted per
milestone; releases had no equivalent, so each one depended on somebody
remembering to ask.

So closing a milestone now *includes* its promotion pull request and its tag,
under the same standing approval as that milestone's merges. The release is
part of finishing the work.

**A floor, not a ceiling.** A milestone close must produce a release; a release
does not require one. v0.1.0, v0.2.0 and v0.5.0 went out at milestone closes;
v0.3.0 and v0.4.0 went out in the middle of M12, because a verification core
and a close-out were each worth having on `main` before the milestone around
them finished. Both stay allowed and both stay asked about.

**The checks did not move.** The gate above is exactly the same gate - CI green
on both runners plus the smoke test a person reads. #329 was a question about
the trigger, and the reason to settle it in writing is that a decision made by
default, once per release, is the one that quietly stops being made at all.

Below 1.0 the `ctrl+o` key table, `~/.omatty/config.toml` keys and the
`state.json` schema are explicitly not frozen; the embedded terminal library
underneath is itself pre-1.0 (invariant 4). A break in any of them is a minor
bump. v1.0.0 is the claim that those three have settled, and nothing here is
in a hurry to make it.

| Release | Date | Contents |
|---|---|---|
| v0.1.0 | 2026-09-10 | M1-M8, all eight milestones. 307 commits. (#134) |
| v0.2.0 | 2026-09-22 | M9, M10, M11, M13, the session lifecycle (#316-#319, #321) and M12's research. 122 commits. (#328) |
| v0.3.0 | 2026-09-25 | M12's verification core (#311, #310, #335, #342), the release pipeline (#327), the MIT license (#362). The first release built by `release.yml`. (#364) |
| v0.4.0 | 2026-09-25 | M12's close-out: `omatty carry` (#309), the pane's own text selection (#360), the hook path that survives a reinstall (#380), and the defects v0.3.0 surfaced. 19 issues. (#390) |
| v0.5.0 | 2026-09-26 | M14 The Tracker: a project's open issues and pull requests in the review column, read through the operator's own `gh` and never written to (#393-#399). 15 commits. (#407) |

## M12 - The Field

**Delivers:** an answer to the question the first eleven milestones assumed.
`README.md` claimed that "every other tool in this space is either a desktop
app or scoped to a single repository". M9's thesis put omatty in "a field of
roughly a hundred and fifty agent orchestrators". Seven features were refused
in "Not on the roadmap" against a field this repository never named. Nothing
here had ever been checked against a live page.

**Why this and not something else.** Every refusal above is worth exactly the
evidence behind it, and there was none. The method is `akitaonrails/ai-memory`'s
- the same artifact set one project-stage later: a dated landscape survey,
per-competitor deep dives, tracker mining, a prior-art ledger, a self-critical
parity audit, and a public comparison. Its rules came with it: every
load-bearing claim checked against a live primary page rather than remembered,
vendor "alternatives" pages refused as sources and named, and research
documents that make no implementation decisions - the roadmap makes those, in
the open, which is this section.

**The research, in eight slices**, one issue and one PR each: #295 and #298 the
field inventory and its `R1-R9` recommendations, #296 four deep dives read at
code level, #297 five tracker minings, #299 the ledger at P0/P1/P2, #300 the
parity audit, #301 `docs/comparison.md` and this section, #302 the method
captured as a skill. All of it is in `docs/research/`, captured 2026-09-18.

**What it found, and none of it was expected:**

- **The first party shipped the category's core.** `claude agents`, in research
  preview, is "one screen for all your background sessions", each isolated
  "into an isolated git worktree under `.claude/worktrees/`". That is M1's
  skeleton and M2's status, free and in the box. It is M9's bet paying off -
  parallelism was never the scarce thing - and it cost the sentence `README.md`
  opened with, which #301 rewrote.
- **`README.md`'s field claim was false.** `kbwo/ccmanager` is a terminal tool
  with a documented Multi-Project Mode and recursive repository discovery;
  `brizzai/fleet` groups sessions by repo in a Go TUI. Rewritten in #301.
- **The gate is still nobody else's feature.** Across every README read - the
  terminal camp, the desktop camp, the board camp - no tool runs the project's
  own check line per session and shows the verdict beside it. That square is
  empty, and M9 and M10 are standing in it.
- **Invariant 2 was right, and now has evidence instead of an argument.**
  `claude-squad` matches literal English UI strings in a captured tmux pane and
  `ccmanager` regex-matches Claude's drawn prompt box; between them that is the
  field's dominant bug class, including `claude-squad`'s most-discussed issue.
  `fleet`, which reads hooks, has one status bug.
- **The moat is narrower than the pitch was.** One feature and one property -
  the gate, and content-anchored comments - scoped to several sessions across
  several repositories. Two of the seven lines in the moat ledger are parity
  rather than advantage. `competitive-parity.md` deleted three more for not
  being true, and answers "is this just lazygit and a CI badge?" with a
  concession before an argument.
- **#152 was wrong about Codex.** `fleet` ships Codex hooks - `SessionStart`,
  `UserPromptSubmit`, `PermissionRequest`, `Stop` - at `~/.codex/hooks.json`,
  in the same event-map shape as Claude's. The correction is posted on #152.

**What it builds.** Three issues, all inside the thesis rather than beside it,
and the research deliberately stops before deciding their shape:

- **#309 a worktree you can actually run.** The field's most-repeated unmet
  need - `ccmanager` #7 shipped it, `claude-squad` #260 and #277 are open
  asking, `fleet` shipped its own - and omatty has no answer. It is P0 because
  M9 made the gate the product: a gate step that fails because `.env` is
  missing is the gate being wrong about the code, and a red card the operator
  learns to ignore is worse than no card.

  **Built** as `omatty carry <project> <path>...`, with the list in
  `state.json` as `Project.Carry` beside `Project.Gate`. The design question
  the issue owed an answer to is settled against the two competitors: they put
  the list in a repository file (`.worktreeinclude`, `.fleet.json`), omatty
  keeps it per project in `state.json`, because that is how every other
  per-project setting here works, the empty value is derivable so `Version`
  stays 1 (invariant 9), and a clone must not get to choose which files are
  copied off the operator's disk. The cost, named rather than hidden: the list
  is not shared with a team. The copy runs before the session is registered,
  so anything that starts next can rely on the files - the ordering ccmanager
  arrived at deliberately.
- **#310 PR and CI state on the session card.** The field asks for the remote
  verdict (Orca #18484, #18485, #18487; `fleet` ships it). omatty has the local
  one. They are complements, and this is verification rather than
  orchestration - the card reports a verdict someone else computed and acts on
  nothing.
- **#311 a review scoped to *since when*.** Orca #11840 asks for "changes this
  turn" / "since my last review", backed by "a refs/orca baseline snapped at
  agent turn boundaries" - a mechanism omatty has the parts for, since the
  `Stop` hook already marks the boundary. omatty shows everything a
  session changed, so a reviewer re-reads three turns on the third turn. This
  is the only gap the pass found *in the thesis itself*.

**Taken back** (2026-09-25, #379). The P1/P2 list was first cut here as
real, small, and none of it the reason to open the tool. Two of its items
then shipped anyway - a comment that knows it was sent (#335) and Homebrew
(#327) - and the rest was filed with M12's label while this section still
argued against it. Rather than keep a milestone whose roadmap refuses its own
issues, M12 took them back:

- **#336** scrollback survives a dtach reattach (#191's remainder).
- **#337** per-file reviewed, and changed since reviewed.
- **#338** generated files collapsed in the review tree.
- **#339** several comments per line, and comments on part of a line.
- **#331** ship a green session from its card, **#332** lead time and
  first-pass gate rate, **#334** revert a session to the start of its last
  turn, and **#333** an LLM audit as a gate step - each built on the
  verification core v0.3.0 shipped (#310's remote verdict, #311's per-turn
  baseline, the gate).

They are still not the reason to open the tool. They are what the reason
needs once it is there, and `prior-art-findings.md` keeps their priorities.

**Added** (2026-09-26, #410). The field signals "working" with one small
moving mark - claude's own `✻`, a braille spinner in the agent TUIs, a busy
dot in ccmanager and claude-squad - and none of them draws a history. omatty's
activity lane (#128) did: six block cells whose height repeated line one's
glyph and made every busy card the same grey wall. Line one's glyph now
spins through braille frames at 100 ms while a session thinks or runs a
tool, one spinner for both; a session with no process keeps its still glyph.
The lane is gone and its seven columns went to the branch, 16 to 23. The
frames come from a spin tick of their own, armed by whichever message makes
a session spin and stopped when nothing does (#412: speeding up the
heartbeat instead left the glyph still for up to a second), so an idle
omatty ticks once a second as M13 left it.

**Deliberately cut:**

- **Widening the agent seam to match `ccmanager`'s eight.** #152 stays the
  scope. `ccmanager`'s #82 and #107 are what each added profile costs: an
  escape-key bug per agent.
- **A full read of Orca's tracker.** 3,036 open issues; #297 probed it against
  omatty's own design and its document says plainly that a probe cannot support
  a claim about what its users complain about most. Nimbalyst and vibe-kanban
  were not mined at all. That is where a second pass starts.

---

## M13 - Memory and idle CPU

**Delivers:** an answer to "why is omatty holding most of a gigabyte", and
the part of that answer omatty owns. Measured on a live window: thirteen
sessions, three days up, **505 MB physical footprint** with a further 298 MB
swapped, RSS sawtoothing **183 MB to 364 MB every two seconds**, and
**12-17% of a core while nothing was happening**.

**Why this and not something else.** Twelve milestones shipped without a
single measurement of what the binary costs to leave open, which is the one
thing a tool you leave open all day is judged on. The sawtooth said the
answer was two separate problems - roughly 183 MB genuinely retained, and
roughly 180 MB of garbage produced every two seconds - and that split is
what the work below follows.

**What was fixed.**

- **The frame was rebuilt on every message, and measured three times over.**
  A CPU profile of the idle process named `ansi.StringWidth` as 62% of a
  frame: `fitLine` measured a line to decide whether to cut it and `padRight`
  measured it again to pad it, then `lipgloss.JoinVertical` and
  `JoinHorizontal` measured every line of every column a third time to align
  blocks that are already exactly their own width - the #174 promise
  `TestFrame_EveryLineIsExactlyTheWindow_issue174` has asserted all along.
  `joinRows` and `joinColumns` trade on that invariant instead of
  rediscovering it. Lane cells, meter cells and status glyphs were restyled
  per cell per frame although each is fixed by a tiny domain, and are now
  rendered once. `BenchmarkFrame` at thirteen sessions: 385 us and 464 allocs
  to 135 us and 386.
- **The frame is memoised, invalidating by default.** Most messages are
  output from a pane nobody is looking at, and such a frame is identical to
  the one before it - status comes from the transcript, never the grid
  (invariant 2). The memo is keyed on the window and the focused pane's
  grid; `Update` drops it for every message and exactly one path puts it
  back, the broadcast that mutates a terminal and never the model. A message
  type added later is stale-proof by default, and eighteen of them are
  asserted against a fresh rebuild.
- **Archive forgot five of fifteen per-session maps.** The rest stayed for
  the life of the process, most expensively `covers`, a `map[int]bool` per
  source line per file. The guard is a reflection test, not a written list.
- **Gate output was bounded only after `CombinedOutput` had held all of it**,
  which is the failure `internal/gate/bound.go` already described, one layer
  earlier than it was being fixed.
- **Every checkout was polled while the window was blurred**, two or three
  git processes per session every ten seconds.
- **`OMATTY_HEAP_PROFILE`**, because none of the above could be asked of the
  binary; it all had to be inferred from `vmmap`, `ps` and a new benchmark.

**Measured end to end**, eight chatty sessions through a real PTY:
**35-41% of a core to 16-18%**, peak RSS 153-155 MB to 135-137 MB.

**Not fixed, and not omatty's to fix.** The retained heap is dominated by two
upstream constants, which `OMATTY_HEAP_PROFILE` showed on its first run - on
a **two**-session window, 61% of the live heap was one of them:

```
8194.75kB 61.48%  github.com/charmbracelet/x/ansi.(*Parser).SetDataSize
1034.66kB  7.76%  github.com/charmbracelet/ultraviolet.NewBuffer
```

- `x/vt`'s `NewEmulator` calls `SetDataSize(1024 * 1024 * 4)` - **4 MiB of
  ANSI parser buffer per emulator**, 64x the `ansi.NewParser` default,
  allocated whether or not the session ever draws. At thirteen sessions that
  is 52 MB before a byte of anything else.
- Each emulator holds **two screens, each with a 10,000-line scrollback**
  (`vt.DefaultScrollbackSize`), of which omatty reads *nothing*: the wheel
  forwards `PgUp`/`PgDn` to the child so claude's own pager answers
  (`internal/ui/wheel.go`), and the repository contains no reference to
  `Scrollback`, `GetCells` or `CellAt`. A `uv.Cell` is 112 bytes with five
  pointer words, so a saturated lane of scrollback is tens of MB per session
  that nothing can display.

`vt.Emulator` exports `SetScrollbackSize`, but `bubbleterm.Emulator` holds
its `vt` field unexported with no accessor, so none of this is reachable from
here. Fixing it means a passthrough in `taigrr/bubbleterm` and a smaller
default - or an argument for one - in `charmbracelet/x`. That is an upstream
conversation, not a change to this repository, and invariant 4 is what keeps
its blast radius to `internal/termwrap` when it happens.

---

## M14 - The Tracker

**Delivers:** the question every session starts from, answered without leaving
the window. `ctrl+o i` shows a project's open issues and open pull requests,
`enter` reads one in full, and `n` turns the one you picked into a session named
and branched after it.

**Why here.** M12 gave a card its pull request and its CI (#310). The rest of
the forge was still a browser tab: how many issues are open, what they are
about, which one to start next. That is the last daily-friction item left that
does not need a new subsystem - `internal/forge` already owns `gh`, the review
column already has four faces, and the sidebar already has a row per project
doing nothing but naming it.

**Why it is not the planning board this roadmap refuses.** "A planning board
inside the TUI" and "a board driving the agents" are both refused under "Not on
the roadmap", and both are argued from *two sources of truth*. M14 stores
nothing: no issue state, no `state.json` field, no reconciliation. The rows are
derived at render time from the last poll and are gone when omatty exits, which
is the same standing a card's pull request has had since #310. It is a **window
onto GitHub's board**, not a second one, and the distinction is exactly the one
#310 already won for pull requests: reading the forge with the operator's own
`gh`, on their own authentication, holding no token and syncing nothing.

Two things keep that honest, and both are refusals:

- **No forge writes.** No create, no comment, no close, no label, no assign.
  Those are the ones that would need a board of their own to make sense, and
  they are out. The keys that act - `n`, `a`, `b` - act on *this machine*: a
  worktree, a composer, a browser.
- **No prompt is ever sent.** `a` pastes the item's reference with no carriage
  return (invariant 8), so the turn is the operator's to start. `n` hands over
  the keyboard and types nothing. There is still no mode in which omatty works
  while nobody is reading.

**What it is, in seven slices**, one issue and one PR each:

- **#393 `forge.ListIssues` (PR #400).** `Issue` and `FoldIssues`; one
  `gh issue list --state open --limit 100` call per project, #358's rule applied
  to the other list. `openFields` gains `title`, `isDraft` and `updatedAt` -
  cheap fields on a call already being made. Both lists share one runner and one
  context, so #356's bound stays "an answer inside thirty seconds" rather than
  thirty per half. Plus `testdata/forgeprobe`, the `dtachprobe`/`gateprobe`
  argument applied to this seam: the tests fold recorded JSON, which proves the
  fold and nothing about gh.
- **#394 the poll (PR #401).** `internal/ui/issuepoll.go`, every registered
  project and not only those holding a session, on a five-minute tick where
  `prEvery` is one - CI changes in minutes, an issue list in days. Everything
  the two polls share goes through one `mayAsk`, which is also what keeps `dupl`
  quiet and what stops them drifting apart. `prOff` became `notGitHub`: it now
  gates two lists.
- **#395 the counts (PR #402).** `13i 2p` on each project's sidebar header,
  right-aligned and muted. Unknown is never drawn as zero - no gh, not GitHub,
  not yet polled leaves the header byte-for-byte as it was - and the counts come
  off before the name does, fifteen cells being what #155 settled still
  identifies a row.
- **#396 the view (PR #403).** `ViewTracker`, the fifth face of the review
  column. Its opener is deliberately not `toggleView`, which returns early with
  no session selected: this view belongs to a project, so it opens on one with
  no sessions at all (#158). Its state is keyed by project, so following the
  sidebar between two sessions of one project keeps the list and the cursor.
- **#397 the item (PR #404).** `forge.ViewIssue`/`ViewPR` and
  `ViewTrackerItem`, the list's child in `keptView`. One call on `enter`, cached
  until `r`, bounded at 64 KiB with whole comments dropped rather than cut, and
  every line wrapped to the column.
- **#398 working from it (PR #405).** `n`, `a`, `b`. The branch is passed at
  creation because `git worktree add -b` bakes it in (#151), and slugged because
  an issue title is untrusted text on its way to a ref and a path (#127).
- **#399 the filter and the docs (PR #406).** `/` over the number, the title and
  the labels through `internal/fuzzy`; the marker never leaves the title (#285).
  README's key table and "Issues and pull requests" section, this section, and
  `internal/forge` in `docs/ARCHITECTURE.md`'s package table, where #310 had
  never added it.

**Three things worth remembering from building it:**

- **The real-PTY run found two defects no unit test could.** The rule between
  the two lists panned away with the rows, leaving them merged with nothing to
  say where the issues stopped - it is a label, so it fits its own column now,
  the way a diff's file header does (#291). And `testdata/fake-gh` answered both
  `gh pr list` calls from one file, so every open pull request was listed twice
  and every count doubled. Rule 2 earning its place for the fourth milestone
  running.
- **A shared fact belongs to both lists, a preference to neither.** `gh` missing
  and a checkout that is not on GitHub are facts about the machine and the
  checkout, so either poll finding one stops both; what is in flight and when it
  was last asked are each list's own. Getting that split wrong silenced a poll
  under test, which is how it was found.
- **Wrapping catches its own bugs.** The truncation notice went in unwrapped and
  was cut at the column edge in a view where every other line wraps. In a wrapped
  view, an unwrapped line reads as a rendering bug - which is the useful part: it
  shows.

**Done when:** `ctrl+o i` answers "how many, and what are they about" for every
registered project without leaving the window, and `n` on an issue puts you in a
session named after it. Both verified in a real PTY at 120x32 and 80x24, plus a
project holding no sessions.

**Deliberately out:** every forge write (above); closed issues, which are
history the forge already keeps; search across repositories; and a `[forge]`
config section - the poll is zero-config, as #310's is.

*Amended by M16 (#451):* `[forge.hosts]` exists only to name a self-hosted
forge omatty cannot recognise by its host. The poll stays zero-config for
every host omatty already knows.

## M15 - The Polish

**Delivers:** the review column's four faces and the help modal read as one
product. The same cursor, the same keys, the same glyph and colour for the same
state, the face's name in its own chrome, and room to read when you ask for it.
Design: `docs/superpowers/specs/2026-09-26-omatty-m15-polish-design.md` (#420).

**Why here.** Every face was built in its own milestone - the diff in M3, the
tree in M5, the gate in M9, the tracker in M14 - and each chose its own
conventions. By M14 there were four cursor styles, three glyph sets for state,
a gate that drew its verdicts without colour, a column rule that said `review`
on every face, and a help list that had been stale since M9. None of these is a
feature gap. Together they make a daily tool feel assembled, and at five faces
the cost of learning each one separately is paid every day.

**Mapping the column found three bugs**, which land first:

- **#421:** the gate view cannot scroll. `GateOffset` is only ever set to 0.
- **#422:** help omits the gate's and the tracker's keys. The fix derives help
  from the handlers' key tables so the two cannot drift again.
- **#423:** the tracker's age is appended after the title instead of pinned to
  the right edge, so it is off-screen on most rows.

**The foundation**, which the other slices build on:

- **#424:** one list window with one cursor style, `N/M` in the title, and
  `g`/`G`/`ctrl+d`/`ctrl+u` and click-to-select on every face.
- **#425:** one state vocabulary (a glyph and a colour per state), with Nerd
  Font glyphs opt-in behind `[ui] icons = "nerd"`.
- **#426:** the chrome names the face, and titles and footers shorten by
  priority.
- **#427:** `ctrl+o z` zooms the column over the pane.

**Then each face** borrows from the tool that does it best:

- **The gate reads like a CI check page:** summary counts in the title, the
  first failure opened on arrival, durations; `r` re-runs and `/` searches the
  opened output (#428, #429).
- **The tree:** compact single-child folders, status rolled up to directories,
  changed-only, and opt-in file-type icons (#430, #431).
- **The tracker:** state, CI and review glyph columns; an item that reads like
  a page; a preview beside the list when zoomed (#432-#434).
- **The diff:** syntax and word-level highlighting, `]`/`[` between files and
  `n`/`N` between hunks, and a file list when zoomed (#435-#437).
- **Help:** opens on the face you are in, styled and filterable, and the footer
  lists the next keys while the leader is armed (#438, #439).

**Two earlier cuts move.** M5 and M8 refused Nerd Font glyphs. They come back
**opt-in only**: a tofu box in a terminal without the font is worse than no
icon, which is why the default stays plain Unicode. A second colour theme stays
cut. M5 deferred diff highlighting until `internal/highlight` existed. It
exists, and #435 is the caller.

**Why zoom and not a resizable split.** A split needs a persisted width, a way
to change it, and a layout that is right at every width. Zoom is one flag and
two layouts that already exist. The views that need room (#434, #437) need it
sometimes, and zoom is how you ask for it. Zoom is a viewing state and is not
persisted (invariant 9).

**Invariants held.** Every new key exists only while the column has focus, and
the leader hints (#439) change what the footer *shows*, not what the router
routes (invariant 1). Gate search and wrapping are display; the verdict is
still the exit code (invariant 12).

**Done when:** every face scrolls, moves and marks state the same way, names
itself, and fits at 80x24. Zoomed at 200x50, the tracker and the diff show their
second pane. The smoke run is read by a person at both sizes, and it includes
`ctrl+o q` from inside a filtered help modal (M4's trap).

**Deliberately out:**

- A side-by-side diff: 160 columns to earn it, and a second copy of every diff
  path.
- A resizable column (above).
- A second colour theme.
- A command palette: #438's filter answers "what was that key", and fleet's
  palette is its own bug source (fleet #139).
- gh-dash's saved-search sections, which would configure a view M14 kept
  zero-config.
- Moved-code colouring.

#337, #338 and #339 stay M12. #436 and #437 are built so as not to preclude
#337.

**What shipped**, one PR per issue unless noted, all merged 2026-09-26:

- **Bugs:** #421 (PR #446), #422 (#445), #423 (#444).
- **Foundation:** #424 with #447 (#467), #425 (#468), #426 (#469), #427 (#470).
- **Gate:** #428 (#476), #429 (#477).
- **Tree:** #430 (#478), #431 (#479).
- **Tracker:** #432 (#482), #483 (#484), #433 (#485), #434 (#486).
- **Diff:** #435 (#487), #436 (#488), #437 (#489).
- **Help:** #438 (#490), #439 (#491).

**Two bugs were found building it**, each with its own issue and regression test:

- **#447:** the gate never panned. `h`/`l` moved the `+N` marker in the title
  but not the rows, and #231's pan test passed because the marker alone changed
  the frame. Fixed inside #424, whose rows it was.
- **#483:** nothing forge folds from `gh` was stripped of control characters.
  Titles, bodies, comments and labels kept their `ESC` and `BEL`. A real-PTY run
  showed bubbletea's cell renderer absorbing the OSC 52 and `ESC[2J` an issue
  carried, so the clipboard exploit the issue first described does not reproduce
  end to end - corrected on the issue. The fix stands as defence in depth: text
  is plain at the edge, once, and `n` no longer writes a title's escapes into
  `state.json`.

**Where it differs from the plan:**

- **#429:** `r` during a run in flight says the gate is already running rather
  than superseding it; superseding is the Runner's (#229).
- **#430:** `c` is in help, not the footer, which #103's test holds under 80
  columns. A directory's letter now says the strongest change beneath it, which
  reverses M5's "a directory reads M" rule on purpose.
- **#424:** `g` became "top" on every face, so #338's generated-files toggle
  moved to `.`, the key lf, ranger, yazi and nnn use for hidden files.
- **#435:** a screen of Go diff rows costs ~12-15% more per frame than before
  (`BenchmarkDiffRows`, ~100 µs to ~115 µs), the ANSI-aware fit of coloured
  text; the lexer and the word diff are memoised per hunk.

**Worth remembering:**

- **Merge the combination, then gate it.** A second session merged M12 and M16
  work into `develop` throughout. Two PRs each green on CI failed only together:
  #331's help rows and #427's `z` pushed a legend out of a 50-row help window.
  Before merging a stack, merge current `develop` into its top branch and run the
  full gate there.
- **A test that passes before the code is not a test yet.** Three of M15's new
  tests passed on first run - #437's list test matched the diff's own headers,
  #434's debounce test had no row to skip - and each was rewritten until it
  failed without the change.
- **GitHub's mergeability goes stale after a merge.** A PR reported "merge
  conflicts" that a local merge did not have; merging `develop` into its branch
  and pushing made GitHub recompute. Move a card only after the merge reports
  success.

## M16 - The Forges

**Delivers:** every forge omatty's users are on, at the parity GitHub has
today. That means GitLab (gitlab.com and self-managed), Azure DevOps (Services
and Server), Gitea, Forgejo and Codeberg, and Bitbucket (Cloud and Data
Center). Each gets open PRs (MRs) with a CI rollup on the card, open issues
(work items) in the tracker, an item's body and comments, and browse.
Design: `docs/superpowers/specs/2026-09-26-omatty-m16-forges-design.md` (#448).

**Why here.** #310 and M14 made the forge part of the window, but only for
GitHub. Everyone else gets "not on GitHub" and a browser tab. The seam already
exists: the UI depends on four func types, not on `gh`. So widening
`internal/forge` behind a router touches one package and the wiring, not the
UI.

**Decided with the user:**

- **Transport is CLI first, REST fallback.** omatty runs `glab`, `az` or `tea`
  on the operator's own auth when it is installed. Otherwise it calls REST
  with a token read from the environment per call and never stored. Bitbucket
  has no official CLI, so it is REST only.
- **Boards are out**, GitHub Projects included. M16 is issues and PRs.
- **Every slice sits in Backlog.** M16 is designed, not scheduled.

**The foundation** comes first, and GitHub is the first backend:

- **#449:** a forge-neutral vocabulary: `MissingToolError`, `ErrNoForge`, and
  the PR/MR noun in the copy.
- **#450:** read the remote (`vcs.RemoteURL`) and name its forge from the
  host.
- **#451:** `[forge.hosts]` names self-hosted forges, which also covers
  GitHub Enterprise.
- **#452:** `forge.Router` dispatches per project to an unexported backend,
  and every forge CLI is fenced to `internal/forge`.
- **#453:** the REST transport: bounded bodies, redacted tokens, and a
  `net/http` fence. Labelled `invariant`.

**Then a backend per forge,** CLI then REST: GitLab (#454, #455), Azure DevOps
(#456, #457), Gitea/Forgejo (#458, #459), Bitbucket Cloud and Data Center (#460,
#461), and GitHub's own REST fallback (#462).

**Then the close-out:** a real probe per forge (#463), #331's ship actions on
every forge once #331 ships (#464), and a support matrix that claims only what
the probes showed (#465).

**Why one package and not one per forge.** A package per forge would add five
`os/exec` importers to an allowlist where adding one "is a decision", and
would make `ui` choose between five packages. One package with a backend file
per forge is how the agent seam grows (#46).

**Invariants held.** git stays in `vcs` and every forge CLI stays in `forge`
(invariant 4). Nothing is persisted, and a project's forge is derived from its
remote (invariant 9). M16 adds no forge write, and #464 only carries #331's
bounded three.

**Done when:** a project on each of the five forges shows its PRs with CI on
the card and its issues in `ctrl+o i`, through both the CLI and the REST path.
Each is verified by a real `forgeprobe` run read by a person (#463). Any forge
not probed is named as untested, not claimed.

**Deliberately out:** boards on every forge; Jira; forge writes beyond #331;
OAuth, device flow, or any login or token store in omatty; several remotes per
project; per-check CI detail; SourceHut, Gerrit, Phabricator and CodeCommit.

## Not on the roadmap

Considered and cut, so they do not creep back in through the side door.

### omatty is a window, not an orchestrator

This is the first entry because it is the one with the most pull on it. The
field omatty ships into is full of tools that delegate; each of the following
would be a reasonable-sounding step toward becoming one, and each is refused
for a stated reason rather than by omission.

| Refused | Why |
|---|---|
| A coordinator agent; agents that spawn agents | Success rates compound. At 90% a step, a ten-step delegated chain lands around 35%, and it removes the human at exactly the point where corrections are still cheap. Published measurements agree: most production agents run fewer than ten steps before someone intervenes. |
| A spec or plan approval gate before work may start | A spec precise enough to generate correct code is already a program, just written in prose — and the spec-code contract breaks at the first hotfix. Plans stay optional artifacts you may write; never a state the UI makes you pass through. |
| Unattended task queues, scheduled runs, PR or chat subscriptions | A session runs because a person started it. There is no mode in which omatty works while nobody is reading. |
| Cloud, accounts, sync | A project's accumulated knowledge is `AGENTS.md` and whatever memory tooling you use — on disk, in the repo, read by the agent *and* by you. Hidden context that only the tool can see is a regression, not a feature. |
| A planning board inside the TUI | The board is GitHub project 13. A second one inside omatty means two sources of truth and a reconciliation problem nobody asked for. M14's tracker is a *window* onto that board - it stores nothing and writes nothing - which is the distinction #310 already won for pull requests; see the M14 section. |
| Agent-to-agent messaging | Same reason as the coordinator. If two sessions need to agree on something, that is a conversation for the person watching both of them. |

**Each of these now has a live example, found by M12's research** (2026-09-18).
A refusal that names what it is refusing is harder to re-open by accident than
one argued from principle alone:

| Refused | Who ships it | Still refused because |
|---|---|---|
| Auto-accepting prompts | `claude-squad`'s `-y/--autoyes` (its #222, #151), and `ccmanager`'s experimental AI auto-approval | It bypasses Claude Code's own confirmations. `ccmanager`'s README argues against `claude-squad`'s version by name — "not recommended for safe operation" — which is a competitor making omatty's case. |
| Agents dispatching to each other | `fleet skill install` — an Agent Skill teaching agents `fleet wt` and `fleet send` | Well made, opt-in, and exactly the thing above. The option was available and taken by someone else; the reason for declining has not changed. |
| Agent runs triggered by CI | Orca #10131, "auto-run agents when PR checks fail" | One step past #233's auto-run, which gates a session when its turn ends and *sends nothing*. That step is where a verification tool becomes an orchestrator. |
| A board driving the agents | `vibe-kanban`, and a camp three times the size of the terminal camp | Two sources of truth. The board is GitHub project 13. Reading that board inside omatty is not driving anything: M14 shows it and acts only on the keypress in front of you, and it cannot write to it at all. |
| Mobile companions, cloud sessions, account sync | Orca, Nimbalyst | Hidden context only the tool can see is a regression. |
| Merging when the checks go green | GitHub's own auto-merge, and every CI service with a merge queue | The same step as Orca #10131 one row up, arrived at from the other side: it acts because a check changed, with nobody reading. #331's ship key merges only what is *already* green, on a keypress, and refuses otherwise. Auto-merge is a real feature and a reasonable thing to want - it belongs on the forge, which has it, not inside a tool whose whole claim is that it only ever acts while you are watching. |

**Reading a pull request's state is not "cloud, accounts, sync"** (#310).
omatty runs the operator's own `gh`, holds no token of its own, makes one call
per project and none while it is in the background. Acting on a pull request -
pushing, opening, merging - was a separate decision (#331), taken when it was
proposed rather than by this one. It has since been proposed, taken by the
paragraph below, and **built**: `ctrl+o p`, one session, one keypress. Reading
still happens on a timer; writing happens only when somebody presses that key.

**Acting on a pull request: decided 2026-09-25, built 2026-09-26 (#331).** It is
accepted, bounded to what a person asks for while reading:

- **Push the branch and open the pull request**, and **merge one whose local
  *and* remote verdicts are already green**. Otherwise do nothing and say which
  of the two is missing.
- **One keypress, one session, every time.** That is the same shape as `S`
  sending the gate's failures back, and it is the criterion this section
  actually applies - not whether the forge is touched, but whether omatty acts
  while nobody is reading.
- **Never** merging when checks *go* green, which is the row added to the table
  above. Never a force-push, never a branch deletion.
- **Never into a protected branch.** AGENTS.md makes `main` moveable only by a
  promotion pull request: "This applies to the repository owner too - that is
  the point of it." A ship key that could merge into `main` would route around
  omatty's own release gate, so the base is the project's base branch
  (`develop` here), and a protected target is a refusal like any other.

Using the operator's existing `git` remote and `gh` auth is not "cloud,
accounts, sync" - there is no account, no token and no sync. It is the
credential the operator already uses by hand, on a keypress they pressed.

*Amended by M16 (#453):* when a forge's CLI is absent, omatty may read a
token the operator already put in the environment (`GITLAB_TOKEN`,
`GH_TOKEN`, ...), per call. It still **stores** no token: nothing is written
to config, `state.json` or a log, there is no login and no account, and
nothing syncs. "Holds no token" above now reads "stores no token". The
refusal of cloud, accounts and sync is unchanged.

One idea found in the field is **not** refused, only unanswered: **forking a
session's conversation** (`fleet`'s `f`). Invariant 9 asks the first question —
what is the copy's uuid, and which transcript does it claim? `fleet` #142 and
#226 are both identity bugs, which suggests the question is the hard part and
the UI is not. If it is ever proposed, start there.

What is left after all that is the loop the rest of this roadmap builds: start
a session, read what it did, say what is wrong, run the gate, send the failures
back. Tests, review, and short iterations — ordinary engineering, applied to a
faster pair.

### Also cut

- Claude self-reviewing its own diff
- ~~Commit / push / PR from inside omatty~~ - **accepted 2026-09-25**, bounded,
  as "Acting on a pull request" above sets out. Listed here from M1 until then.
  Its dangerous half, merging when checks go green, is refused by name in the
  orchestrator table and did not come with it.
- Running N sessions on one task and comparing the results
- Broadcasting one prompt to several sessions
- SSH / remote sessions
- Attaching to a `claude` already running in another terminal. omatty renders
  a PTY it owns; there is no supported way to adopt one it does not. M6's
  dtach sockets cover the case that matters - omatty's own sessions surviving
  a quit. Not to be confused with M4's project discovery (#91), which reads
  the transcript store and registers nothing by itself.
- Themes beyond one, or keybinding customisation beyond the leader. M15's
  opt-in Nerd Font glyphs (#425) swap glyphs, not colours; they are not a
  second theme.

## Rules

1. **Every bug gets a failing test before the fix.** Procedure in AGENTS.md.
2. **Every milestone ends with a real-binary smoke test in a sized PTY.**
   M1's three worst bugs - sessions dying at start (#31), terminals never
   pumped (#33), sessions dying on restart (#36) - all passed a 92% coverage
   gate, because every test substituted a fake for claude. Coverage measures
   units; these were failures of the wiring between them. `testdata/`
   carries a PTY harness for this; using it is part of "done".
3. **A milestone is not done while a blocker is open.** M1 is the example.
4. **Invariants are argued, never assumed.** Anything touching the twelve in
   AGENTS.md says so in its commit message.
5. **A milestone ends on `develop`; a release ends on `main`.** The promotion
   is a PR clearing rule 2's gate, and it is tagged. See "Releases". #134 is
   what nine days without this rule cost. And closing a milestone *includes*
   that promotion and that tag (#329) - a milestone is not finished while its
   work is only on `develop`.
