# M15 - The Polish — design (#420)

Approved 2026-09-26 in a brainstorming session. Nineteen implementation issues,
#421-#439, one PR each; this document and the ROADMAP section are #420.

## Context

The review column has four faces - the diff (M3), the file tree and preview
(M5), the gate (M9) and the tracker (M14) - and the `ctrl+o ?` help modal (M4).
Each was built in its own milestone to its own taste, and it shows:

- four cursor styles: reverse video (diff, tree), `▸ ` (gate), accent
  foreground (tracker), `»` (pick list);
- gate verdicts drawn without colour, and three separate glyph sets for state
  (gate marks, CI marks, the sidebar gate strip);
- a column rule that reads `review` on every face;
- no scroll position anywhere, no page or jump keys, clicks on two faces of four;
- a help list that is unstyled and stale since M9.

Mapping the column for this milestone also found three bugs (#421-#423).

Decisions taken with the user:

| Decision | Choice |
|---|---|
| Goal | **Polish and consistency** - the faces read as one product. Speed and discoverability improve as a side effect, not as the aim. |
| Packaging | **A milestone**, M15, with its own ROADMAP section; every issue in Sprint Backlog. |
| Glyphs | **Nerd Font icons, opt-in** (`[ui] icons = "nerd"`), default plain Unicode. Reverses the M5 and M8 cuts; a second colour theme stays cut. |
| Width | **Polish in place, plus zoom.** The column keeps `(width-28)*2/5`; `ctrl+o z` maximises it, and two-pane layouts exist only while zoomed and wide enough. |

## Where the ideas come from

Researched 2026-09-26; each issue names its source.

| Face | Borrowed from | What |
|---|---|---|
| Gate | GitHub Actions, `gh pr checks` | summary counts in the title, passing steps collapsed, first failure opened, durations |
| Tree | VS Code, neo-tree, broot, yazi | compact single-child folders, status rolled up to directories, changed-only |
| Tracker | gh-dash | glyph columns for state/CI/review, preview beside the list |
| Diff | delta, diffnav, diffview.nvim, lazygit | syntax + word-level highlight, `]`/`[` files, `n`/`N` hunks, file list |
| Help | which-key, zellij, lazygit, k9s | context-first help, next keys after the leader, footers that shorten |

## Shared rules every slice follows

1. **One window.** Every face scrolls through the type #424 extracts: cursor,
   offset, height, keep-the-cursor-visible. Reverse video is the cursor. `N/M`
   goes in the title and is the first thing dropped when the title is short.
   `g`/`G`/`ctrl+d`/`ctrl+u` and click-to-select work on every face.
2. **One vocabulary.** Every state drawn anywhere comes from #425's table: a
   glyph and one of the eight palette colours. Missing is amber, never red
   (invariant 12's corollary). Unknown is never drawn as a verdict (M14).
3. **Chrome names the face.** The rule says `diff`, `files`, `preview`,
   `gate`, `tracker` (#426). Titles and footers shorten by priority, whole
   entries only, never cut mid-word.
4. **Keys exist only while the column has focus.** Nothing here takes a key
   from the terminal pane or changes `internal/keys`' routing (invariant 1).
   #439 changes what the footer *shows* while the leader is armed, not what
   is routed.
5. **Display only.** Nothing in M15 decides anything from text: gate search
   and wrapping (#429) are rendering, verdicts stay exit codes (invariant 12);
   item markdown styling (#433) applies to parsed lines of untrusted text and
   emits no control sequence from it.
6. **Nothing persisted.** Zoom, folds and the changed-only toggle are viewing
   state. `state.json` does not change (invariant 9).
7. **Every new key has a help row**, enforced by #422's test.

## The slices

In landing order. A slice's dependencies land before it.

| # | Slice | Depends on |
|---|---|---|
| **Bugs** | | |
| #421 | the gate view cannot scroll | - |
| #422 | help omits the gate's and tracker's keys; help rows derived from key tables | - |
| #423 | the tracker's age is not at the right edge | - |
| **Foundation** | | |
| #424 | one list window for every face | (absorbs #421) |
| #425 | one state vocabulary; `[ui] icons = "nerd"` | - |
| #426 | chrome names its face; titles and footers shorten by priority | - |
| #427 | `ctrl+o z` zooms the review column | - |
| **Gate** | | |
| #428 | reads like a CI check page | #421, #425 |
| #429 | `r` re-runs; output wraps; `/` searches opened output | #421, #424 |
| **Tree** | | |
| #430 | compact folders, rolled-up status, `c` changed-only | #424 |
| #431 | file-type icons, opt-in | #425 |
| **Tracker** | | |
| #432 | state, CI and review glyph columns; `reviewDecision` in `openFields` | #423, #425 |
| #433 | an item reads like a page | #425 |
| #434 | preview beside the list when zoomed | #427, #433 |
| **Diff** | | |
| #435 | syntax and word-level highlighting | - |
| #436 | `]`/`[` files, `n`/`N` hunks, header counts and position, fold a file | #424 |
| #437 | file list beside the diff when zoomed | #427, #436 |
| **Help** | | |
| #438 | opens on the active face, styled, filterable | #422 |
| #439 | footer lists the next keys while the leader is armed | #426 |

Each issue carries its own "Done when"; this table is the order, not the spec
of each slice.

### Four decisions worth the space

- **Why zoom and not a resizable split.** A split needs a persisted width, a
  drag or key to change it, and a layout that is right at every width. Zoom is
  one flag and two layouts, both of which already exist. The two-pane views
  (#434, #437) need room only sometimes, and zoom is how you ask for it.
- **Why the leader hints go in the footer, not a popup.** A which-key popup
  would cover claude's pane during the keystroke the operator is choosing. The
  footer is omatty's own row, and replacing it costs nothing (#439).
- **Why the gate's running state spins the run, not the step.** `gate.Runner`
  reports a finished `Report`, not per-step progress. Streaming step state is a
  runner change with its own supersede and bound questions (#229), and it is not
  polish. #428 spins the title and shows elapsed time.
- **Why the tracker's markdown is line rules, not a renderer.** M5 cut a
  markdown renderer for the preview, and the argument holds. #433 bolds
  headings, turns bullets into `•`, and mutes fences on text that is already
  wrapped. That is enough to read an issue, and it is not a dependency.

## Amended cuts

- **M5 and M8 cut Nerd Font glyphs.** M15 takes them back, opt-in only (#425,
  #431), because a tofu box is worse than no icon and that argument still holds
  for the default. **A second colour theme stays cut.**
- **M5 deferred diff highlighting** as "one more caller of #197's package once
  that exists". #435 is that caller.

## Deliberately out

- **A side-by-side diff.** It needs 160+ columns to be worth it, zoom gets most
  of the way there, and it doubles every diff-rendering path.
- **A resizable column**, argued above.
- **A second colour theme** (above).
- **A command palette.** fleet's `ctrl+k` shows an empty screen under some tmux
  key setups (fleet #139). #438's filterable help answers "what was that key".
- **gh-dash's saved-search sections.** Those would be configuration of a view M14
  kept zero-config on purpose.
- **Moved-code colouring** (delta). It is worth having, but it is not polish.
- #337 (reviewed marks), #338 (generated files collapsed) and #339 (several
  comments per line) stay M12. #436 and #437 must not preclude #337, and #437
  draws its marks once it lands.

## Verification

Each slice's PR clears the full local gate (AGENTS.md) and CI on both runners.
Every bug fix follows the regression-test procedure.

The milestone ends with the real-binary smoke test (ROADMAP "Rules", rule 2),
read by a person, at **80x24** and **200x50**, zoomed and unzoomed, on every
face. It also runs the M4 trap from #438: `ctrl+o q` from inside a filtered help
modal quits.
