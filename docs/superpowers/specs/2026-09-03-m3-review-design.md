# omatty M3 review design

Approved 2026-09-03. Implementation plan: `docs/superpowers/plans/2026-09-03-m3-review.md`.

## Context

M1 and M2 are merged: sessions run the real `claude` in embedded panes and the
sidebar shows live status. M3 is the reason omatty exists: review a session's
diff without leaving it, comment on lines, and send the whole batch back as one
message. The roadmap adds the file tree (#24) to M3 as of 2026-09-02.

Decisions taken with the user on 2026-09-03:

| Decision | Choice | Rejected |
|---|---|---|
| Base branch for the diff | Persist `Session.Base` at worktree creation; empty means fall back to the project root's current branch; a main-checkout session diffs against HEAD | Derive at review time (wrong after branch switches); fixed `main` |
| Layout | One toggled right column with two views: diff (`ctrl+o d`) and tree (`ctrl+o f`); one column at a time | Tree inside the sidebar; full-screen review mode |
| Comment persistence | In memory per session; footer shows the pending count | `~/.omatty/reviews/<uuid>.json` |
| Scope split | One spec, two plans: A = #21 #22 #23, B = #24 | One plan; defer #24 to M5 |
| Anchor | `(file, hunk header, line hash, nth occurrence)`, with a same-file hash fallback before orphaning | Strict `(file, hunk, hash)` per the issue text |

## Session base branch

`registry.Session` gains `Base string` (`json:"base,omitempty"`).
`Creator.Create` reads the project root's current branch before
`git worktree add`, passes it as the explicit start point (new
`vcs.Git.AddWorktree(repoRoot, dir, branch, base)`), and records it. A detached
root reports the literal `HEAD`; that is stored as empty so review falls back
rather than comparing the worktree against itself. No `Version` bump: a missing
field decodes as empty, which is the fallback signal.

## Git surface

`vcs.Git` gains `MergeBase(dir, ref)`, `Diff(dir, commit)`, `Untracked(dir)`,
`UntrackedDiff(dir, path)`. `Diff` is
`git -c core.quotepath=false diff --no-color --no-ext-diff -M <commit> --`, the
working tree against a commit, so committed and uncommitted changes are one
diff. Untracked files come from `git ls-files --others --exclude-standard`, each
rendered by `git diff --no-index -- /dev/null <path>`, whose exit code 1 means
"differences" and is tolerated by a new untrimmed `capture` helper. Every method
passes the issue #29 directory guard.

## `internal/review`

Pure except for `Source`, which takes a `vcs.Git` interface.

- **Types.** `Diff{Files}`, `File{Path, OldPath, Status, Binary, Hunks}`,
  `Hunk{Header, Lines}`, `Line{Kind, Text, OldNo, NewNo}`. `ParseDiff(io.Reader)`
  converts `go-gitdiff` structs at the edge; `Diff.LineAt(Position)` indexes a
  line.
- **Anchor (invariant 7).** `Anchor{File, Hunk, Hash, Nth}`. `LineHash` is the
  first 6 bytes of SHA-256 over `kind:text`. `AnchorAt(diff, pos)` builds one.
  `Comment{Anchor, Quote, Note}`; `Comments` is the per-session in-memory queue
  with `Add`, `All`, `Len`, `Remove(i)`, `Clear`.
- **Placement.** `Place(diff, comments) Placed` resolves each comment: exact
  `(hunk header, hash, nth)` first, then the first line with the same hash
  anywhere in the file, else an orphan of its file (`Placed.Orphans`), or lost
  when the file is gone. `Placed.Where` maps comment index to `Position`.
  `Flatten(diff, placed) []Entry` yields the rows the cursor walks: file header,
  orphans, hunk header, lines, each line's comments beneath it.
- **Compose (#23).** `Compose(diff, comments)` writes `Review comments (N):` then
  per comment `N. file:line`, `> quoted line`, the note. The line number is the
  new-file number from the current diff (old number for a removed line); an
  orphan says `file (line moved or removed)`. `BracketedPaste(body)` returns
  `ESC[200~ body ESC[201~ \r` (invariant 8).
- **Source.** `NewSource(git).Load(sess, projectRoot)`: HEAD for a main-checkout
  session; else `MergeBase(sess.Dir, base)` with `base = sess.Base` or the
  project root's current branch; then `Diff` plus every `UntrackedDiff`, parsed.

## UI

- **Focus.** `keys.Router` is unchanged. The model computes a focus target:
  terminal, review pane, or note editor. All three are "focused" for the router,
  so `ctrl+o` is the leader everywhere and invariant 1 holds; the `ToTerminal`
  route is dispatched to the target. A prompt or an empty sidebar leaves nothing
  focused, as today.
- **Keys.** `ctrl+o d` opens the diff view and focuses it, or closes the column.
  In the pane: `j`/`k` move, `c` opens the note editor on a diff line, `d`
  deletes the comment under the cursor, `r` reloads, `S` submits, `esc` or
  `ctrl+c` returns focus to the terminal and keeps the column open. Note editor:
  typed text (via `KeyPressMsg.Text`, so capitals and spaces arrive as text),
  `backspace`, `enter` queues, `esc` discards. Leader commands still work with
  the pane focused; `ctrl+o j/k` moves an open column to the new session.
- **Layout.** `PaneSize(width, height, reviewOpen)` and
  `PTYSize(width, height, reviewOpen)`. `ReviewWidth(width, open)` is two fifths
  of the width after the sidebar, floor 24, or 0 when closed. Opening or closing
  resizes the focused terminal.
- **Loading.** `Deps.Diff DiffFunc`
  (`func(sess registry.Session, projectRoot string) (review.Diff, error)`).
  `loadDiff` runs it as a `tea.Cmd` returning `DiffLoadedMsg`; a stale message
  (pane closed or moved to another session) is dropped. Reload on `r`, on session
  switch, and when the pane's session transitions to `done` or `waiting`.
- **Render.** Title `diff · N files · M comments`. File header `path +a -b`
  (renames `old → new`, binaries `(binary)`), muted hunk headers, green `+`, red
  `-`, plain context, comments as `  >> note` in amber beneath their line,
  orphans `  >> (moved) note` at the top of the file, cursor row reversed,
  viewport scrolled to keep the cursor visible. Note editor is the pane's last
  row: `note: buffer_`. Footer swaps to the review keymap while the pane is
  focused; the main footer gains `ctrl+o d diff` and `ctrl+o f files`.
- **Submit.** `S` with no comments sets the footer error. Otherwise `Compose`,
  `Clear`, unfocus the pane, and `term.SendInput(BracketedPaste(body))`. The
  `Fake` terminal records it in `Sent`.

## File tree (Plan B)

`vcs.ListFiles(dir)` = `git ls-files --cached --others --exclude-standard`.
`review.NewTree(paths, touched)` builds a pre-order listing with collapsible
directories; `Visible()` skips children of collapsed directories; a directory is
touched when any touched file is under it. `review.ReadPreview(dir, rel)` reads
up to 256 KiB, flags binaries (NUL byte) and truncation, and refuses absolute or
parent paths. `ctrl+o f` opens the column in tree view (or closes it from tree
view; switches from diff view). In the tree: `j`/`k`, `enter` toggles a directory
or previews a file, `esc` to the terminal. In preview: `j`/`k` scroll, `esc` back
to the tree. Touched marker `*` comes from the loaded diff's file paths.

## Testing

`review` to 100% with inline fixture diffs and a named `FakeGit`. `vcs` with real
temp repos. `ui` drives keys through `termwrap.Fake` and asserts `Sent[0]`
carries the envelope; a `deliver` helper runs returned commands and feeds their
messages back. Each plan ends with the gate and a `ptyrun` smoke test of the real
binary at 100x30 and 160x45.
