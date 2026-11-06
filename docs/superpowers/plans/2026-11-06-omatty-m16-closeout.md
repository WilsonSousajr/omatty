# M16 close-out (#594, #585, #596) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Finish M16: every forge waits out a rate limit and asks less; a forge
under a relative URL root is read at the right place; Azure DevOps Server is
read with a token of its own.

**Architecture:** Three slices, one branch and PR each, into `develop`. Slice 1
adds a clock to the forge Router, lifetimes to the CI cache, a finished-list
cache and a typed `RateLimitError`, and a per-host wait in the TUI. Slice 2 lets
a `[forge.hosts]` key carry a root path, moved off `Remote.Path` into
`Remote.Root` and put back by `webBase`. Slice 3 adds Azure DevOps Server's
bound token and a once-per-instance API-version probe.

**Tech Stack:** Go 1.26.10, stdlib `testing`, `net/http/httptest`, bubbletea v2
(TUI tests through the existing Model test helpers), BurntSushi/toml.

**Spec:** `docs/superpowers/specs/2026-11-06-omatty-m16-closeout-design.md`

## Global Constraints

- Read the spec before each slice. AGENTS.md rules apply to every task:
  - functions are 4–20 lines;
  - files stay under 500 lines;
  - C.R.A.P. stays under 12;
  - no more than 2 levels of indentation;
  - error messages carry the offending value and the expected shape;
  - every exported identifier has a doc comment with a usage example.
- TDD for every task: write the failing test, run it and **read the failure**
  (it must fail for the bug's reason), implement, then watch it pass. Name tests
  `Test<Thing>_<Behaviour>_issueNNN`.
- Fakes are named types in `*_test.go`: `FakeBitbucketAPI`, `FakeGitLabAPI`,
  `FakeGiteaAPI`, `FakeBitbucketDCAPI`, `FakeAzureAPI`, `FakeVars`, and the
  `TestEnv`/`NewTestRouter` pair in `internal/infra/forge/export_test.go`.
- No `time.Sleep`; time is a fake clock (`func() time.Time` over a variable).
- Tokens never appear in an error, a log line or a URL. Every new path that
  borrows one gets a test asserting that.
- `internal/domain/forge` imports only the stdlib. The TUI imports
  `internal/domain/forge` and never `internal/infra/...`.
- Commits are `type(#N): message` and end with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. No session URL
  anywhere.
- Work happens in a git worktree off `origin/develop`, outside iCloud's
  `~/Documents`. Branches:
  - `docs/m16-closeout-spec` (this plan and the spec)
  - `perf/594-rate-limit`
  - `feat/585-relative-root`
  - `feat/596-azure-server`
- The full gate from AGENTS.md runs before each PR:

  ```
  gofmt -l .; go vet ./...; go mod tidy -diff; golangci-lint run; govulncheck ./...;
  ./scripts/check-deps.sh; ./scripts/check-file-length.sh 500; ./scripts/check-layers.sh -enforce;
  shellcheck scripts/*.sh; go test ./... -race; ./scripts/check-coverage.sh 90; ./scripts/check-crap.sh 12
  ```

## Review Focus

1. **A 429 from a CI-status read in the middle of a fill.** The list call must
   return `RateLimitError`, not a list with some CI marks silently missing.
   Covered in Task 3.
2. **A sibling project on the same host.** After project A gets a 429, project
   B on that host must not be asked either, even though B never saw the 429.
   Covered in Task 6.
3. **A tea login or `BITBUCKET_DC_URL` that carries a path, on a remote with no
   rooted key.** It must still bind exactly as before. Regression tests in
   Task 10.
4. **An ssh remote on a host with both a bare key and a rooted key.** The bare
   key wins, with no refusal. Covered in Task 9.
5. **Azure's PAT or az token while reading a Server.** Neither may appear in any
   request to the Server, or in any error. Covered in Task 12.

---

## Task 0: Spec and plan PR

**Files:** `docs/superpowers/specs/2026-11-06-omatty-m16-closeout-design.md`,
`docs/superpowers/plans/2026-11-06-omatty-m16-closeout.md`

- [ ] **Step 1:** Commit both:
  `docs(#594): M16 close-out design and plan for #594, #585, #596`.
- [ ] **Step 2:** Push, then open a PR into `develop` whose body links all
  three issues. Do not write `Closes`. Put the PR on board 13 in Review.
- [ ] **Step 3:** After it merges, move #594 to In Progress and branch
  `perf/594-rate-limit` off the updated `origin/develop`.

---

## Slice 1: #594 (branch `perf/594-rate-limit`)

### Task 1: `RateLimitError` in the domain

**Files:**
- Modify: `internal/domain/forge/errors.go`
- Test: `internal/domain/forge/errors_test.go` (create it if it does not exist)

**Interfaces:**
- Produces:
  ```go
  // RateLimitError is a forge that said "later": a 429, or a 403 that rateLimited recognises.
  type RateLimitError struct {
      Host  string    // the host that refused, with its port when it has one
      Until time.Time // when it said to ask again; zero when it named no time
  }
  func (e *RateLimitError) Error() string
  ```

- [ ] **Step 1: Write the failing test.**
  `TestRateLimitError_NamesHostAndTime_issue594`:
  - `(&RateLimitError{Host: "bitbucket.org", Until: time.Date(2026,11,6,15,4,0,0,time.UTC)}).Error()`
    contains `"bitbucket.org"` and `"15:04"`;
  - with a zero `Until`, it contains `"bitbucket.org"` and `"no time"`.
- [ ] **Step 2:** `go test ./internal/domain/forge -run RateLimit` fails to
  compile because the type is undefined.
- [ ] **Step 3:** Implement it. The text is
  `"forge: <host> is rate-limiting omatty until 15:04 UTC"`, or
  `"forge: <host> is rate-limiting omatty and named no time to ask again"`.
- [ ] **Step 4:** The test passes.
- [ ] **Step 5:** Commit `perf(#594): a typed rate limit in the forge vocabulary`.

### Task 2: REST and CLI answers become `RateLimitError`

**Files:**
- Create: `internal/infra/forge/ratelimit.go`, which holds `retryAt` and
  `saysRateLimit`
- Modify:
  - `internal/infra/forge/rest.go` (`answerError`, `send`)
  - `internal/infra/forge/fetch.go` (`statusError`)
  - `internal/infra/forge/forge.go` (`classify`)
- Test: `internal/infra/forge/ratelimit_test.go`

**Interfaces:**
- Produces:
  - `func retryAt(h http.Header, now time.Time) time.Time` reads `Retry-After`
    (integer seconds, or an `http.ParseTime` date), then `X-Ratelimit-Reset`
    (epoch seconds). It returns zero when neither is present or both fail to
    parse.
  - `func saysRateLimit(text string) bool` is a case-insensitive check for
    `"rate limit"`. Move the body check out of `rateLimited` so the two share
    it.
- `restClient` needs a clock for `retryAt`. Add a field
  `now func() time.Time` to `restClient`, set by `newREST(now)`. Task 3 adds
  `Options.Now`; until then, call `newREST(time.Now)` in `NewRouter`.

- [ ] **Step 1: Write the failing tests.**
  - `TestRetryAt_SecondsAndDate_issue594`:
    - with now=T, `Retry-After: 120` gives T+2m;
    - `Retry-After: Fri, 06 Nov 2026 15:04:00 GMT` gives that instant;
    - `X-Ratelimit-Reset: <T+300 as epoch>` gives T+5m;
    - `Retry-After: soon` with no reset header gives zero.
  - `TestAnswerError_429IsARateLimit_issue594`: an httptest server answering
    429 with `Retry-After: 60`. `restClient.get` returns an error that
    `errors.As` turns into a `*dforge.RateLimitError` with `Host` set to the
    server's host and `Until` = now+60s, using a fixed `now`.
  - `TestAnswerError_GitHub403RateLimitIsNotAuth_issue594`: a 403 with
    `X-Ratelimit-Remaining: 0` gives a `RateLimitError`, and
    `errors.As(err, &*dforge.AuthError)` is false.
  - `TestStatusError_CLI429_issue594`: `statusError("gitlab.com","glab",429,"…")`
    is a `RateLimitError` with zero `Until`.
  - `TestStatusError_CLI403SayingRateLimitIsNotAuth_issue594`:
    `statusError(h,"gh",403,"API rate limit exceeded for user")` is a
    `RateLimitError`.
  - `TestClassify_GhRateLimitText_issue594`:
    `classify(root,"pr list","GraphQL: API rate limit exceeded",errExit)` is a
    `RateLimitError`.
- [ ] **Step 2:** Run `go test ./internal/infra/forge -run 'RetryAt|AnswerError_|StatusError_|Classify_Gh' -race`.
  It fails: undefined `retryAt`, and the 429 is not typed.
- [ ] **Step 3: Implement.**
  - In `answerError`, add a case before the 401/403 case:
    `status == 429 || status == 403 && rateLimited(resp)` returns
    `&dforge.RateLimitError{Host: host, Until: retryAt(resp.Header, c.now())}`.
    `answerError` therefore gains a `now time.Time` parameter, passed in by
    `send`.
  - In `statusError`: a 429, or a 401/403 whose `said` matches
    `saysRateLimit`, returns `&dforge.RateLimitError{Host: host}`.
  - In `classify`: after the `notGitHub` loop, when `saysRateLimit(stderr)`,
    return `&dforge.RateLimitError{Host: "github.com"}`. gh does not name its
    host here, and this path is gh against github.com only.
- [ ] **Step 4:** The tests pass. Then run the whole package with
  `go test ./internal/infra/forge -race` to catch existing tests asserting the
  old 403 or 429 text.
- [ ] **Step 5:** Commit `perf(#594): a 429 is a rate limit with the time the forge named`.

### Task 3: A clock, and CI verdicts that expire

**Files:**
- Modify:
  - `internal/infra/forge/cicache.go`
  - `internal/infra/forge/router.go` (`Options.Now`, `Router.now`, `NewRouter`)
  - every `listPRs` that calls `ci.fill`: `gitlab.go`, `gitea.go`,
    `bitbucket.go`, `bitbucketdc.go`, `azure.go`
- Test: `internal/infra/forge/cicache_test.go`

**Interfaces:**
- Produces:
  - `Options.Now func() time.Time`. Nil means `time.Now`.
  - `func newCICache(now func() time.Time) *ciCache`
  - `func (c *ciCache) fill(ctx, prs, key, ask) error`. It returns the first
    `*dforge.RateLimitError` an ask returned, and stops scheduling further asks
    once it has one. Asks already running finish.
  - Constants, each with a comment saying why it has that value:

    | Constant | Value |
    |---|---|
    | `ciKeepRunning` | `time.Minute` |
    | `ciKeepFailing` | `5 * time.Minute` |
    | `ciKeepNone` | `5 * time.Minute` |

  - `done` becomes `map[string]ciVerdict`, where
    `type ciVerdict struct{ state dforge.CIState; until time.Time }`. A zero
    `until` means it is kept for good, which only a pass gets.
- Consumes: `dforge.RateLimitError` (Task 1).

- [ ] **Step 1: Write the failing tests.** All use a fake clock `now := T`.
  - `TestCICache_KeepsNoneForFiveMinutes_issue594`:
    - remember `CINone` for key k at T, then `unknown` at T+4m59s does not
      return k's index;
    - at T+5m it does.
  - `TestCICache_AsksRunningAgainAfterAMinute_issue594`: the same check, with
    `CIRunning` and a 1-minute boundary.
  - `TestCICache_PassIsKeptForGood_issue594`: at T+24h, a pass is still known.
  - `TestCIFill_RateLimitStopsAsks_issue594`:
    - 10 open PRs; the ask function returns `&RateLimitError{}` on its first
      call and counts its calls;
    - `fill` returns an error that `errors.As` matches;
    - the count is at most `ciParallel`;
    - no PR is given a CI state from the failed ask.
  - `TestBitbucket_ListReturnsRateLimitFromCI_issue594`: a `FakeBitbucketAPI`
    whose statuses endpoint answers 429. `Router.ListPRs` returns a
    `RateLimitError`.
- [ ] **Step 2:** Run `go test ./internal/infra/forge -run 'CICache|CIFill|Bitbucket_ListReturnsRateLimit' -race`.
  It fails to compile (`newCICache` takes no argument), then the assertions
  fail.
- [ ] **Step 3: Implement.**
  - `remember` stores `until = now()+keep(state)`.
  - `known` treats an expired entry as unknown and deletes it.
  - `fill` collects the first rate-limit error under the mutex, and checks it
    before taking each slot.
  - Each backend's `listPRs` does `if err := X.ci.fill(...); err != nil { return nil, err }`.
  - `NewRouter` uses `o.Now`, or `time.Now` when that is nil, for both
    `newCICache` and `newREST`.
- [ ] **Step 4:** The tests pass, and so does the whole forge package.
- [ ] **Step 5:** Commit `perf(#594): CI verdicts other than a pass expire instead of being asked every poll`.

### Task 4: The finished list is re-read only when it can have changed

**Files:**
- Create: `internal/infra/forge/finished.go`
- Modify: the `listPRs` of `forge.go` (ghCLI), `github_http.go`, `gitlab.go`,
  `gitea.go`, `bitbucket.go`, `bitbucketdc.go` and `azure.go`. Each backend
  struct gains `fin *finishedCache`, set from `Router.fin` in its `pick*`.
- Test: `internal/infra/forge/finished_test.go`, plus the acceptance test in
  `internal/infra/forge/bitbucket_test.go`

**Interfaces:**
- Produces:
  ```go
  const finishedEvery = 10 // polls between re-reads of an unchanged project
  type finishedCache struct{ mu sync.Mutex; seen map[string]finishedRead }
  type finishedRead struct{ open string; finished []dforge.PR; polls int }
  func newFinishedCache() *finishedCache
  // read is finished for key: cached when open's set (numbers and heads) is the one
  // last read and fewer than finishedEvery polls have passed, else fetch's answer, kept.
  func (c *finishedCache) read(ctx context.Context, key string, open []dforge.PR,
      fetch func(context.Context) ([]dforge.PR, error)) ([]dforge.PR, error)
  func openSet(open []dforge.PR) string // sorted "N@head" joined by ","
  ```
- Each backend reads its open list first, then calls
  `b.fin.read(ctx, <host+"/"+slug>, open, <its finished fetch>)`.
  - **GitLab's fetch** reads both of its finished lists: the merged one and the
    closed one in `glLists`.
  - **Bitbucket Cloud** no longer reads its open and finished lists
    concurrently, since the finished read is now usually skipped.
  - **gh's key** is `repoRoot`, because ghCLI has no `Remote`.
  - **A finished fetch that fails** fails the list, as today, and nothing is
    cached.

- [ ] **Step 1: Write the failing tests.**
  - `TestFinished_ReusedWhileOpenSetIsUnchanged_issue594`: the fetch counter is
    1 after 3 reads with the same open set.
  - `TestFinished_RereadWhenOpenSetChanges_issue594`: when a PR's head
    changes, the counter is 2. When a PR leaves the open set, it is 3.
  - `TestFinished_RereadEveryTenPolls_issue594`: the counter is 2 after 11
    reads with an unchanged set.
  - `TestFinished_ErrorIsNotCached_issue594`: the first fetch errors, and the
    next read fetches again.
  - `TestBitbucket_NoCIProjectStaysUnderTheHourlyLimit_issue594`. This is the
    acceptance test:
    - one counting `FakeBitbucketAPI` serves two repositories, each with 10
      open PRs and zero statuses;
    - drive 60 `ListPRs` per project, advancing the fake clock 1 minute each
      time;
    - the total request count is below 1000;
    - polls 6 through 10 of one project make exactly 1 request each. Those
      polls are between CI expiries and before the 10-poll re-read, so this
      checks the "one list read per poll" steady state.
- [ ] **Step 2:** Run `go test ./internal/infra/forge -run 'Finished_|NoCIProject' -race`.
  It fails: `finishedCache` is undefined, and once it exists the count is
  about 1,440 per project.
- [ ] **Step 3:** Implement `finished.go`, then switch each backend over.
  Keep every `listPRs` within 20 lines: the finished fetch becomes a method
  such as `func (b bbBackend) finished(ctx context.Context) ([]dforge.PR, error)`.
- [ ] **Step 4:** The tests pass, and so does the whole forge package. The
  existing tests asserting finished PRs appear must still pass, because the
  first read always fetches.
- [ ] **Step 5:** Commit `perf(#594): the finished list is re-read only when the open set moves`.

### Task 5: The TUI can tell which host a project is on

**Files:**
- Modify:
  - `internal/infra/forge/router.go`: add `func (r *Router) Host(repoRoot string) string`
  - `internal/tui/app/deps.go`: add a `Host HostFunc` field
  - `internal/tui/app/forgestop.go`: add the type and an unwired default
  - `internal/tui/app/model.go`: add the `hostOf` field and wire it in the
    Deps assignment line
  - `cmd/omatty/wiring.go`: `Host: router.Host`
- Test:
  - `internal/infra/forge/router_test.go`
  - `cmd/omatty/wiring_test.go`, next to the existing `deps.Label` assertion

**Interfaces:**
- Produces:
  - `Router.Host(repoRoot) string` is the resolved remote's `Host`, or `""`
    when the project is not resolved yet. It never resolves; it uses `cached`,
    as `Label` does.
  - `type HostFunc func(projectRoot string) string`
  - `func unwiredHost(string) string { return "" }`
  - `func (m *Model) host(project string) string` is `m.hostOf(m.projectRoot(project))`,
    falling back to `project` itself when that is empty, so an unresolved
    project still waits on its own.

- [ ] **Step 1: Write the failing tests.**
  - `TestRouter_HostIsTheResolvedRemotes_issue594`:
    - before any call it returns `""`;
    - after `ListPRs` on a `bitbucket.org` remote it returns `"bitbucket.org"`.
  - `TestWiring_HostIsWired_issue594`: `deps.Host("/nowhere") == ""`, and the
    func is not nil.
- [ ] **Step 2:** Run the tests. They fail with an undefined method or field.
- [ ] **Step 3:** Implement and wire it.
- [ ] **Step 4:** The tests pass.
- [ ] **Step 5:** Commit `perf(#594): the TUI can ask which host a project is on`.

### Task 6: The TUI waits out a rate limit, per host

**Files:**
- Create: `internal/tui/app/forgewait.go`
- Modify:
  - `internal/tui/app/prpoll.go` (`mayAsk`, `prFailure`, `onPRs`, `withPRMaps`)
  - `internal/tui/app/issuepoll.go` (`issueFailure`, `onIssues`)
  - `internal/tui/app/trackerrender.go` (`trackerNote`)
- Test: `internal/tui/app/forgewait_test.go`

**Interfaces:**
- Produces, in `forgewait.go`:
  ```go
  const waitFirst, waitMost = 5 * time.Minute, time.Hour
  type hostWait struct{ until time.Time; backoff time.Duration }
  // Model gains forgeWait map[string]hostWait, allocated in withPRMaps.
  func (m *Model) waitHost(project string, limited *forge.RateLimitError)
  func (m *Model) hostWaiting(project string) (time.Time, bool) // until, and whether m.clock() is before it
  func (m *Model) hostAnswered(project string)                  // deletes the host's wait
  func waitPhrase(host string, until time.Time) string          // "rate-limited by <host> until 15:04; omatty asks again then."
  ```
- `waitHost` works like this:
  - a non-zero `limited.Until` gives `until = limited.Until`, with `backoff`
    unchanged;
  - a zero one gives `backoff = clamp(prev*2, waitFirst, waitMost)` and
    `until = clock()+backoff`.
- Consumes: `forge.RateLimitError` (Task 1) and `m.host` (Task 5).

- [ ] **Step 1: Write the failing tests.** They drive the Model through its
  existing message types (`PRsLoadedMsg`, `IssuesLoadedMsg`) and the fake
  clock used by the prpoll tests.
  - `TestPRPoll_WaitsForRetryAfter_issue594`:
    - project A answers `RateLimitError{Until: T+10m}`;
    - at T+9m, `mayAsk` for A is false;
    - at T+10m it is true.
  - `TestPRPoll_SiblingOnSameHostWaits_issue594`:
    - A and B both map to host `bitbucket.org` through the fake `Host` dep;
    - after A's 429, B's `mayAsk` is false for both the PR and the issue maps.
  - `TestPRPoll_OtherHostIsNotPaused_issue594`: C on `gitlab.com` is still
    asked.
  - `TestPRPoll_BackoffDoublesToAnHour_issue594`: zero-Until errors in a row
    give waits of 5m, 10m, 20m, 40m, 60m and 60m.
  - `TestPRPoll_AnswerClearsTheWait_issue594`: after a successful
    `PRsLoadedMsg` for B, A is no longer waiting, and the next zero-Until wait
    starts again at 5m.
  - `TestPRPoll_RateLimitIsNotAStop_issue594`: after the 429,
    `m.forgeStopped[A]` is nil and `m.prFailed[A]` is true. The card's label
    still ends in `" ?"`.
  - `TestTracker_SaysRateLimitedUntil_issue594`: `trackerNote()` for A, after a
    429 with Until 15:04 in the clock's location, contains
    `"rate-limited by bitbucket.org until 15:04"`.
- [ ] **Step 2:** Run `go test ./internal/tui/app -run 'PRPoll_|Tracker_SaysRateLimited' -race`.
  It fails: undefined symbols, then the assertions.
- [ ] **Step 3: Implement.**
  - `prFailure` and `issueFailure`: when `errors.As` finds a rate limit, call
    `waitHost` and then fall through to today's failed-mark path. It is never
    treated as a stop.
  - `onPRs` and `onIssues` success: call `hostAnswered`.
  - `mayAsk`: first line `if _, waiting := m.hostWaiting(project); waiting { return false }`.
    If gocyclo complains, split the existing stop check into
    `func (m *Model) forgeHeld(pending map[string]bool, project string) bool`.
  - `trackerNote`: a new case after `forgeStopped`, for a project whose host
    is waiting, returning `waitPhrase`.
- [ ] **Step 4:** The tests pass, and so does `go test ./internal/tui/app -race`.
- [ ] **Step 5:** Commit `perf(#594): a rate-limited host is waited out, every project on it`.

### Task 7: Docs, gate, smoke, PR

**Files:**
- `README.md`: the forge section's polling and cost paragraph gets one
  sentence on the rate-limit wait and the backoff.
- `docs/ROADMAP.md`: the M16 #594 line is marked done.
- `CHANGELOG.md`: nothing. The promotion PR writes it.

- [ ] **Step 1:** Edit the docs.
- [ ] **Step 2:** Run the full gate (Global Constraints). Every step must be
  clean, and `check-crap.sh 12` must run against a fresh `cover.out`
  (memory: stale profile trap, #385).
- [ ] **Step 3:** Run the smoke test against a short scratch HOME:
  `go run ./testdata/ptyrun omatty`. A person reads the screen; there is no
  forge change to see here, so the point is that omatty still boots and quits.
- [ ] **Step 4:** Commit `docs(#594): the rate-limit wait in README and ROADMAP`.
  Push, then open PR `perf(#594): every forge waits out a rate limit and asks less`
  with `Closes #594`, and move the card to Review.
- [ ] **Step 5:** Once CI is green on both runners, ask the operator before
  merging (AGENTS.md: PR evaluation asks before merging). After the merge,
  move the card to Done.

---

## Slice 2: #585 (branch `feat/585-relative-root`, off `develop` after slice 1)

### Task 8: `[forge.hosts]` accepts a key with a root

**Files:**
- Modify: `internal/infra/config/config.go` (`refuseBadForgeHosts`, plus a new
  `rootSegment` regexp)
- Modify: `internal/infra/forge/kind.go`: add
  `func SplitHostKey(key string) (host string, root []string)`, exported so
  config and forge share one parser
- Test: `internal/infra/config/config_test.go` and
  `internal/infra/forge/kind_test.go`

**Interfaces:**
- Produces: `forge.SplitHostKey("host.corp/gitlab/x")` returns
  `("host.corp", []string{"gitlab","x"})`, and `SplitHostKey("host")` returns
  `("host", nil)`.
- `rootSegment` is `^[A-Za-z0-9._~-]+$`, excluding `.` and `..`.

- [ ] **Step 1: Write the failing tests.**
  - `TestSplitHostKey_issue585`: covers the cases above.
  - `TestConfig_AcceptsARootInAHostKey_issue585`: TOML with
    `"host.corp.example/gitlab" = "gitlab"` loads, and
    `cfg.Forge.Hosts["host.corp.example/gitlab"] == forge.KindGitLab`.
  - `TestConfig_RefusesABadRoot_issue585`: a table test, where each case's
    error message contains the key and `"want"`:

    | Key | Why it is refused |
    |---|---|
    | `"https://h/x"` | scheme |
    | `"h:8443/x"` | port |
    | `"h/x/../y"` | `..` |
    | `"h//x"` | empty segment |
    | `"h/x?y"` | query |
    | `"h/"` | trailing slash, so an empty segment |

  - `TestConfig_SameRootTwiceIsRefused_issue585`: `"H/gitlab"` and
    `"h/gitlab"` are refused as duplicates. `"h/gitlab"` and `"h/GitLab"` are
    both accepted.
- [ ] **Step 2:** The tests fail: undefined `SplitHostKey`, and "is not a host
  name".
- [ ] **Step 3:** Implement. The refusal text for a bad root is
  `config %s: forge.hosts key %q has a root %q that is not a plain URL path, want one such as "host.corp.example/gitlab"`.
  The duplicate key is the lowercased host plus `/` plus the root as written.
- [ ] **Step 4:** The tests pass.
- [ ] **Step 5:** Commit `feat(#585): a [forge.hosts] key may name a root path`.

### Task 9: A remote is resolved against the longest root

**Files:**
- Modify:
  - `internal/infra/forge/remote.go`: add `Root []string`, and update the doc
    comment on `Slug`
  - `internal/infra/forge/kind.go`: `Hosts.Resolve`
  - `internal/infra/forge/alias.go`: `name` calls `Resolve` in both places it
    calls `KindOf`
  - `internal/infra/forge/gitlab.go`: `webBase` appends `"/"+strings.Join(Root,"/")`
    when Root is non-empty
  - `internal/domain/forge/errors.go`: `AmbiguousHostError`
- Test: `internal/infra/forge/kind_test.go`, `internal/infra/forge/router_test.go`,
  `internal/domain/forge/errors_test.go`

**Interfaces:**
- Produces:
  ```go
  // in domain/forge
  type AmbiguousHostError struct{ Host string }
  func (e *AmbiguousHostError) Error() string // names the host and "clone over https"
  func (e *AmbiguousHostError) Unwrap() error { return ErrNoForge }
  // in infra/forge
  func (h Hosts) Resolve(r Remote) (Remote, Kind, error)
  ```
- `Resolve` rules:
  - **http(s):** among the keys whose host matches case-insensitively, take
    the one with the longest root that is a segment prefix of `r.Path` and
    leaves at least one segment.
  - **ssh, no bare key:** if the host has exactly one rooted key, set
    `Root = that root`. If it has several, return `&dforge.AmbiguousHostError{Host}`.
  - **Everything else:** `h.KindOf(r.Host)`, with Root nil.

- [ ] **Step 1: Write the failing tests.**
  - `TestHosts_LongestRootWins_issue585`:
    - keys `h/a` = gitlab and `h/a/b` = forgejo;
    - remote `https://h/a/b/g/p` gives kind gitea, Root `[a b]` and Path `[g p]`;
    - `https://h/a/g/p` gives gitlab with Root `[a]`.
  - `TestHosts_RootMustLeaveARepository_issue585`: with key `h/a`, the remote
    `https://h/a` does not match by root.
  - `TestHosts_SSHUsesTheOnlyRoot_issue585`: with key `h/gitlab` alone, the
    remote `git@h:g/p` gets Root `[gitlab]` and Path `[g p]`.
  - `TestHosts_SSHPrefersABareKey_issue585`: with keys `h` and `h/gitlab`,
    the ssh remote gets no Root and the bare key's kind.
  - `TestHosts_SSHWithTwoRootsRefusesAloud_issue585`: the result matches both
    `errors.As(&AmbiguousHostError)` and `errors.Is(ErrNoForge)`.
  - `TestRemote_SlugExcludesTheRoot_issue585`:
    `Remote{Root:[gitlab], Path:[g p]}.Slug() == "g/p"`, and
    `webBase(...) == "https://h/gitlab"`.
  - `TestRouter_ResolvesARootedRemote_issue585`: through `NewTestRouter` with
    `Hosts{"h/gitlab": KindGitLab}`, `Label` gives GitLab.
- [ ] **Step 2:** The tests fail.
- [ ] **Step 3:** Implement. Keep `Resolve` short by splitting it into
  `rootedKeys(host) [][]string`, `longestPrefix` and `sshRoot`.
- [ ] **Step 4:** The tests pass, and so does the whole forge package.
  Unrooted behaviour is unchanged.
- [ ] **Step 5:** Commit `feat(#585): a remote under a rooted key loses the root from its slug and keeps it in its base`.

### Task 10: Rooted reads: GitLab REST only, Gitea and DC, binding by path

**Files:**
- Modify:
  - `internal/infra/forge/router.go` (`pickGitLab`)
  - `internal/infra/forge/tealogin.go` (`namesInstance`)
  - `internal/infra/forge/bitbucket.go` (`dcWebRoot`, if the root needs
    passing)
- Test:
  - `internal/infra/forge/gitlab_rest_test.go`
  - `internal/infra/forge/gitea_rest_test.go`
  - `internal/infra/forge/bitbucketdc_test.go`
  - `internal/infra/forge/tealogin` tests (wherever `namesInstance` is tested
    today)

**Interfaces:**
- `pickGitLab`: when `len(remote.Root) > 0`, skip the CLI branch entirely. Use
  REST when there is a token and the transport is not CLI; otherwise return
  `&dforge.MissingToolError{TokenEnv: "GITLAB_TOKEN"}`.
- `namesInstance(rawURL, remote)`: when `len(remote.Root) > 0`, it also
  requires
  `strings.Trim(u.Path, "/") == strings.Join(remote.Root, "/")`.

- [ ] **Step 1: Write the failing tests.**
  - `TestGitLabREST_UnderARelativeRoot_issue585`:
    - `FakeGitLabAPI` records the request paths, with
      `Hosts{"host/gitlab": gitlab}` and `GITLAB_TOKEN` set;
    - for both `https://host/gitlab/group/proj.git` and
      `git@host:group/proj.git`, the first request path is
      `/gitlab/api/v4/projects/group%2Fproj/merge_requests`, with the query
      ignored;
    - the PR `URL` fields start with `https://host/gitlab/group/proj`.
  - `TestGitLab_RootedSkipsGlab_issue585`: with glab on the fake PATH and
    `GITLAB_TOKEN` set, no glab invocation is recorded.
  - `TestGitLab_RootedWithoutTokenIsMissing_issue585`: with glab on PATH and
    no token, the result is a `MissingToolError` with `TokenEnv == "GITLAB_TOKEN"`
    and `Tool == ""`.
  - `TestGiteaREST_UnderARelativeRoot_issue585`:
    - with `GITEA_INSTANCE_URL=https://host/gitea` and `GITEA_TOKEN`, a
      request to `/gitea/api/v1/repos/o/r/pulls...` carries the token;
    - with `GITEA_INSTANCE_URL=https://host/other`, no request carries it.
  - `TestNamesInstance_ComparesThePathOnlyWhenRooted_issue585`:
    - a rooted remote and the URL `https://h/gitlab` give true;
    - a rooted remote and `https://h` give false;
    - an unrooted remote and `https://h/bitbucket` give true, which is
      today's rule.
  - `TestBitbucketDC_SSHContextPathStillBinds_issue585` (a regression):
    `BITBUCKET_DC_URL=https://h/bitbucket` with an ssh remote and no rooted
    key still reads `https://h/bitbucket/rest/...`.
  - `TestBitbucketDC_UnderARootedKey_issue585`: with key `h/bitbucket` and
    https remote `https://h/bitbucket/scm/KEY/repo.git`, the request path is
    `/bitbucket/rest/api/1.0/projects/KEY/repos/repo/...`, and the context
    path is not doubled.
- [ ] **Step 2:** The tests fail.
- [ ] **Step 3:** Implement. If the DC test shows the root doubled, have
  `dcCoordinates` read `Path` only (the root is already split off) and let
  `webBase` supply the root.
- [ ] **Step 4:** The tests pass, and so does the whole forge package.
- [ ] **Step 5:** Commit `feat(#585): a rooted GitLab, Gitea or Data Center is read at its root, tokens bound by it`.

### Task 11: Say the ambiguity aloud; docs, gate, PR

**Files:**
- Modify: `internal/tui/app/forgestop.go` (`stoppedPhrase`)
- Modify: `README.md` (`[forge.hosts]` section around the example at
  "git.corp.example", and the sentence that says a path in a key is refused)
- Modify: `docs/ROADMAP.md` (the M16 #585 line)
- Test: `internal/tui/app/forgestop_test.go` (or wherever `stoppedPhrase` is
  tested)

- [ ] **Step 1: Write the failing test.**
  `TestStoppedPhrase_AmbiguousHostIsSaid_issue585`:
  `stoppedPhrase(&forge.AmbiguousHostError{Host: "h"})` gives
  `("h serves several forges under different paths; clone over https so omatty can tell which", true)`.
  `retryForge` still does not retry it, because it is `ErrNoForge`.
- [ ] **Step 2:** It fails, returning `("", false)`.
- [ ] **Step 3:** Add the case before the others in `stoppedPhrase`.
- [ ] **Step 4:** The test passes.
- [ ] **Step 5:** Docs: the README `[forge.hosts]` section gets the rooted
  example and a sentence saying a rooted GitLab is read over REST, so it needs
  `GITLAB_TOKEN`. Run the full gate. Run the smoke test with the scratch
  config line `[forge.hosts]` `"git.example.test/gitlab" = "gitlab"`; omatty
  must start, and a person reads the screen. Commit, push, and open PR
  `feat(#585): a forge under a relative URL root` with `Closes #585`. Move the
  card to Review, and ask before merging.

---

## Slice 3: #596 (branch `feat/596-azure-server`, off `develop` after slice 2)

### Task 12: A Server's own bound token

**Files:**
- Create: `internal/infra/forge/azserver.go`
- Modify: `internal/infra/forge/azlogin.go` (`pickAzure` branches to
  `r.pickAzureServer(remote)` for a non-Services host)
- Test: `internal/infra/forge/azserver_test.go`. Extend `FakeAzureAPI` in
  `azure_test.go` with `collection string` and `maxVersion string`. A request
  whose `api-version` is above `maxVersion` gets 400 with the body
  `{"message":"The requested REST API version of 7.1 is out of range for this server."}`.

**Interfaces:**
- Produces:
  ```go
  // serverBinds is whether rawURL (AZURE_DEVOPS_SERVER_URL) names remote's instance:
  // scheme, host and port equal, and its path a segment prefix of webPath.
  func serverBinds(rawURL string, remote Remote, webPath []string) bool
  func (r *Router) pickAzureServer(remote Remote) (backend, error)
  ```
  `webPath` is the base path that `azureCoordinates` returns, split on `/`.
- Order inside `pickAzureServer`:
  1. coordinates;
  2. a non-https base gives `PlainHTTPError{TokenEnv: "AZURE_DEVOPS_SERVER_TOKEN"}`;
  3. borrow `AZURE_DEVOPS_SERVER_TOKEN` and `AZURE_DEVOPS_SERVER_URL`; no
     token, or no binding, gives
     `MissingToolError{TokenEnv: "AZURE_DEVOPS_SERVER_TOKEN for " + base}`;
  4. `auth = noSignIn(basicAuth("", tok))`;
  5. version (Task 13; until then, `"7.1"`);
  6. `azBackend{..., env: "AZURE_DEVOPS_SERVER_TOKEN"}`.

- [ ] **Step 1: Write the failing tests.**
  - `TestAzureServer_ReadsWithItsOwnBoundToken_issue596`:
    - `Hosts{"tfs.corp": KindAzure}`, remote
      `https://tfs.corp/tfs/DefaultCollection/Proj/_git/repo`;
    - `AZURE_DEVOPS_SERVER_URL=https://tfs.corp/tfs` and the token set;
    - `ListPRs` succeeds;
    - the request carries Basic auth with an empty user and the token.
  - `TestServerBinds_PrefixRules_issue596`: a table test against web path
    `[tfs DefaultCollection]`:

    | URL | Binds |
    |---|---|
    | `https://tfs.corp` | yes |
    | `https://tfs.corp/tfs` | yes |
    | `https://tfs.corp/tfs/DefaultCollection` | yes |
    | `https://tfs.corp/tfs/Other` | no |
    | `https://tfs.corp:8443/tfs` | no |
    | `http://tfs.corp/tfs` | no |
    | `https://other/tfs` | no |
    | `https://tfs.corp/tf` | no, because it is not a segment prefix |

  - `TestAzureServer_UnboundURLIsMissingTool_issue596`: with the URL for
    another collection, the result is a `MissingToolError` whose `TokenEnv`
    starts with `"AZURE_DEVOPS_SERVER_TOKEN for https://tfs.corp/tfs/DefaultCollection"`,
    and no request is made.
  - `TestAzureServer_PlainHTTPIsRefused_issue596`.
  - `TestAzureServer_NeverSendsServicesCredentials_issue596`:
    - `AZURE_DEVOPS_EXT_PAT=svc-pat` is set, and fake az is on PATH issuing
      `az-tok`;
    - the Server token is unset;
    - no request reaches the fake, and the error text contains neither
      `svc-pat` nor `az-tok`;
    - with the Server token set as well, no request's `Authorization` header
      decodes to either of them.
  - `TestAzureServices_Unchanged_issue596`: a `dev.azure.com` remote with only
    `AZURE_DEVOPS_SERVER_TOKEN` set still gives today's Services note, so the
    Server token never reaches Services.
- [ ] **Step 2:** The tests fail: today a Server host gives ErrNoForge.
- [ ] **Step 3:** Implement. Delete the refusal in `pickAzure`, but keep its
  comment's reasoning, moved to `pickAzureServer`'s doc comment, and reference
  #456 and #596.
- [ ] **Step 4:** The tests pass, and so does the whole forge package.
- [ ] **Step 5:** Commit `feat(#596): Azure DevOps Server is read with a token bound to it`.

### Task 13: The API version is probed once per instance

**Files:**
- Modify:
  - `internal/infra/forge/azure.go`: `azBackend.version`, used by `api()`;
    the constant becomes `azureServicesVersion = "7.1"`
  - `internal/infra/forge/azure_items.go`: the `-preview.4` site
  - `internal/infra/forge/azure.go`: the `-preview.1` site
  - `internal/infra/forge/azserver.go`: the probe
  - `internal/infra/forge/router.go`: `azVersions map[string]string` under `mu`
  - `internal/domain/forge/errors.go`: `UnsupportedError`
- Test: `internal/infra/forge/azserver_test.go`, `internal/domain/forge/errors_test.go`

**Interfaces:**
- Produces:
  ```go
  // domain/forge
  type UnsupportedError struct{ Host, Reason string }
  func (e *UnsupportedError) Error() string
  // infra/forge
  var azServerVersions = []string{"7.1", "6.0", "5.0"}
  func (r *Router) azServerVersion(base string, a auth) (string, error)
  ```
- `azServerVersion`:
  - it returns the cached version for `base` when there is one;
  - otherwise, for each version, it does a GET of
    `base+"/_apis/projects?$top=1&api-version="+v`, with a context bounded by
    `r.timeout`;
  - 2xx: cache that version and return it;
  - an error matching `errBadRequest`: try the next version;
  - any other error (auth, rate limit, outage): return it unchanged;
  - all three exhausted: `&dforge.UnsupportedError{Host: host, Reason: "speaks no API version omatty reads (5.0 to 7.1)"}`.
- `answerError` needs to tell a 400 apart. Add `var errBadRequest = errors.New("bad request")`
  in `rest.go`, wrapped for a 400 the same way `errNotFound` is.
- `api()` writes `"api-version=" + a.version`. The preview sites append
  `"-preview.N"` to that string.

- [ ] **Step 1: Write the failing tests.**
  - `TestAzureServer_FallsBackTo60On400_issue596`: with `maxVersion "6.0"`,
    `ListPRs` succeeds, and every list request carries `api-version=6.0`.
  - `TestAzureServer_FallsBackTo50_issue596`: the same with `"5.0"`. The
    work-item comments request carries `api-version=5.0-preview.4`.
  - `TestAzureServer_ProbesOncePerInstance_issue596`: across 3 `ListPRs`, the
    projects probe is hit at most 3 times on the first call and 0 times after.
  - `TestAzureServer_UnsupportedIsATypedError_issue596`: with
    `maxVersion "4.1"`, `errors.As` matches `*UnsupportedError`, and the error
    text names the host.
  - `TestAzureServer_AuthDuringProbeIsAuthError_issue596`: a 401 on the probe
    gives `AuthError` with `TokenEnv` set to `AZURE_DEVOPS_SERVER_TOKEN`.
  - `TestAzureServices_StillSends71_issue596`: a Services request carries
    `api-version=7.1`.
- [ ] **Step 2:** The tests fail.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** The tests pass, and so does the whole forge package.
- [ ] **Step 5:** Commit `feat(#596): a Server's API version is found once, 7.1 down to 5.0`.

### Task 14: Say "unsupported" aloud; docs, follow-up issue, gate, PR

**Files:**
- Modify: `internal/tui/app/forgestop.go`. `stopping` includes
  `UnsupportedError`. `stoppedPhrase` returns
  `"Azure DevOps Server at " + Host + " " + Reason, true`.
  `retryForge` retries it, because it is not `ErrNoForge`.
- Modify:
  - `README.md`: the forge table gets a Server row; the token-binding
    paragraph names `AZURE_DEVOPS_SERVER_TOKEN` and `AZURE_DEVOPS_SERVER_URL`
    and what the URL may name; the "Server ... not read yet (#596)" line is
    replaced. Mark it "built against a fake; not yet run against a real
    Server".
  - `docs/ROADMAP.md`: the #596 lines.
- Test: the `forgestop` tests

- [ ] **Step 1: Write the failing test.**
  `TestStoppedPhrase_UnsupportedServerIsSaid_issue596`: the phrase contains
  `"Azure DevOps Server at tfs.corp speaks no API version"`.
  `stopping(err)` is true. After `retryStoppedForges`, `m.forgeRetry[p]` is
  true.
- [ ] **Step 2:** It fails.
- [ ] **Step 3:** Implement.
- [ ] **Step 4:** It passes.
- [ ] **Step 5:**
  - Edit the docs.
  - Open the follow-up issue: `test(M16): run Azure DevOps Server reading against a real Server`,
    labelled `test`, `M16` and `area:forge`, on board 13 in Backlog. Its body
    is what a real run must check: 2019 (5.0), 2020 (6.0), 2022 (7.x), a
    collection URL, and a server URL.
  - Run the full gate and the smoke test.
  - Push, then open PR `feat(#596): Azure DevOps Server, with a token of its own`
    with `Closes #596`, and move the card to Review. Ask before merging.

---

## Close-out (after all three merge)

- [ ] Cards #594, #585 and #596 are Done. CI is green on `develop`.
- [ ] Ask the operator about the M16 release (AGENTS.md: a milestone close
  produces a release, and the tag needs explicit approval).
