# Review scoped to the last turn — design (#311)

Approved section by section on 2026-09-23. One issue, #311; one implementation
plan, built test-first, one PR per task where the plan says so.

## Context

`internal/review` shows everything a session changed against its base. On the
third turn you reread three turns of diff to find the one thing that changed.
omatty's claim is time-to-judgement (M9), so this is a gap in the thesis, not
a convenience: M12 found it as the only P1 inside the thesis itself
(`docs/research/prior-art-findings.md` §P1-2). The field asks for it by name —
Orca #11840, "turn-scoped diff review … backed by a refs/orca baseline snapped
at agent turn boundaries".

Decisions taken with the maintainer on 2026-09-23:

| Decision | Choice | Rejected |
|---|---|---|
| Baseline moment | **Turn start** — the `UserPromptSubmit` hook | Turn end (`Stop`), which counts your own edits between turns as the agent's; last review sent, which spans several turns |
| Mechanism | **A tree ref built through a temporary index** | `git stash create`, which omits untracked files — the files an agent creates; copying files under `~/.omatty` and `git diff --no-index` |

## Constraints that shape the design

- **Invariant 4.** Every git call is in `internal/vcs`. The UI reaches it
  through injected functions, as it already does for `RemoveWorktree`.
- **Invariant 9.** `state.json` must suffice to relaunch. The ref's name is
  derived from the session's row id, so no field is added.
- **Invariant 11.** A hook never blocks claude. The snapshot therefore races
  claude's first edit, by milliseconds against model latency measured in
  seconds. Stated, not hidden.
- **Invariant 2 in spirit.** Status and scope come from hooks and git, never
  from the screen.
- **The tailer reports `PromptSubmitted` for tool results** (`DeriveKind`,
  `internal/watcher/transcript.go`), i.e. in the middle of a turn. A snapshot
  triggered by it would advance the baseline mid-turn and drop the turn's
  earlier edits. Only the hook may trigger one.
- **Invariant 7.** Anchors are `(file, hunk header, content hash, nth)`. A
  turn diff's hunk headers count from its baseline and a line the session
  added can be context in a later turn, so neither the header nor the hash
  carries across the two diffs, and `resolve`'s fallback to the first line
  that reads the same would put a comment on another `}`. The two diffs are
  mapped on what does carry across - path, new-side line number and text -
  by `review.AnchorFor` and `review.PlaceIn` (corrected by the final review).

## 1. Capturing the baseline

**Trigger.** `watcher.Event` gains `Hook bool`, set by the socket listener and
never by the tailer. On an event with `Kind == PromptSubmitted && Hook` for a
session it holds, the UI starts one background command that snapshots the
session's directory and points its turn ref at the result. Every way a turn
starts goes through that hook: a typed prompt, the review's `S`, the gate's
`S`.

A session whose hooks are not arriving — the listener failed to bind and the
UI degraded to the tailer (#49) — never gets a baseline. Its turn scope says
so (§2); it never falls back to a wrong one.

**`vcs`**, five new methods on `Git` and `CLI`:

- `SnapshotTree(dir string) (string, error)` — resolve
  `git rev-parse --git-path index` (a linked worktree has its own), copy that
  file to an `os.CreateTemp` path so git reuses its stat cache, then with
  `GIT_INDEX_FILE` set to the copy run `git add -A` (honours `.gitignore`,
  picks up untracked files) and `git write-tree`. The copy is removed on
  every path out. HEAD, the real index and the stash are never touched, and
  no git identity is needed because no commit is made. A missing index (a
  repository with no commits yet) starts from an empty temporary index.
- `SetTurnRef(dir, id, tree string) error` —
  `git update-ref refs/omatty/turn/<id> <tree>`. Refs live in the common
  repository: a worktree's ref is visible from the main checkout, and it
  survives omatty restarting.
- `TurnRef(dir, id string) (string, bool, error)` — the tree the ref names,
  false when there is none (`git rev-parse --verify --quiet`).
- `DeleteTurnRef(dir, id string) error` — `git update-ref -d`; deleting a ref
  that does not exist is not an error.
- `DiffTrees(dir, from, to string) (string, error)` — `git diff` between two
  trees, with the same `diffArgs` the working-tree diff uses. Because both
  sides are trees built with `add -A`, new files arrive as additions and no
  separate untracked pass is needed.

**Known limits.** The race above. A repository with a large untracked but
unignored tree pays for `git add -A` on every prompt, as `git status` already
does. `git push --mirror` would publish `refs/omatty/*`; an ordinary push does
not.

## 2. The review side

**Toggle.** `t` in the diff view switches between the whole session (today's
diff, the default) and this turn. `ReviewPane` gains `Scope`; a fresh pane —
the column moving to another session — starts at the whole session, as the
rest of the pane already does. The help modal and README list the key.

**Title.** In turn scope the diff title gains `this turn` at `keepFlag`
priority, the same as `⚠ no tests`, so a narrow column never gives it up. A
turn view that looks like the whole diff is the worst failure this feature
could have.

**Loading.** `review.Source.LoadTurn(sess registry.Session) (Diff, error)`:
read the turn ref; with none, return `ErrNoTurn`; otherwise snapshot the
current tree (no ref written) and parse `DiffTrees(dir, base, current)`.
`loadDiff` loads the turn diff alongside the full one whenever the scope is
this turn, so the turn view refreshes on exactly the triggers the full diff
already has: opening the column on a session, `r`, and the session coming to
rest in `done` or `waiting` (a closed column is marked stale and pays on
reopen, #124). Switching to turn scope loads it at once. The load result carries its scope, so a
turn diff that arrives after the operator switched back is dropped, as a diff
for another session already is.

**Two diffs, one shown.** `m.review.Diff` stays the full session diff and is
always loaded; the turn diff is a second field. A `shownDiff()` accessor feeds
everything that draws or indexes rows — entries, file headers, the coverage
markers, `⚠ no tests`, panning. Three things deliberately keep the full diff:

- **The file tree's change markers**, which M5 defines as what the session
  changed.
- **`PruneSent`** (#335): a sent comment is not dropped because it lies outside
  this turn. It runs only when a *full* diff loads.
- **`Compose`**: `S` sends every pending comment, whichever scope it was
  written in, located by its line in the full session diff.

**Comments in turn scope.** A comment that places on a line of the turn diff
draws there. One that does not is not `(moved)` — it is elsewhere in the
session — so turn scope hides it instead of floating it up. It is still
counted in the title and still sent by `S`.

**Coverage (M10).** The overlay is keyed by path and new-side line number, so
its markers work unchanged on a turn diff: what this turn added that no test
covers.

**Notices.** The turn scope says why it has no rows instead of showing none,
and a failure looks like one (#353). The two neutral notices are muted:

- No ref yet: `no turn recorded yet: a baseline is taken when you send a
  prompt`.
- A load in flight: `reading this turn...`.

The three failures use the error style:

- The hook socket did not bind (#49): `hooks are not arriving, so no turn
  baseline can be taken: see the log`. Any ref standing then was left by
  another run, and diffing against it would call someone else's turn this one.
- A failed snapshot (§3): `this turn's baseline could not be taken:`.
- A failed load: `reading this turn failed:`.

Each failure is followed by git's own words and `full error in the log`,
wrapped to the column rather than cut at its edge (#351).

## 3. Lifecycle, failure, tests

**Cleanup.** Archive (`ctrl+o x`) and `omatty rm` delete each affected
session's ref, from the project root. A failed delete is logged and never
blocks the archive. `ctrl+o s` keeps it: the session still exists, and
resuming keeps its last turn. There is no boot-time sweep of stray refs, since
two omatty `HOME`s can share one repository (the smoke tests do) and a sweep
in one would delete the other's baselines; a crash can therefore leave a ref
behind, which is accepted.

**A failed snapshot must not leave the previous turn's ref standing as this
turn's baseline** — diffing against it would present two turns as one. A
per-session `turnErr` records the failure, the next success clears it, and
while it is set the turn scope shows the notice instead of a diff. A
per-session in-flight flag drops a snapshot requested while one is running;
the resulting diff shows more than one turn, never less, which is the safe
side. Both maps join `forgetSessionMaps`, and the reflection tests
(`TestForgetSession_ClearsEveryPerSessionMap`,
`TestStopSession_KeepsEveryPerSessionMapButTerms_issue318`) cover them.

**Tests first, named `_issue311`.**

- `internal/vcs`, against real git in `t.TempDir()` as `git_test.go` already
  does: `SnapshotTree` includes a new untracked file and excludes an ignored
  one; leaves the real index file byte-identical, HEAD unmoved and the stash
  empty; works in a linked worktree, whose ref reads back from the main
  checkout; works in a repository with no commits. `DiffTrees` reports an
  added, a modified and a deleted file. `SetTurnRef` / `TurnRef` /
  `DeleteTurnRef` round-trip, and deleting a missing ref succeeds.
- `internal/watcher`: the listener sets `Hook`; the tailer never does.
- `internal/review`: `LoadTurn` returns `ErrNoTurn` without a ref and the tree
  diff with one (fake `vcs.Git`).
- `internal/ui`, fakes for the injected functions: a hook `PromptSubmitted`
  starts a snapshot and a tailer one does not; a second prompt while one is in
  flight starts nothing; `t` switches scope and the title keeps `this turn` at
  a narrow width; the no-ref and failed-snapshot notices; a comment outside
  the turn is hidden, not `(moved)`, and is still sent; a turn load never
  prunes a sent comment; archive deletes the ref.
- **Real-PTY smoke test.** `fake-claude` fires no hooks, so the run pipes a
  real `{"hook_event_name":"UserPromptSubmit"}` payload into `omatty hook`
  with `OMATTY_SESSION` set, the method #316's smoke test used. Edit a file
  before the pipe and another after it; the turn scope must show only the
  second, the whole-session scope both. A run with real claude would confirm
  the hook's timing and is recorded as owed, not claimed.

## Not in #311

- **Per-file "reviewed, and unchanged since"** — #337, a different axis
  (what the person has read, not when it changed).
- **Reverting a turn** — #334, which reuses this ref and adds only the
  restore.
- **A turn history.** One ref per session, the latest turn. Stepping back
  through turns would need one ref per turn and a way to pick among them;
  nothing has asked for it.

## Done means

The gate green on both runners; the smoke test above run and read; README's
Review section and the help modal list `t`; `CHANGELOG.md` gains the entry
under Unreleased; #311 closed by the PR that ships the toggle.
