# omatty M5 file tree design

Approved 2026-09-09. Docs issue #193; feature issues #194-#200, one plan and
one PR each under the normal workflow. There is no single implementation
plan: each issue is small enough to carry its own.

## Context

#24 shipped the tree in M3 as the review column's second view: a
`git ls-files --cached --others --exclude-standard` listing folded into a
tree by `review.NewTree`, `*` on a file the session's diff changed, `enter`
folding a directory or previewing a file to 256 KiB as numbered plain text,
`h`/`l` panning. M5 then delivered nothing of its own.

The features below are what the comparable terminal tools all have and the
tree does not: yazi, superfile, nvim-tree, the OpenCode TUI and its file-tree
plugin, and the Bubble Tea explorers built on chroma.

Decisions taken with the user on 2026-09-09:

| Decision | Choice | Rejected |
|---|---|---|
| Milestone | Refill the existing `M5` label and section | A new M9 |
| Scope | All seven features; docs first, then one PR each in build order | The core four only |
| Change state source | `review.File.Status` from the diff omatty already loads | A second `git status --porcelain` call through `vcs` |
| Highlighter | chroma v2 behind `internal/highlight`, an omatty-owned style | A stock chroma theme; tree-sitter; a hand-written lexer |
| Filter key | `/` inside the focused column | nvim-tree's `f`; a leader chord |
| Attach key | `a`, paste brackets without `\r` | `enter` on a file (already the preview); submitting the reference |

## Constraints that shape every issue

- **Invariant 4 in spirit.** chroma is reachable only through
  `internal/highlight`. It is not pre-1.0, but the blast radius rule is the
  same: one package we own.
- **The colour rule** (`internal/ui/style.go`): one hue, one meaning; the
  accent means focus alone; amber, green and red are comment, added and
  removed. #196 reuses those hues for exactly those meanings; #197 avoids
  them.
- **Panning slices by display cell** (`fitContent` in `internal/ui/pan.go`)
  and `previewMaxWidth` memoises the widest plain row (#133). Styled rows
  are cut with `charmbracelet/x/ansi`; plain rows keep doing the measuring.
- **`Visible()` is the only gate** on what the tree draws. Sorting (#194)
  must keep a directory's subtree one contiguous pre-order run; filtering
  (#198) is applied inside it; every change to the visible set calls
  `moveTreeCursor(0)` and `contentChanged()`.
- **Invariant 8.** Anything written to the PTY that spans a newline goes in
  paste brackets. #199 adds the non-submitting form.
- **Invariant 1.** The leader router is not touched. `/`, `a` and `o` are
  plain keys inside a column that already owns the keyboard.

## The seven, in build order

| Issue | Package(s) | Core change | Test that pins it |
|---|---|---|---|
| #194 sort | `review` | component-wise comparator in `NewTree` | `names()` renderer, `internal` beside `internal-old` |
| #195 re-list | `review`, `ui` | `Tree.Relist` keeps collapse; `refreshReview` batches `loadFiles` | new file appears after `StatusDone` with no `r` |
| #196 markers | `review`, `ui` | `TreeNode.Change` enum from `File.Status`; deleted rows; letter + hue | four kinds rendered; `enter` on deleted does not error |
| #197 highlight | `highlight` (new), `review`, `ui` | `Preview.Styled` at read time; ANSI-aware cut; own style | `stripSGR(styled) == plain`; pan never splits an escape |
| #198 filter | `review`, `ui` | `Tree.SetFilter`; `/` editline; title shows query | folded match reachable; `esc` restores collapse |
| #199 attach | `review`, `ui` | `BracketedText`; `a` writes `@path ` and drops focus | fake terminal receives exactly the bracketed text |
| #200 cross-links | `ui` | `o` both ways via `Position` and `Line.NewNo` | round trip returns to the starting row |

## Not in M5

Nerd Font icons and a second theme (refused in M8). Line wrapping in the
preview. A markdown renderer. fsnotify on the worktree. Highlighting the diff
view, which waits on #197's package and is a follow-up issue if wanted.

## Done means

Each PR clears the gate (`gofmt`, `vet`, `golangci-lint`, `go test -race`,
90% coverage), updates the footer, `ctrl+o ?` help and the README key table
where it adds a key, and the milestone ends with rule 2's real-PTY smoke run,
where #199's open question is answered.
