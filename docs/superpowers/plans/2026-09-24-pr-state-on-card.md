# PR and CI state on the session card — implementation plan (#310)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** a session card whose branch has a pull request shows `#N` and one CI mark on line 2, read through the operator's own `gh`.

**Architecture:** a new `internal/forge` owns the `gh` CLI and folds `gh pr list --json` into `[]forge.PR`. The UI polls it once per project (boot, focus, every 60 s while focused, when a session comes to rest), keeps the answer in project-keyed maps, and matches a session to a PR at render time by its current branch.

**Tech Stack:** Go 1.26.8, the `gh` CLI (verified against 2.87.3), bubbletea v2.

**Spec:** `docs/superpowers/specs/2026-09-24-pr-state-on-card-design.md`

## Global Constraints

- `gh` is run only from `internal/forge`; `forge` becomes the eighth `os/exec` package: `.golangci.yml` subprocess rule, `scripts/depguard_test.go` `execAllowed`, AGENTS.md's list — all three together.
- One `gh pr list` per project per poll; never a per-PR call; no poll while `m.hasFocus` is false.
- Nothing about PRs is persisted (invariant 9); matching is derived at render time.
- Marks are uncoloured, like the gate strip; line 2 stays exactly 27 cells.
- Tests are named `_issue310`; no test touches the network or the real `gh`.
- Commits `type(#310): message` with the two attribution lines. The full gate after every task (as in the #311 plan).

## Review Focus

1. **A branch with several PRs** (an old closed one and a new open one). Expect the highest number. Pinned in Task 3.
2. **A main-checkout session on the base branch** (`develop`, whose promotion PR merged). Expect the branch name, not `#343 merged`. Pinned in Task 3.
3. **A poll that fails after a PR was known** (network drop, auth expiry). Expect `#N ?`, never the old `✓`. Pinned in Task 3.
4. **Focus lost for an hour.** Expect no `gh` process in that hour, one poll per project on return. Pinned in Task 2.
5. **`gh` absent from PATH.** Expect line 2 exactly as today and no further attempts. Pinned in Task 2.

---

### Task 1: `internal/forge`

**Files:** create `internal/forge/forge.go`, `internal/forge/fold.go`, `internal/forge/forge_test.go`, `internal/forge/fold_test.go`; modify `.golangci.yml`, `scripts/depguard_test.go`, `AGENTS.md` (Dependencies list and the package tree).

**Produces:** `forge.PR{Number int; Branch string; State PRState; CI CIState; Conflict bool}`; `PRState` (`Open`, `Merged`, `Closed`); `CIState` (`CINone`, `CIPassing`, `CIRunning`, `CIFailing`); `var ErrNoGH, ErrNotGitHub error`; `func Fold(raw []byte) ([]PR, error)`; `type CLI struct{ bin string }`; `func NewCLI() *CLI`; `func (c *CLI) ListPRs(repoRoot string) ([]PR, error)`; test hook `func NewCLIWithBin(bin string) *CLI` in `export_test.go`.

- [ ] **Step 1: failing fold tests** (`fold_test.go`, package `forge_test`), table-driven over JSON fixtures, one case per rule:
  - a CheckRun `COMPLETED/SUCCESS` → `CIPassing`; plus one `COMPLETED/FAILURE` → `CIFailing`; plus one `IN_PROGRESS` (no conclusion) and the rest passing → `CIRunning`; failure and in-progress together → `CIFailing` (failure outranks running);
  - a StatusContext `state: PENDING` → `CIRunning`, `FAILURE` → `CIFailing`, `SUCCESS` → `CIPassing`;
  - `SKIPPED` and `NEUTRAL` conclusions → `CIPassing`; empty rollup → `CINone`;
  - `mergeStateStatus` `DIRTY` and `BEHIND` → `Conflict`; `UNKNOWN`, `CLEAN`, `BLOCKED` → not;
  - `state` `OPEN`/`MERGED`/`CLOSED` → the three `PRState`s; `number` and `headRefName` carried;
  - malformed JSON → an error naming forge.
- [ ] **Step 2:** `go test ./internal/forge` → build failure (no package). Expected.
- [ ] **Step 3: implement `fold.go`.** Unmarshal into
  `[]struct{ Number int; HeadRefName string; State string; MergeStateStatus string; StatusCheckRollup []check }` with
  `check{ Typename string \`json:"__typename"\`; Status, Conclusion, State string }`. `rollup(checks)`: first pass any failing conclusion (`FAILURE ERROR CANCELLED TIMED_OUT ACTION_REQUIRED STARTUP_FAILURE`) or StatusContext `state` `FAILURE`/`ERROR` → failing; else any CheckRun `status != COMPLETED` or StatusContext `PENDING`/`EXPECTED` → running; else `len > 0` → passing; else none. Each branch a small function to stay under gocyclo/funlen.
- [ ] **Step 4:** fold tests pass.
- [ ] **Step 5: failing exec tests** (`forge_test.go`): a fake `gh` written by the test into `t.TempDir()` as a shell script (`#!/bin/sh` printing a fixture from an env var, or a given stderr and exit code), passed via `NewCLIWithBin`:
  - success → folded PRs, and the fake received exactly `pr list --state all --limit 100 --json number,headRefName,state,mergeStateStatus,statusCheckRollup` in the repo root (the script appends `$PWD` and `$*` to a file);
  - stderr `no git remotes found`, `none of the git remotes configured for this repository point to a known GitHub host`, `failed to run git: fatal: not a git repository` → `errors.Is(err, ErrNotGitHub)`;
  - another stderr with exit 1 → an error that is neither sentinel and carries the stderr;
  - a bin that is not on PATH / does not exist → `ErrNoGH`.
- [ ] **Step 6: implement `forge.go`.** `NewCLI()` is `&CLI{bin: "gh"}`. `ListPRs`: `exec.LookPath(c.bin)` failing → `ErrNoGH`; run with `cmd.Dir = repoRoot`, capture stderr; on error classify stderr by the three substrings → `fmt.Errorf("forge: %s: %w", stderr, ErrNotGitHub)`, else `fmt.Errorf("forge: gh pr list in %q: %s: %w", …)`; on success `Fold(out)`.
- [ ] **Step 7: the fence.** Add `"!**/internal/forge/**"` to `.golangci.yml`'s subprocess `files`, `forge` to its `desc`, `"forge"` to `execAllowed`; AGENTS.md: the list of packages that may import `os/exec` gains `forge`, "Adding an eighth package" → "Adding a ninth package", and the package tree gains `├── forge/  OUR interface over the gh CLI (#310).` Add `TestNoGhOutsideForge` beside `TestNoGitOutsideVcs` in `scripts/depguard_test.go` (`codeLineNaming(src, `"gh"`)` outside `internal/forge`).
- [ ] **Step 8:** `go test ./internal/forge ./scripts` and `golangci-lint run` green; commit `feat(#310): forge reads a repository's pull requests through gh`.

### Task 2: `internal/ui` — polling

**Files:** create `internal/ui/prpoll.go`, `internal/ui/prpoll_test.go`; modify `internal/ui/deps.go` (`PRListFunc`, `Deps.PRs`, default), `internal/ui/run.go` (`RunDeps.PRs`, passed through), `cmd/omatty/wiring.go` (`PRs: forge.NewCLI().ListPRs`), `internal/ui/model.go` (maps, `Init`, `onWindowFocus`, tick dispatch, message table), `internal/ui/status.go` (`afterStatus`), `internal/ui/removeproject.go` (`forgetProject`), `internal/ui/forget_internal_test.go` (`skipSessionMaps`), `internal/ui/export_test.go` (`PollPRs`, `PRsOf`).

**Produces:** `type PRListFunc func(projectRoot string) ([]forge.PR, error)`; `PRsLoadedMsg{Project string; PRs []forge.PR; Err error}`; `PRTickMsg`; model fields `prs map[string][]forge.PR`, `prPending map[string]bool`, `prFailed map[string]bool`, `prOff map[string]bool` (project stopped: not GitHub), `ghMissing bool`.

- [ ] **Step 1: failing tests** (`prpoll_test.go`), with a named fake `FakePRs{Lists map[string][]forge.PR; Err error; Asked []string}` keyed by project root:
  - `PollPRs()` asks each project that holds a session once, with its root (`twoProjectState` → two calls);
  - a poll in flight for a project is not repeated until it answers;
  - `PRTickMsg` re-arms itself; while blurred (`m.Update(tea.BlurMsg{})`) a tick asks nothing (**Review Focus 4**), and `tea.FocusMsg` asks every project at once;
  - a session going to `done` polls its own project only;
  - `ErrNoGH` from the first poll: no further poll on tick, focus or rest (**Review Focus 5**);
  - `ErrNotGitHub` for one project stops that project and not the other;
  - another error keeps the last list and marks the project failed; logs once per outage (the `statFailed` test's slog capture);
  - removing the empty project (`forgetProject`) clears its three entries.
- [ ] **Step 2:** run → build failure. Expected.
- [ ] **Step 3: implement.** `prpoll.go` mirrors `repostat.go`: `prEvery = 60 * time.Second`; `schedulePRTick()`; `onPRTick()` = `tea.Batch(m.pollPRs(), schedulePRTick())`; `pollPRs()` returns nil when `m.ghMissing || !m.hasFocus`, else one `pollProject(name)` per project with a session; `pollProject` skips `prPending`/`prOff`, marks pending, runs `m.prList(root)` in a `tea.Cmd`; `refreshPRs(id, before, after)` polls the session's project on `done`/`waiting` (the `refreshStat` rule); `onPRs(msg)` clears pending, then: `ErrNoGH` → `ghMissing = true` + one `slog.Info`; `ErrNotGitHub` → `prOff[project] = true` + one `slog.Info`; other error → `prFailed[project] = true` (warn once); success → `prs[project] = msg.PRs`, `delete(prFailed, project)`. Wire: `Init` appends `m.onPRTick()`; `onWindowFocus`'s `FocusMsg` returns `tea.Batch(m.pollAll(), m.pollPRs())`; the tick switch gains `case PRTickMsg: return m.onPRTick()`; `onSessionMsg` gains `case PRsLoadedMsg`; `afterStatus` batches `m.refreshPRs(...)`; `forgetProject` deletes the project from `prs`, `prPending`, `prFailed`, `prOff`; `skipSessionMaps` lists those four names; the maps are allocated in a `withPRMaps()` beside `withTurnMaps()`. Default `Deps.PRs` returns `forge.ErrNoGH`.
- [ ] **Step 4:** tests pass; full gate; commit `feat(#310): poll each project's pull requests while omatty has focus`.

### Task 3: `internal/ui` — the card

**Files:** modify `internal/ui/card.go` (`cardMeta`), create `internal/ui/prcard.go`, `internal/ui/prcard_test.go`.

**Produces:** `func (m *Model) prFor(sess registry.Session) (forge.PR, bool)`; `func (m *Model) prLabel(pr forge.PR, project string) string`.

- [ ] **Step 1: failing tests** (`prcard_test.go`), reading `CardOf(id)[1]` stripped, the 16 columns after the rail's two spaces (the `issue180` test's slice), with `SetRepoStat` and a delivered `PRsLoadedMsg`:
  - no PR → `main      +12 −3` exactly as today;
  - open passing → `#349 ✓    +12 −3`; failing `✗`; running `◍`; conflict `⚠`; failing and conflict together → `✗` (precedence);
  - no checks → `#349      +12 −3`;
  - merged → `#349 merged` with the diffstat given way only as far as it must;
  - two PRs for one branch, #12 closed and #349 open → `#349` (**Review Focus 1**);
  - a main-checkout session on `develop` with a merged `#343` → `develop …` (**Review Focus 2**); the same session with an open PR from `develop` shows it;
  - a failed poll after a known PR → `#349 ?` (**Review Focus 3**);
  - every case: line 2 is `SidebarWidth-1` cells.
- [ ] **Step 2:** run → fail on the rendered text. Expected.
- [ ] **Step 3: implement.** `prFor`: branch from `m.repoStat[id].Branch` (empty → none); scan `m.prs[sess.Project]` for `Branch == branch`, skipping non-open ones unless `sess.Worktree`, keep the highest `Number`. `prLabel`: `"#"+strconv.Itoa(n)` then `" ?"` when `m.prFailed[project]`, else `" merged"`/`" closed"`, else the mark by precedence (`✗` failing, `⚠` conflict, `◍` running, `✓` passing, none for `CINone`). `cardMeta` uses `prLabel` in place of `st.Branch` when `prFor` finds one; everything else unchanged (`fitLine` already gives the left part the room the diffstat leaves).
- [ ] **Step 4:** tests pass; full gate; commit `feat(#310): the card names a session's pull request and its CI`.

### Task 4: the boundary, docs, smoke, PR

**Files:** `docs/ROADMAP.md` ("Not on the roadmap"), `README.md` (card anatomy), `CHANGELOG.md` (Unreleased), `testdata/fake-gh` (smoke only).

- [ ] **Step 1:** ROADMAP "Not on the roadmap", under the refusals table: *Reading a pull request's state is not "cloud, accounts, sync".* omatty runs the operator's own `gh`, read-only, holds no token, polls only while focused and one call per project; it writes nothing to the forge — acting on a PR is #331's decision, not #310's. README: line 2 of the card, the `#N` + mark table. CHANGELOG: `### Added` entry for #310.
- [ ] **Step 2: smoke.** `testdata/fake-gh`: a script printing `$OMATTY_FAKE_GH_JSON` (a file path) or exiting 1 with `$OMATTY_FAKE_GH_ERR`. Scratch HOME `/tmp/omv2`, a worktree session on branch `feat-x`, `fake-gh` symlinked as `gh` in `$T/bin`. Runs, each read on line 2: passing `✓`; failing `✗`; conflict `⚠`; merged; `gh` removed from PATH → the branch as today and one log line. Then one read-only `gh pr list` against this repo through `forge` (a tiny `go run` of `forge.NewCLI().ListPRs(".")`) folds without error.
- [ ] **Step 3:** full gate; commit `docs(#310): the forge boundary, README and changelog`; push; final whole-branch review (Opus); fix pass; PR into `develop` (`Closes #310`); CI green on both runners → merge under the standing approval; board to Done.
