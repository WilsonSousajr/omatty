# PR and CI state on the session card — design (#310)

Approved 2026-09-24 (sections 1-3; section 1 separately). One issue, one
implementation plan, one PR.

## Context

#310 is the last issue in the verification-core batch (standing approval: merge
the PR once gate + CI + read smoke are green). omatty's card shows the **local**
verdict (the gate strip, M9) and nothing about the **remote** one; the field asks
for it (Orca #18484/#18485/#18487; `brizzai/fleet` ships it). It also unblocks
#331 (ship from the card), which needs the remote verdict.

Decisions already taken with the user:

| Decision | Choice |
|---|---|
| Card placement | Once a PR exists, **line 2's branch becomes `#349` + one CI mark**; no PR → branch as today; line 3 stays the local gate |
| Data source | **One `gh pr list` per registered project per poll**, matched to sessions by branch — never per-PR calls (they tripped GitHub's secondary rate limit here before) |

## 1. Reading PR state — `internal/forge`

- New package `internal/forge`, OUR interface over the `gh` CLI (invariant 4 in
  spirit, like `vcs`/`detach`). It is the **eighth `os/exec` package**: update
  `.golangci.yml`'s depguard allowlist, AGENTS.md's list ("Adding an eighth
  package is a decision"), and `TestDepguard_ExecAllowlistMatchesReality`.
- `forge.CLI.ListPRs(repoRoot) ([]PR, error)` runs, in the project root:
  `gh pr list --state all --limit 100 --json number,headRefName,state,mergeStateStatus,statusCheckRollup`
  (fields verified against gh 2.87.3 on this repo). gh resolves the repo from
  the remote and uses the user's own auth; omatty stores nothing.
- `PR{Number int; Branch string; State PRState; CI CIState; Conflict bool}`,
  folded from the JSON by a pure function:
  - `State`: Open / Merged / Closed.
  - `CI` precedence: any `FAILURE|ERROR|CANCELLED|TIMED_OUT|ACTION_REQUIRED|STARTUP_FAILURE`
    → Failing; else any CheckRun not `COMPLETED` or StatusContext `PENDING|EXPECTED`
    → Running; else any checks → Passing (`SKIPPED`/`NEUTRAL` pass); none → None.
  - `Conflict`: `mergeStateStatus` `DIRTY` or `BEHIND`; `UNKNOWN` (GitHub still
    computing) is not a conflict.
- Typed failures: `ErrNoGH` (gh not on PATH — `exec.LookPath` pre-flight, the
  gate's lesson that a missing tool must not read as a failed call),
  `ErrNotGitHub` (gh cannot resolve a GitHub repo); anything else is an ordinary
  error carrying gh's stderr.

## 2. Polling and matching — `internal/ui`

- **Seam:** `Deps.PRs PRListFunc` (`func(projectRoot string) ([]forge.PR, error)`),
  wired in `cmd/omatty/wiring.go` `tuiDeps` to `forge.NewCLI().ListPRs`; unwired
  default returns `forge.ErrNoGH` (every test's model polls nothing).
- **Triggers** (reusing the diffstat poll's shape in `internal/ui/repostat.go`):
  1. a session comes to rest (`done`/`waiting`) → poll its project
     (beside `refreshStat` in `afterStatus`, `status.go`);
  2. window focus returns → poll every project (beside `pollAll` in
     `onWindowFocus`, `model.go:487`);
  3. a `PRTickMsg` every **60s**, polling every project that holds a session —
     **never while blurred** (the `m.hasFocus` gate `pollAll` already uses, #314);
  4. boot, once.
- **One call in flight per project** (`prPending[project]`), as `statPending` does
  per session. Cost: one GraphQL call per project per minute while focused.
- **Matching** a session to a PR, at render time, derived not stored (invariant 9):
  the session's current branch is `m.repoStat[id].Branch` (the stat poll's
  `CurrentBranch`); the PR is the **highest-numbered** one whose `headRefName`
  equals it. Worktree sessions take any state; a **main-checkout session takes
  open PRs only**, so a checkout sitting on `develop` never shows the merged
  promotion PR `#343`. Empty or detached branch → no PR.
- **State:** `prs map[string][]forge.PR`, `prPending`, `prFailed` keyed by
  **project name** — listed in `skipSessionMaps` (`forget_internal_test.go`, the
  hatch built for project-keyed maps) and deleted in `forgetProject`
  (`removeproject.go:61`). Never persisted.

## 3. The card, degradation, the boundary, tests

- **Line 2** (`cardMeta`, `card.go:72`; `metaCols` = 16): with a PR, the branch
  slot becomes `#349` + a space + one mark; the diffstat keeps its place.
  Marks follow the gate strip (uncoloured — the colour rule reserves green/red/
  amber for added/removed/comment): open PR by precedence **✗ failing > ⚠
  conflict/behind > ◍ running > ✓ passing**, no checks → `#349` alone;
  `#349 merged`, `#349 closed` (the diffstat then gives way if it must).
  An open PR's label is never cut: when it and the diffstat do not both fit -
  `#1234 ✓` beside `+312 −1.2k` - the label drops the space before its mark,
  then the diffstat sheds whole parts, `+312 −1.2k` to `+312` to nothing,
  never a number cut mid-way (#357).
- **Unknown is never passing** (Orca #18484): if the project's last poll failed
  while a PR was known, the mark is `?` until a poll succeeds.
- **Degradation**, as quiet as `internal/detach` without dtach: `ErrNoGH` → stop
  polling for the run; `ErrNotGitHub` → stop polling that project; both leave
  line 2 exactly as today and log once. Any other error keeps the last PR with
  `?` and logs once per outage (the `statFailed` pattern). No footer notice.
- **The boundary, written down before it is built** (the issue's counter-argument):
  `docs/ROADMAP.md` "Not on the roadmap" gains a line — reading PR/CI state
  through the operator's own `gh` is verification: read-only, no token held, no
  polling while blurred; omatty writes nothing to the forge (a ship action is
  #331's decision, not this one's). README's card anatomy and the CHANGELOG
  gain #310.
- **Tests** (all `_issue310`, TDD):
  - `forge`: the fold against JSON fixtures — every CI precedence case, a
    StatusContext, an empty rollup, DIRTY/BEHIND/UNKNOWN, merged/closed; the exec
    path against a fake `gh` script on a scratch PATH (like `testdata/fake-claude`);
    `ErrNoGH` with gh absent; no network anywhere.
  - `ui`: each card state renders in 27 cells beside a diffstat; highest-number
    match; main checkout ignores a merged PR; polls on rest / focus / tick,
    none while blurred; one in flight per project; a failed poll shows `?`;
    `ErrNoGH` stops polling and leaves line 2 as today; `forgetProject` clears;
    the reflection test still passes with the skip entries.
  - `TestDepguard_ExecAllowlistMatchesReality` with `forge` added.

## Critical files

- New: `internal/forge/forge.go`, `internal/forge/fold.go`, tests + `testdata/fake-gh`
- `internal/ui/`: `card.go` (`cardMeta`), `repostat.go` (poll pattern), new
  `prpoll.go`, `deps.go`, `model.go` (`onWindowFocus`, maps, dispatch),
  `status.go` (`afterStatus`), `removeproject.go` (`forgetProject`),
  `forget_internal_test.go` (`skipSessionMaps`)
- `cmd/omatty/wiring.go` (`tuiDeps`), `internal/ui/run.go` (`RunDeps`)
- `.golangci.yml`, `AGENTS.md`, `docs/ROADMAP.md`, `README.md`, `CHANGELOG.md`

## Verification

- Full gate: gofmt, vet, `go mod tidy -diff`, golangci-lint, check-deps,
  govulncheck, `go test ./... -race`, coverage ≥ 90, C.R.A.P. < 12, build.
- Real-PTY smoke: scratch `HOME` at `/tmp/omv2`, `fake-claude`, and a
  `testdata/fake-gh` on PATH printing canned JSON for a worktree session's branch;
  read line 2 for `#N ✓`, then switch the fixture to failing / conflict / merged
  and read `✗` / `⚠` / `merged`; remove `gh` from PATH and confirm line 2 is the
  branch as today with one log line. One read-only run against the real `gh` on
  this repo confirms the live fields fold without error.
- CI green on both runners, then merge under the standing approval.
