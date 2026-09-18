# `stablyai/orca` — deep dive

> Captured **2026-09-18** against `stablyai/orca` at `main`, MIT, TypeScript,
> 71,802 stars, 4,695 forks, created 2026-03-17, YC-backed, released daily
> (v1.4.205 on 2026-09-17). **Read at code level**, not from the README: every
> claim below about how a feature works names the file it was read in.
> Marketing claims are labelled as such. Analysis only.

The most important project in the field for omatty, for a reason that has
nothing to do with its size: Orca's own GitHub topics include **`ade`**, and
its README calls it "the ADE for working with a fleet of parallel agents."
That is omatty's noun and very nearly omatty's sentence, from a project a
hundred times its size, six months old.

## 1. Purpose and scope

An Electron desktop application for macOS, Windows and Linux that runs Codex,
Claude Code, OpenCode and Pi side by side, each in its own git worktree. The
pitch is orchestration — "the AI Orchestrator for 100x builders", "fan one
prompt across five agents … compare the results and merge the winner" — which
is precisely the variable M9's thesis says omatty does not optimise. On that
axis the two projects disagree by design and neither is confused about it.

Beyond the core: a mobile companion (iOS App Store, TestFlight, Android APK),
**SSH worktrees** ("run agents on a beefy remote box with full file editing,
git, and terminals — auto-reconnect and port forwarding"), and "Ghostty-class
terminals with WebGL rendering, infinite splits, and scrollback that survives
restarts". All four are README claims, not code-read.

The SSH feature deserves a note because it touches omatty's own pitch. omatty
is terminal-native and therefore runs *on* the headless box over SSH; Orca is
a desktop app that *reaches into* one. Both let you use a big remote machine.
Only one of them works when all you have is a terminal.

## 2. Session and process model

One git worktree per agent session, tracked centrally; persistence is a JSON
document (`orca-data.json`) holding worktree metadata. Terminals are rendered
in the app, not delegated to tmux. Sessions belong to repos, and the app is
multi-repository by construction (`worktreesByRepo` in the renderer store).

## 3. Review surface — the part that matters

Orca's README advertises "**Annotate AI Diffs**: drop comments on any diff line
and ship them back to the agent — review, edit, and commit without leaving
Orca." Read against M3, that is the same feature. Read against the code, it is
the same feature built on the opposite anchoring decision, and the difference
is the whole of invariant 7.

`src/shared/diff-comment-types.ts` defines the stored comment:

```ts
export type DiffComment = {
  id: string
  worktreeId: string
  filePath: string
  /** Exact text selected when creating a markdown note, when available. */
  selectedText?: string
  /** Inclusive range start. Must be <= lineNumber when present. */
  startLine?: number
  lineNumber: number
  body: string
  …
  diffIdentity?: string
  side: 'modified'
}
```

**A diff note is anchored by `lineNumber`.** `selectedText` exists, but its own
doc comment scopes it to markdown notes, not diff notes. There is no content
hash and no re-resolution pass.

What happens when the file changes underneath — which, for a comment on a
running agent's work, is the normal case rather than the edge case — is
handled by *detecting* the change rather than surviving it. `diffIdentity` is
built in `mobile/src/session/mobile-diff-review-queue.ts`:

```ts
function statusEntryIdentity(entry, scope): string {
  return buildMobileDiffIdentity([
    scope, entry.area ?? '', entry.status, entry.oldPath ?? '', entry.path,
    String(entry.added ?? ''), String(entry.removed ?? ''),
    entry.conflictStatus ?? ''
  ])
}
```

and compared in `queueNoteCounts`:

```ts
if (comment.diffIdentity !== undefined && comment.diffIdentity !== item.diffIdentity) {
  staleNoteCount += 1
}
```

Two consequences follow, and they are facts about the code rather than
opinions about the product:

1. **A stale note is flagged, not repaired.** The comment keeps pointing at a
   line number that may now be a different line.
2. **The identity includes the file's `added` and `removed` counts**, so any
   edit anywhere in the file changes it. One line added at the top of a file
   marks *every* note on that file stale, including notes on lines that did
   not move.

omatty's `internal/review/anchor.go` makes the other choice — `Anchor{File,
Hunk, Hash, Nth}`, where `Hash` is a content hash of the line's kind and text
and `Nth` disambiguates repeats, with the comment stating the reason plainly:
"Line numbers are deliberately absent - Claude edits files while you read
them." `internal/review/place.go` then resolves in two passes, exact hunk and
occurrence first, then the same content anywhere in the file, because "the
hunk header carries line numbers, so any edit above a hunk rewrites it while
the commented line itself is untouched." A comment that cannot be resolved
degrades to an orphan of its file rather than to a wrong line.

**This is the clearest verified difference between the two products**, and it
is narrow enough to state honestly: both let you comment on a diff line and
send it to the agent; omatty's comment still points at the right line after
the agent edits the file, and Orca's is marked stale. It is one property, not
a verdict, and #300 decides what it is worth.

Orca also tracks `sentAt` per note — "set after the note has been handed to an
agent. Edits clear it" — which is a piece of state omatty's review pane does
not keep and probably should. Filed as a candidate in #299.

## 4. Gate and verification

Nothing found. No feature runs the project's own `fmt`/`vet`/`lint`/`test`
line per session and reports a verdict beside the session. Orca reviews
diffs; it does not judge them.

## 5. Strengths worth borrowing

- **`sentAt` on a comment, cleared by an edit.** omatty composes and sends;
  it does not record that a comment was sent, so a second send repeats it.
- **A per-file "reviewed" mark with a change-detection identity**
  (`isReviewed`, `changedSinceReview`). omatty's review pane has no notion of
  "I have already read this file, and it has not changed since."
- **Generated-file suppression in the review queue** — paths under `/build/`,
  `/coverage/`, `*.generated.ts` are filtered out of review. omatty shows
  every changed file.
- **Scrollback that survives a restart** (README claim). This is exactly what
  #191 left open: `Terminal.Repaint` gets the pane redrawn after a dtach
  reattach, but the scrollback is gone.

## 6. Weaknesses to avoid

- **Line-number anchoring with staleness flagged rather than resolved**, and
  an identity coarse enough that one unrelated edit invalidates a whole file's
  notes.
- **3,036 open issues** against 2,783 closed — the highest open ratio in the
  field. #297 reads that tracker; a number alone is not a verdict.
- Electron, a mobile app, a cloud story and an SSH remote in six months is a
  surface area omatty could not staff, and — more to the point — could not
  hold to invariant 12.

## 7. Bottom line

Orca is the field's centre of gravity and is chasing the orchestration
variable openly. It has independently arrived at omatty's review loop and at
omatty's own word for the category, which is evidence the shape is right and
evidence that the shape is not a moat. What survives contact with it, so far,
is content-anchored comments and the gate — both narrow, both real, both
`#300`'s to judge.

## Sources

- `github.com/stablyai/orca`, `main`, read 2026-09-18: `README.md`,
  `src/shared/diff-comment-types.ts`, `src/shared/diff-comment-schema.ts`,
  `mobile/src/session/mobile-diff-review-queue.ts`.
- GitHub API: stars, forks, creation date, topics, releases, issue counts.
- omatty, for the comparison: `internal/review/anchor.go`,
  `internal/review/place.go`.
