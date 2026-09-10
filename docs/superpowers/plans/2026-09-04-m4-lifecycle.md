# M4 Lifecycle Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task (the user wants inline execution, not subagents). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Manage sessions and projects from inside omatty — rename one in place, archive one and optionally remove its worktree, jump to one by typing a few letters, and register repositories by choosing from the ones Claude Code already knows you use (issues #95, #41, #40, #42, #91).

**Architecture:** `internal/ui` grows a single **modal layer**: one `modal` field with a kind enum, replacing the ad-hoc `Prompt` bool, through which four surfaces (prompt, rename, confirm, list) take the keyboard without the `keys.Router` learning anything. Resize stops asking about keyboard ownership, which is #95 and the class of bug every new surface would otherwise re-create. Two new pure packages: `internal/fuzzy` (subsequence ranking) and `internal/discover` (the Claude transcript store → proposed repositories). `internal/registry` gains rename and remove commands; `internal/vcs` gains `MainCheckout`; `internal/watcher` learns to stop one tailer.

**Tech Stack:** Go 1.26, `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, stdlib `testing`, real git in temp repos for `vcs`. No new dependencies.

**Execution shape:** Six branches, sequential, each its own PR to `develop`, each closing exactly one issue:

| PR | Branch | Closes | Introduces (later PRs consume) |
|---|---|---|---|
| 0 | `docs/m4-lifecycle-plan` | — | this document |
| 1 | `fix/95-resize-behind-an-open-prompt` | #95 | the resize helper |
| 2 | `feat/41-rename-a-session` | #41 | the modal abstraction, `lineEditor`, modal rendering, `Run(Deps)` |
| 3 | `feat/40-kill-or-archive-a-session` | #40 | — |
| 4 | `feat/42-fuzzy-switcher` | #42 | `pickList`, `internal/fuzzy`, `Sidebar.SelectByID` |
| 5 | `feat/91-discovery-picker` | #91 | — |

One issue per PR: `Closes #A, #B, #C` auto-closes only `#A` (the trap hit in the M1/M2 close-out).

## Global Constraints

Copied from `AGENTS.md`; every task's requirements include these.

- Functions 4-20 lines (`funlen`: 20 lines, 15 statements). `gocyclo` 10. `gocognit` 15. `dupl` threshold 100. Files under 500 lines. Max 2 levels of indentation.
- Names specific and unique. Banned: `data`, `handler`, `manager`, `util`, `helper`, `process`, `info`, `obj`.
- No `any` / `map[string]any` across a package boundary.
- Error messages carry the offending value and the expected shape.
- Keep existing comments. Write WHY, not WHAT. Doc comment on every exported identifier with one usage example. Reference the issue number where a line exists because of a bug.
- No package-level mutable state, no `init()` side effects. Inject through constructor or parameter.
- `stdout` belongs to the TUI (invariant 5). `fmt.Print*` is banned by `forbidigo`.
- Invariant 1: key routing is modal, never heuristic. Invariant 4: only `internal/vcs` shells out to git; `ui` never imports it. Invariant 9: `state.json` must always suffice to relaunch every session. Invariant 10: `cmd/` stays thin.
- Tests: failing test first, run it and read the failure, then implement, run again. Name regression tests `TestX_describesTheBehaviour_issueNN`. No `time.Sleep`. Filesystem tests use `t.TempDir()`. Named fake types in `fakes_test.go`.
- Commit messages `type(#issue): message`, ending with:
  ```
  Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5
  ```
- The gate before claiming a branch ready (golangci-lint v2.13.2 is installed at `$(go env GOPATH)/bin`):
  ```bash
  gofmt -l .
  go vet ./...
  $(go env GOPATH)/bin/golangci-lint run
  go test ./... -race
  ./scripts/check-coverage.sh 90
  ```
- Never merge. Open the PR to `develop`, move the issue and PR to the Review column, and stop.
- Board: project 13, status field `PVTSSF_lAHOBTZlyM4BiIw3zhhB38g`; option ids Review `585b7724`, Sprint Backlog `6c6ffcfe`, Done `fca54811`. Fetch item ids with gh's own `--jq`, never through a shell variable (issue bodies carry control characters).

---

## Design spec

### Context

M1-M3 are merged: sessions run the real `claude` in embedded panes, the sidebar shows live status, and the review column diffs and comments. What omatty cannot do is *manage* any of it. With ten sessions there is no way to remove one except editing `state.json`, no way to fix a title typed wrong, and `j`/`k` is the only way to reach a session. Projects are registered one directory at a time, typed from memory.

Decisions taken with the user on 2026-09-04:

| Decision | Choice | Rejected |
|---|---|---|
| Archive semantics | Hard-delete the row from `state.json`; the transcript survives on disk | An `Archived bool` every future feature must reason about; refusing to archive while a worktree exists |
| Confirm shape | Three keys, keep-the-worktree plain: `[y] keep  [w] remove  [esc] cancel`; `y`/`esc` only for a main checkout | Two sequential prompts; a dirty-state check before the dialog |
| Keymap overflow | Freeze the footer's head, announce keys per-modal, add a `ctrl+o ?` help modal | Appending four keys (183 columns); letting it truncate |
| Discovery surface | `omatty discover` CLI **and** a `ctrl+o a` TUI picker | CLI only; a non-interactive list plus manual `omatty add` |
| Modal state | One `modal` field with a kind enum | Four independent `Active` bools; a `modalSurface` interface |

### Shared piece 1 — split keyboard ownership from pane layout (PR1)

`focusedTerminal()` (`internal/ui/model.go:402`) answers two questions with one `nil`: "does the terminal own the keyboard" and "which PTY fills the pane". `resizeFocused()` (`model.go:391`) asks the layout question and gets the keyboard answer, so a `WindowSizeMsg` arriving while the prompt is open resizes nothing and no path re-resizes on `esc`. That is #95 — the #51/#73/#75 symptom reached by a fourth path.

```go
// selectedTerminal is the terminal the sidebar cursor is on, whether or not it
// currently owns the keyboard. Layout asks this one; key routing asks
// focusedTerminal. Conflating them is issue #95.
func (m *Model) selectedTerminal() termwrap.Terminal
```

`focusedTerminal()` becomes `if m.modalOpen() { return nil }; return m.selectedTerminal()`. `resizeFocused` is **renamed** to `resizeSelected` — renamed, not aliased, so every call site surfaces in review. Four call sites change: `onResize` (`model.go:378`), `moveCursor` (`:385`), `toggleView` (`review.go:144`), `refocusOrClose` (`review.go:157`). `renderTerminal` (`render.go:79`) keeps `focusedTerminal()`; there the nil legitimately means "draw the modal instead".

This immunises the three surfaces added later. Under the old code each would have re-created #95 independently, and the natural per-surface patch — "resize on close" — must be repeated on every exit path of every surface. The split removes the class instead.

**Rule this establishes:** a modal owns the terminal pane's *content*, never its *geometry*. `PaneSize`/`PTYSize` (`layout.go:53,71`) never learn about modals.

### Shared piece 2 — the modal layer (PR2)

Four surfaces compete for the keyboard. Four independent bools make illegal states representable (rename open *and* confirm open) and multiply branches across the six sites that ask "is anything open" — `focus()`, `command()`, `focusedTerminal()`, `renderTerminal()` twice, `footerKeys()`. `focus()` and `command()` are 12 and 13 lines against a 20-line/15-statement cap and would not survive four `if`s each. An interface was rejected: every method needs `*Model` back, so it is a callback bundle with no polymorphism it exploits, and it breaks the `m.Prompt()` test idiom.

New `internal/ui/modal.go`:

```go
type modalKind int

const (
	modalNone modalKind = iota
	modalPrompt  // new session (n / N)
	modalRename  // retitle the selected session (R, #41)
	modalConfirm // kill or archive it (x, #40)
	modalList    // fuzzy switcher (/) and discovery picker (a), #42/#91
	modalHelp    // the full leader keymap (?)
)

type modal struct {
	Kind    modalKind
	Editor  lineEditor // modalPrompt, modalRename
	Confirm confirmBox // modalConfirm
	List    pickList   // modalList
}

func (m *Model) modalOpen() bool { return m.modal.Kind != modalNone }
```

`focusTarget` (`review.go:52`) does **not** change. A modal never reaches `dispatch`, because `focus()` reports "nothing focused" and `keys.Router.Next(key, false)` (`keys/router.go:53`) routes every key down the omatty path. That is the existing prompt trick, generalised: invariant 1's structure survives four new surfaces without the router learning anything.

`onKey` passes the message rather than the keystroke (`routing.go:27` becomes `m.command(msg)`), because text editors need `msg.Text`. `command` keeps its `ctrl+c`-first check (issue #28) and then delegates to `onModalKey`, a switch on `Kind`.

`Prompt` and `m.Prompt()` (`model.go:160`) survive as a **projection** over the modal, so every test in `prompt_test.go` and `quit_test.go:36` passes untouched. `m.prompt`, `onPromptKey` and `promptLine` are deleted — net −35 lines from `model.go`, which matters at 414 of the 500-line cap.

`lineEditor` fixes a latent bug while generalising: `onPromptKey` (`model.go:289`) guards on `len([]rune(key)) == 1` over the *keystroke*, so `shift+r` appends nothing — you cannot type a capital into a session title today. The editor takes `tea.KeyPressMsg` and appends `msg.Text`, as `onNoteKey` already does correctly (`reviewkeys.go:114`).

Opening keys go in a third table, `modalCommand`, chained from `paneCommand`'s default: `navigate` is at gocyclo ≈7 and four more cases break the limit of 10. `R` matches both `"shift+r"` and `"R"`; `"shift+R"` never occurs (issue #87), and lowercase `r` is already restart.

### Shared piece 3 — the pick list and `internal/fuzzy` (PR4)

`#42` and `#91` need the same filtered, scrollable, cursor-driven list. One widget in `internal/ui/picklist.go` with a concrete element type — no generics, nothing untyped crossing the boundary:

```go
// pickItem is one row of a pick list. ID is what the caller resolves back to a
// domain value - a session id for the switcher (#42), a repository root for the
// discovery picker (#91) - so the widget holds no domain type.
type pickItem struct {
	ID, Label, Detail string
	Marked            bool // multi-select only (#91)
}
```

`pickList{Items, Query, Cursor, Offset, Multi, matches, hay}` with `SetQuery`/`Move`/`ToggleMark`/`Current`/`Chosen`/`Window`. `Move` and `Window` call the **existing** `ScrollOffset(cursor, offset, rows)` (`reviewkeys.go:77`) verbatim — the widget grows no scroll maths of its own, both because `dupl` would flag it and because that helper is already covered by the #21/#24 tests. Visible rows mirror `reviewRows()` (`reviewkeys.go:65`).

`j`/`k` cannot move here — they are filter text. Movement is arrows plus `ctrl+j`/`ctrl+k`, and the modal footer says so. That is the one place M4 departs from the sidebar keymap, and the comment must say why.

`internal/fuzzy` is a new pure package rather than a file in `ui`, which is already the largest package with files near the 500-line cap:

```go
// Match reports whether every rune of query appears in s in order, ignoring
// case, and scores the match - lower is better. Consecutive runs and
// word-boundary hits score better, so "psf" ranks "parser-fix" above
// "prompts-final". An empty query matches everything at score 0.
func Match(query, s string) (score int, ok bool)

// Rank returns the indices of items matching query, best first, stable for
// equal scores so an empty query preserves the given order.
func Rank(query string, items []string) []int
```

`hay` is built once when the list opens, not per keystroke, so typing allocates nothing.

### Rendering

There is no compositing layer and M4 does not add one — building one means cell-accurate splicing of ANSI-styled lines, and `panLine`/`fitLine` (`render.go:173-202`) show how much care one horizontal cut already takes. **Every modal replaces the terminal pane's content**, occupying exactly that column at exactly its size, which keeps `TestModel_EveryColumnIsSpentOnARenderedPane_issue34` (`quit_test.go:138`) and its review-open sibling (`:168`) green and unchanged.

Drawing a surface as an extra line *inside* the pane (the `noteEditor` shape) was rejected for the editors: it shrinks the drawn body to `h-2` while the PTY is still `h-1`, clipping the bottom row — the #75 bug, transiently.

`View()` (`render.go:52`) does not change. `renderTerminal` grows to three branches — modal, terminal, empty state — staying at 10 lines by extracting `emptyLines()`. New files `internal/ui/modalview.go` (`modalLines`, `editorLines`, `confirmLines`, `helpLines`, `modalFooter`) and `internal/ui/picklist.go`.

**The footer constant (`render.go:17`) is frozen at its head.** It is already 114 columns, truncated at 100 — `ctrl+o f files` is invisible today — and M4's four keys would take it to 183. `TestModel_footerNamesTheNavigationKeys_issue30` (`quit_test.go:121`) and `_keyHintsStayVisibleWithASessionFocused_issue30` (`:104`) assert `ctrl+o q`/`j`/`n` survive truncation at width 80, so anything inserted before them breaks the tests and anything appended pushes another hint off the end. Discoverability comes from `footerKeys()` gaining a `modalFooter(m.modal.Kind)` branch first, plus the `ctrl+o ?` help modal.

### Kill/archive teardown (#40)

Five things must unwind; missing any one leaks:

1. `term.Close()` then `delete(m.terms, id)`. `m.terms` is the same map object `run.go:52`'s `defer closeTerminals` holds, so deleting prevents a double close. Follow `restartFocused`'s ordering discipline (`model.go:270`).
2. **Stop the tailer.** `watcher.Watch` holds `tailers []*Tailer` (`watch.go:33`) with close-all only; a removed session's goroutine otherwise stats a dead path at 1 Hz forever, holding its 32-entry ring. Change the slice to `map[string]*Tailer`, add `Watch.Remove(id)` (closing a displaced tailer on a duplicate `Add`), and add `Deps.TailStop func(sessionID string)` mirroring the existing `TailStart` (`model.go:79`).
3. `registry.RemoveSession(s *Store, id string) (Session, error)` — splice the row out and save.
4. `git worktree remove` **only** on the `w` answer and **only** when `sess.Worktree` is true (`registry/state.go:31`). `vcs.RemoveWorktree` already exists (`vcs/git.go:117`), unused by production code, and passes `--force`, which is precisely why the confirmation exists. `ui` may not import `vcs`, so it arrives as an injected func closing over `vcs.NewCLI()` and `m.projectRoot(sess.Project)` (`review.go:242`).
5. Clear `status`, `notified`, `comments` (`model.go:76,82,91`) and reset `m.review` if `review.SessionID` pointed at the victim, or the next refresh reads a directory that no longer exists.

Then rebuild with `SetRows` and end with `tea.Batch(m.resizeSelected(), m.followSession())` — the pair `moveCursor` uses (`model.go:385`). This matters: `SetRows` falls back to the *first* session row when the selected id is gone, not the neighbour, so the cursor can jump across projects and nothing else would have resized the row it landed on.

`onStatus` already filters by `m.knownSession` (`status.go:40`), so late hook events for an archived session are dropped and cannot resurrect its row — built for #69, correct here for free.

**Ordering hazard:** closing the PTY kills claude only indirectly (SIGHUP when the master closes); bubbleterm's `Close` neither signals nor waits for the child. The sequence is close → stop tailer → remove worktree, and this plan accepts the residual race rather than adding a `Wait` method to the `Terminal` interface, which would have to be mirrored on `Fake` and `Guard` under invariant 4. If the smoke test shows flakiness, add the bounded exit wait then, under its own issue.

### Discovery (#91)

The slug cannot be reversed — `paths.TranscriptSlug` maps every non-alphanumeric to `-` (`paths.go:40`), so `/`, `.`, space and `_` all collapse. Each transcript records its own `cwd`, which sidesteps the lossy mapping.

Three filters turn a store into a list worth reading, **measured on the author's machine during planning and reproducing the issue's numbers exactly**: 34 slug dirs → 11 whose `cwd` is gone from disk → 5 not a git repository → **6 distinct main checkouts**.

1. The directory must still exist.
2. It must be a git repository (call `RepoRoot`, treat an error as "no" — the idiom `AddProject` already uses).
3. A linked worktree resolves to its parent. `rev-parse --show-toplevel` returns *the worktree*, so the main checkout comes from `rev-parse --path-format=absolute --git-common-dir` with its final `.git` trimmed. Without this, `claude-harness-impl` registers as a project of its own.

Then dedupe by resolved root and order by transcript mtime descending.

**Bounding.** `watcher.Entry` deliberately discards `cwd` and drops the very record types that carry it, so it cannot be reused; `discover` gets its own minimal struct. `cwd` appears by line 7 at the latest across the real store, but reaching it costs up to 249 KB because individual lines are huge — so the cap is **32 lines and 1 MiB**, not a 64 KiB total, which would fail on real data. Precedent: `maxLineBytes`/`maxPollBytes` (`watcher/tailer.go:17,21`) and the `io.LimitReader` shape of `review.ReadPreview` (`review/preview.go:29`). Unbounded transcript reads are issue #64.

**Name collisions.** `AddProject` refuses a duplicate *name* even when roots differ (`registry/commands.go:28`). Discovery makes that reachable in bulk — two `api` directories under different parents. It must report the collision against that one candidate and carry on, never abort the run.

Transcript content is untrusted (AGENTS.md, Security): only `cwd` is consumed, and it is validated against the filesystem before use. Fixtures are hand-written; real transcripts are never copied in unsanitised.

---

## Tasks

### PR0 — the plan

- [ ] **Task 0.** Commit this document to `docs/superpowers/plans/2026-09-04-m4-lifecycle.md` on `docs/m4-lifecycle-plan`. Open the PR to `develop`.

### PR1 — `fix/95-resize-behind-an-open-prompt`, closes #95

- [ ] **Task 1.1.** Move #95 to In Progress. Branch from `develop`.
- [ ] **Task 1.2.** Write the failing test in a new `internal/ui/resize_test.go`, as a **subtest table keyed by the opening keystroke** so PRs 2-5 each add one line instead of writing their own resize test:
  ```go
  m, fakes := modelWithFakes(t)
  m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
  press(m, ctrl('o')); press(m, key('n'))
  m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
  wantW, wantH := ui.PTYSize(120, 40, false) // not literal 90x36: those live in layout.go (#75)
  // assert fakes["s1"] is wantW x wantH, both now and after esc
  ```
  Named `TestModel_ResizeBehindAnOpenPromptStillReachesTheSelectedTerminal_issue95`. Run it; confirm it fails **because the fake kept its old size**, not for a compile error.
- [ ] **Task 1.3.** Add `selectedTerminal()`, rewrite `focusedTerminal()` on top of it, rename `resizeFocused` → `resizeSelected`, update the four call sites. Run the test; watch it pass.
- [ ] **Task 1.4.** Full gate. PR to `develop` closing #95; move issue and PR to Review.

### PR2 — `feat/41-rename-a-session`, closes #41

- [ ] **Task 2.1.** `registry.RenameSession(s *Store, id, title string) error` — `Load` → find → set `Title` → `Save`, in the shape of `AddSession` (`commands.go:39`). Failing test first, including the unknown-id error naming the id.
- [ ] **Task 2.2.** `internal/ui/modal.go`: the kind enum, `modal`, `modalOpen`, `onModalKey`, `lineEditor`, `onEditorKey` (appending `msg.Text`), `commitEditor`. Delete `m.prompt`, `onPromptKey`, `promptLine`; add the `Prompt()` projection. `command` takes `tea.KeyPressMsg`. Every existing `prompt_test.go` and `quit_test.go` test must pass untouched — that is the acceptance criterion for the refactor.
- [ ] **Task 2.3.** Add the "typing a capital reaches the buffer" test for the prompt (currently failing — `shift+r` appends nothing) and for rename.
- [ ] **Task 2.4.** `modalCommand` table chained from `paneCommand`'s default, with `R`/`shift+r`. Test both spellings, in the shape of `TestModel_leaderNOpensAWorktreePromptFromTheBaseKey_issue87` (`prompt_test.go:236`).
- [ ] **Task 2.5.** `internal/ui/modalview.go`: `modalLines`, `editorLines`, `modalFooter`. `renderTerminal`'s three branches; extract `emptyLines()`. `footerKeys()` gains the `modalFooter` branch first.
- [ ] **Task 2.6.** `ui.Deps.Rename RenameFunc`, defaulted in `withDefaults` to a **named** error-returning func in the `noDiff`/`noFiles` shape (`review.go:105`), so missing wiring names itself in the pane. Commit path updates `m.state.Sessions[i].Title` and rebuilds via `SetRows` — not `NewSidebar` — so the cursor does not move.
- [ ] **Task 2.7.** `ui.Run` (`run.go:44`) takes a `Deps` value instead of eight positional parameters; wire in `cmd/omatty/main.go`. Required by this change, not opportunistic: PRs 3-5 each add another injected func.
- [ ] **Task 2.8.** Add the rename row to the `resize_test.go` table and a `ctrl+c` sibling of `TestModel_ctrlCQuitsWhileAPromptIsOpen_issue28`. Full gate. PR closing #41.

### PR3 — `feat/40-kill-or-archive-a-session`, closes #40

- [ ] **Task 3.1.** `registry.RemoveSession(s *Store, id string) (Session, error)`, failing test first.
- [ ] **Task 3.2.** `watcher.Watch.tailers` becomes `map[string]*Tailer`; add `Watch.Remove(id)`, closing a displaced tailer on a duplicate `Add`. Test asserts `<-tl.Done()` for the removed tailer and not the others — `Done()` exists for exactly this (issue #65), so no `time.Sleep`.
- [ ] **Task 3.3.** `confirmBox`, `confirmChoice`, `archiveChoices(worktree bool) []confirmChoice`, `onConfirmKey`. The 2-vs-3 answers stay a data question, never a branch in the key handler.
- [ ] **Task 3.4.** The five-part teardown, in order, ending with `SetRows` and `tea.Batch(m.resizeSelected(), m.followSession())`. `Deps.TailStop`, `Deps.Archive`, `Deps.RemoveWorktree`. Test asserts the killed fake is `Closed`, the survivors are not, the id is gone from the sidebar, **which** row is selected, and that *its* fake recorded `PTYSize`.
- [ ] **Task 3.5.** Route the worktree-removal result through a new `onLifecycleMsg` called from `Update`'s `default:` **before** `onWindowFocus`/`broadcast` (`model.go:225,243`) — `Update` is 18 lines with 7 cases and two more break funlen, and an unhandled typed message would be fanned out to every emulator. Drop stale results the way `onDiffLoaded` does (`review.go:177`).
- [ ] **Task 3.6.** Resize-table row, `ctrl+c` sibling. Full gate. PR closing #40.

### PR4 — `feat/42-fuzzy-switcher`, closes #42

- [ ] **Task 4.1.** `internal/fuzzy`: `Match`, `Rank`. Table tests including the "psf ranks parser-fix above prompts-final" case and the empty-query identity.
- [ ] **Task 4.2.** `internal/ui/picklist.go`: `pickItem`, `pickList` and its methods, reusing `ScrollOffset` verbatim. `pickLines` renderer.
- [ ] **Task 4.3.** `Sidebar.SelectByID(id string) bool` beside `SetRows`, replacing `selectSession` (`model.go:348`), which only walks downward and cannot jump up.
- [ ] **Task 4.4.** `onListKey`, `openSwitcher`, items from `SidebarRows(m.state, m.statusMap())`. Commit is `SelectByID` then the `resizeSelected`/`followSession` pair.
- [ ] **Task 4.5.** Resize-table row, `ctrl+c` sibling, a "keys build the query, not the PTY" test. Full gate. PR closing #42.

### PR5 — `feat/91-discovery-picker`, closes #91

- [ ] **Task 5.1.** `paths.TranscriptsDir(home)`; refactor `Transcript` onto it. Append a case to the `TestOmattyLocations` table (`paths_test.go:61`).
- [ ] **Task 5.2.** `vcs.Git.MainCheckout(dir)`. Add a row to the `TestCLI_AllCommandsValidateTheDirectory_issue29` table (`dir_test.go:57`) and update **both** existing `FakeGit`s (`registry/fakes_test.go`, `review/fakes_test.go`). Test against a real linked worktree in `t.TempDir()`, asserting it resolves to the parent.
- [ ] **Task 5.3.** `internal/discover`: `Candidate{Name, Root, LastUsed}`, the bounded head read (32 lines / 1 MiB), the three filters, dedupe, newest-first. Hand-written fixtures. Collision reported per-candidate, never aborting.
- [ ] **Task 5.4.** `discover.Choose(cands []Candidate, in io.Reader) ([]Candidate, error)` plus the `case "discover":` arm in `dispatch`; update the usage doc block (`main.go:4-9`) and the `default:` message (`:88`).
- [ ] **Task 5.5.** `ctrl+o a` opens the candidates in the pick list with `Multi: true`; `tab` marks, `enter` registers each marked root. The scan runs as a `tea.Cmd`; a result arriving after the picker closed is dropped. `Deps.Discover`, `Deps.AddProject`.
- [ ] **Task 5.6.** Resize-table row, `ctrl+c` sibling. Full gate. PR closing #91.
- [ ] **Task 5.7.** Acceptance against the real store: `omatty discover` lists exactly the 6 known checkouts, newest first, and registering a pick leaves `state.json` as `omatty add` would have.

### Close-out

- [ ] **Task 6.1.** Open the help-modal issue (`feat`, `M4`, `area:ui`) for the keymap overflow — nothing covers it today — and ship `ctrl+o ?` in its own PR after PR2.
- [ ] **Task 6.2.** Milestone smoke test of the real binary in a real, sized PTY, against a **scratch HOME** so real sessions and transcripts are untouched:
  ```bash
  T=$SCRATCH; mkdir -p "$T/.omatty" "$T/bin"
  ln -sf "$PWD/testdata/fake-claude" "$T/bin/claude"
  # $T/.omatty/state.json: one project, two sessions, one of them a worktree
  env -i HOME="$T" PATH="$T/bin:/usr/bin:/bin" TERM=xterm-256color \
    PTY_COLS=120 PTY_ROWS=30 PTY_WAIT=4s PTY_WAIT2=3s \
    PTY_KEYS=$'\x0fx' PTY_KEYS2='y' \
    go run ./testdata/ptyrun /path/to/built/omatty > capture.txt
  go run ./testdata/screen capture.txt 120 30
  ```
  `PTY_KEYS2` exists for exactly this: the confirm must be on screen before the answer arrives. Repeat for `\x0fR` + text + `\r`, `\x0f/` + query + `\r`, and `\x0fa`. Traps: `testdata/screen` prints a trailing `|` right-edge marker (a correct 120-column row measures 121), and `env -i` drops `GOROOT`, so call go by absolute path.
- [ ] **Task 6.3.** Update the M4 row in `docs/ROADMAP.md` and the "Where things stand" table.
