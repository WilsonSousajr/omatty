# omatty M8 surface redesign

Written 2026-09-09, after M7 shipped and #128's three slices (lane, meter,
title-in-rule) were judged on screen.

## Context

omatty draws three rounded boxes side by side: the sidebar, the session pane
and, when open, the review column. Between any two of them stand two border
columns, `││`. Inside the middle box runs Claude Code, which draws its own
rounded box around its prompt. The result is a box in a box, and at 80
columns the four border columns are 5% of the width Claude has to work in.

Two references were read for this design, both desktop UIs for coding agents
with a strong visual identity:

- **monocode** (hardbeat920/monocode). Near-monochrome: its base palette is
  hue 240 at 0% saturation, so hierarchy is lightness, not colour. One soft
  blue accent for focus and selection. No four-sided frames; regions are a
  darker sidebar and one hairline. Session rows are cards: agent and age,
  bold title, branch and `+471 −8`.
- **ade** (arul28/ade, the `ade code` terminal client). An Ink TUI with a
  violet theme. Its theme file states a rule worth quoting: *blue is work
  happening, amber is your move and nothing else ever, green is finished
  and unseen, red is broken, violet is selection, grey is true but not
  actionable.* Every state also carries a text marker so the pane reads with
  colour off. Session cards are a fixed three lines, the selected one wears a
  rail `▎`, and row height is one function shared by the renderer and the
  mouse hit-test. A breadcrumb header, and a footer with facts on one side
  and key hints on the other.

This design takes monocode's surface and ade's anatomy. Flat columns and
hairlines from monocode. Cards, the rail, the breadcrumb, the facts footer
and the one-hue-one-meaning rule from ade. It takes nothing that needs data
omatty does not read (provider glyphs, plans, subagents) and nothing that
fights a terminal (background fills, boxed cards inside boxed panes).

The board calls this M8, issue #172. Three decisions were made before this
document was written and are not reopened here: direction A of three proposed
(flat columns with two-line cards, over one-line rows or a single joined
frame); the colour rule, which changes hues #128 shipped; and a per-session
git poll for branch and diffstat, which is new data.

## The screen

```
 projects · 2              │ omatty ▎ parser-fix · main · ● waiting 4m       ▰▰▰▰▰▱▱▱ 62% cached · 60.2k in / 62.6k out
───────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────
 omatty                    │
▎● parser-fix           4m │ ╭────────────────────────────────────────────────────────────────────────────────────────╮
▎  main      +12 −3 ▁▃▇█▅▂ │ │ > fix the parser so a path with a space still resolves                                 │
 ◐ review-comments     12m │ ╰────────────────────────────────────────────────────────────────────────────────────────╯
   main       +2 −0 ▂▂▆▆▁▁ │ ◐ Thinking…
 billing-api               │
 ● billing-webhook      1h │
   feat/hooks       ▁▁▁▁▁█ │
                           │
 ctrl+o q quit  ctrl+o ? keys  ctrl+o j/k switch  ctrl+o n new  ctrl+o d diff                     3 sessions · 2 waiting
```

Four bands: a header row, a rule, the body, the footer. Columns inside the
body are separated by one hairline `│`, which meets the rule at `┼`. There is
no bottom rule: the footer is muted text and Claude's own status line sits
above it, so a rule there would spend a row to separate two things already
distinct. The rounded box in the pane is Claude's, not omatty's.

## Geometry

Everything below is a constant or a function in `layout.go`, which the caret
(#106), the wheel (#107) and the click hit-test (#45) already derive from.

- `SidebarWidth` stays 28: 27 columns of content and its hairline. #155's
  title-budget argument was made at 28 and is not remade here.
- `headerRows` is 1 and `ruleRows` is 1, replacing `borderRows`. The pane's
  height is `height - headerRows - ruleRows - footerRows`: the same rows
  Claude has today, since the two border rows became the header and the
  rule.
- `borderCols` is gone. The pane's width is
  `width - SidebarWidth - ReviewWidth`. Claude gains two columns at every
  width, four when the review column is open.
- `PaneOrigin` is `(SidebarWidth, headerRows + ruleRows)`.
- `ReviewWidth` keeps its two-fifths rule and its floor. It is the column's
  outer width including its own left hairline, so its content is one column
  narrower than the number.
- The sidebar's hairline is column `SidebarWidth - 1`. It belongs to no one:
  `overSidebar` is `x < SidebarWidth - 1`, and a click on the hairline does
  nothing.
- `titleRows` and `sidebarHeaderRows` stay at 0 and keep their comments; the
  header row is above the body, not a row of any column.

The frame promise of #35 holds: no line of the frame exceeds the window.
`fitBlock` still sizes every column's body exactly, and the header row and
the footer are `fitLine`d to the window.

## Focus

One rule for the whole screen: **an accent vertical line stands on the left
edge of whatever owns the keyboard.**

- Terminal focused: the hairline between the sidebar and the pane is accent.
- Review column focused: the hairline between the pane and the review
  column is accent, and the sidebar's hairline is not.
- Modal open: the pane's hairline is accent, because the modal owns the
  keys, and the pane's header segment names the modal.
- The header segment of the column that owns the keys is ink and bold; the
  other segments are muted.

The sidebar cursor is the same idiom at row scale: the rail `▎` in the first
column of both lines of the selected card, or of the header of an empty
project the cursor rests on (#158). Nothing else on the screen is accent.
The sidebar itself never owns the keyboard (invariant 1: keys go to the
terminal, the review column or a modal), so it never has an accent hairline
of its own.

## Sidebar cards

Every session is exactly two lines. Every project header is one line. There
are no blank margins; the header's colour separates projects. Row height is
`rowHeight(Row) int`, one function, used by the window math and by the click
hit-test, so the two cannot drift (#45, #129).

Line one, 27 columns:

```
▎● parser-fix           4m
```

rail or space (1), status glyph (1), space (1), title, space (1), age
right-aligned in 4 columns, and one blank column before the hairline so
the age never touches it. `AgeString` returns at most `999d`; wider is
clipped. The title budget is 18 columns, up from 15 (#155).

Line two:

```
▎  main      +12 −3 ▁▃▇█▅▂
```

rail or space (1), two spaces, branch, diffstat right-aligned, space (1), the
six-cell lane (#155), and the same blank column before the hairline. The
branch and the diffstat share 16 columns: the
diffstat is drawn whole, `KString`-shortened above 999, and the branch is
clipped to what remains. When the tree is clean the diffstat is omitted and
the branch has the 16. When the branch is unknown the diffstat is omitted
too and the line is the lane alone, right-aligned. `+12` is green and `−3`
is red, the diff colours the review column already uses; a clean tree draws
nothing rather than `+0 −0`.

A project header is the name in muted, one space in from the rail column.
The selected empty header wears the rail.

Session titles are text; the selected card's title is ink and bold. The
lane is unchanged in shape and fade (#154).

`Sidebar.Window(lines)` returns the rows whose lines fit, keeping the whole
selected card visible, never half of it. `revealHeader` keeps its behaviour
in lines: moving up onto a project's first session shows the header too
(#129). `sidebarRowAt(winY)` walks the same placements to find the row under
a click; a click on either line of a card selects it.

## Colour and glyphs

The palette is eight ANSI-256 indices, kept as indices for the reason
`style.go` gives: bubbletea quantises, and the two ramps remain the only
truecolor on screen (#154).

| Role | Index | Approximate | Where |
|---|---|---|---|
| ink | 253 | `#dadada` | the focused header segment, the selected title |
| text | 250 | `#bcbcbc` | session titles, branches, working glyphs |
| muted | 245 | `#8a8a8a` | project headers, ages, counts, keymap, unfocused segments |
| hairline | 238 | `#444444` | every hairline and rule that is not focused |
| accent | 75 | `#5fafff` | the focused hairline, the rail, the modal's name |
| amber | 214 | `#ffaf00` | waiting, and nothing else |
| green | 78 | `#5fd787` | done, `+added` |
| red | 203 | `#ff5f5f` | error, `−removed` |

Indices 39, 240 and 196 leave the palette. 75 replaces 39 because monocode's
accent is hsl(211 92% 62%) and 75 is the nearest index that is not neon.

Status maps to one glyph and one colour, and the map is the only place a
status becomes a colour:

| Status | Glyph | Colour | Meaning |
|---|---|---|---|
| idle | `○` | muted | true, not actionable |
| thinking | `◐` | text | work is happening |
| tool | `◆` | text | work is happening |
| waiting | `●` | amber | your move |
| done | `✓` | green | finished, you have not looked |
| error | `✕` | red | it broke |
| exited | `∅` | muted | true, not actionable |

Working states earn no colour: working is the default, and the lane's height
already says how busy. Amber goes to waiting alone, the one state that needs
you, so the newest amber cell in any lane still answers "which of these
needs me". That frees the accent to mean focus alone, the collision ade
solved with violet. The glyph shapes differ enough that the row reads with
colour off, and the header row and footer carry the status word.

The gear and the pause sign, the two pictographic glyphs `style.go` warned
might draw two cells wide, are gone. Every glyph above is in Geometric
Shapes or Mathematical Operators. `○ ◐ ◆ ●` are East Asian Ambiguous like
the lane cells and the rail, so the existing caveat holds unchanged: a
terminal set to double them doubles all of them or none.

The lane's fade keeps its logic (#154); a working cell fades from text
toward muted, which is a short distance, and its height carries the
information. The meter's ramp endpoints become two named constants, amber
to green, instead of lookups into the status map, so changing what a status
means cannot recolour the meter.

A test asserts the rule: amber is bound to waiting and to no other status,
and the accent appears in no status.

## Header row

The header row is one line across the window, segmented by the hairlines
below it.

- **Sidebar segment:** `projects · N`, N the number of registered projects,
  muted.
- **Pane segment:** `<project> ▎ <title> · <branch> · <glyph> <status> <age>`
  on the left, and `<meter> <pct> cached · <in> in / <out> out` on the
  right, the counts as #170 defines them. The rail here marks the session
  the segment is about; it is drawn in accent only while the pane owns the
  keys. When the two sides do not fit with two spaces between, the right
  side collapses in steps: the counts first, then the meter and percentage,
  then the branch, and last the left side is clipped. A pane with no
  session has an empty segment. A modal names itself in the segment
  instead: each modal kind has one name, sentence case, in a table in
  `modalview.go` (new session, new worktree session, rename, confirm,
  switch, register project, adopt session, keys).
- **Review segment:** today's `reviewTitle`, unchanged.

The rule beneath is hairline-coloured, with `┼` under each hairline. A
hairline segment that is accent for focus is accent from the rule to the
footer, not in the rule itself.

## Footer

The keymap keeps everything #28, #30, #43, #44 and #103 won: the exit key
first, the help key second, the working subset after, the review and tree
variants, the modal variant, the notice and the error taking the line in
that order of precedence. All of that is the left side.

The right side is new: facts, right-aligned, muted: `N sessions`, and
`· K waiting` when K is above zero, the waiting token in amber because it is
the waiting state and the rule allows it. The two sides are joined with at
least two spaces; when the window is too narrow for both, the facts are
dropped whole. Truncation therefore cuts facts, never keys, and the exit key
is on screen at every width it is today.

## Branch and diffstat

New data, so a new seam, following the diff's route exactly (#21).

- `vcs.Git` gains `Shortstat(dir, commit string) (Shortstat, error)`,
  running `git diff --shortstat <commit>` and parsing it into
  `Shortstat{Files, Added, Removed int}`. Empty output is a zero value with
  no error. Only `vcs` runs git (invariant 4).
- `review.Source` gains `Stat(sess registry.Session, projectRoot string)
  (Stat, error)` beside `Load`, so the merge-base resolution in `baseCommit`
  lives once. `Stat{Branch string; Added, Removed int}` is the branch from
  `CurrentBranch(sess.Dir)` and the shortstat against the base commit. It
  counts tracked changes only: the review column also renders untracked
  files as additions, so the row can read lower than the column, and the
  column is the truth when opened. This is written into `Stat`'s comment.
- `ui.Deps` gains `Stat RepoStatFunc`, the same shape as `DiffFunc`, wired in
  `cmd/omatty` as `review.NewSource(git).Stat`. A nil `Stat` draws line two
  with the lane alone, which is what every existing test's `Deps` gets.
- The model keeps `repoStat map[string]review.Stat`, display-only like the
  lane: never persisted, `state.json` still suffices alone (invariant 9).
- **Cadence.** A `statEvery` tick of ten seconds polls every session, and a
  session is polled at once when its status turns done or waiting, the two
  moments the number changes. Each poll is a `tea.Cmd` that runs `Stat` off
  the render path and returns `RepoStatMsg{SessionID string; Stat
  review.Stat; Err error}`. A session with a poll in flight is not polled
  again until it returns.
- **Failure.** A session whose directory cannot be read, or whose git call
  fails, keeps its last known stat if it has one and otherwise draws the
  lane alone; the error is logged with `slog.Warn` once per session, and
  the once-flag is cleared by the next success so a repaired checkout logs
  again if it breaks again. Nothing is shown in the footer: a broken
  checkout is loud in the review column, which is where it is actionable.

## Tests

TDD applies; each is written before the code it names.

- `layout_test`: `PaneSize`, `PaneOrigin`, `ReviewWidth` at the new
  constants, including the minimum-size floors.
- `sidebar_test`: `rowHeight`; `Window` in lines, with the selected card
  never split and the header revealed; a table of cursor positions against
  pane heights.
- `sidebarclick_test`: the click inverse over both lines of a card and over
  the hairline, driven by the same placements.
- `render_test`: the header row's collapse steps at descending widths; the
  footer with and without facts; the rail on the selected card; the
  accent hairline following the keyboard owner across terminal, review and
  modal; the frame width of every line at 80x24, 100x30 and 120x32.
- `style_test`: amber bound to waiting alone; accent absent from the status
  map; every glyph one cell wide under `lipgloss.Width`.
- `ramp_test`: the existing quantisation test, with the meter's endpoints
  read from the new constants.
- `vcs`: `Shortstat` parsing over recorded outputs, including empty, files
  only, and binary-only lines.
- `review`: `Stat` against `FakeGit`, including the untracked-excluded
  case, named so the difference from `Load` is on record.
- `ui` model tests: the stat tick, the done and waiting triggers, the
  in-flight guard, and the once-per-session warning.

The coverage gate stays at 90%.

## Smoke

The gate is necessary, not sufficient (AGENTS.md). The milestone ends with
the real-PTY smoke a person reads, in a scratch HOME with `fake-claude`:

```bash
PTY_COLS=120 PTY_ROWS=32 PTY_KEYS=$'\x0fd' go run ./testdata/ptyrun omatty
PTY_COLS=80  PTY_ROWS=24 go run ./testdata/ptyrun omatty
PTY_COLS=60  PTY_ROWS=20 go run ./testdata/ptyrun omatty
```

What is read: every line exactly the window width through
`testdata/screen`; the accent line on the keyboard owner; cards of two lines
with the age and diffstat aligned; the header row collapsing in the stated
order at 80 and 60 columns; the exit key on screen at 60; a modal naming
itself in the header; and, with the real binary once, Claude's own prompt
box sitting in the pane without an omatty frame around it.

## Out of scope

Written down so nothing returns through the side door:

- A second theme, light or otherwise (ROADMAP, "Not on the roadmap").
- Background fills for the sidebar or any column. They break on
  transparent terminals and light themes and degrade worst at 16 colours.
- Nerd Font or provider brand glyphs. claude is the only agent (#46) and a
  font with no icons must still read.
- A context-window gauge like ade's. The transcript gives usage, not the
  window size; it is #170's neighbour and its own issue if wanted.
- Plan, subagent or "changes" panes. omatty is a window, not an orchestrator.
- Blank margins between cards, tabs, a composer.
- Sidebar width other than 28 (#155 decided it).

## Files this touches

For the plan, not a commitment to line counts: `internal/ui/layout.go`,
`panebox.go` (becomes the hairline and rule builders), `render.go`,
`sidebar.go`, `sidebarclick.go`, `wheel.go` (`overSidebar`), `style.go`,
`ramp.go`, `meter.go`, `lane.go` (`rowChrome`), `status.go` (footer,
stat tick), `modalview.go` (names), `reviewview.go`, `deps.go`, `model.go`;
`internal/vcs/git.go`; `internal/review/source.go`;
`cmd/omatty/wiring.go`; and the tests beside each.
