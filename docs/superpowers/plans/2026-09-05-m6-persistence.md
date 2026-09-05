# M6 Persistence + Session Adoption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task (the user runs plans inline, not via subagents — see the ai-memory note "Execute inline, not subagents"). Steps use checkbox (`- [ ]`) syntax for tracking.

## Context

M5 is empty: the file tree (#24) shipped in M3. M6 is the last feature milestone before M7's open-source polish. Two things it delivers:

1. **Persistence (#43).** Today quitting omatty closes every PTY, and the OS sends SIGHUP to each `claude`, killing an in-flight turn. The conversation survives on disk (`--resume`), but the running turn does not. M6 runs each `claude` under **dtach** so quitting detaches instead of killing; relaunch reattaches to the live process and the turn finishes on screen. `--resume` (#36) stays the fallback when the socket is gone (reboot).

2. **Session adoption (new issue).** M4's discovery registers *projects* from Claude's transcript store but adopts no sessions. An operator with sessions started outside omatty (or lost from `state.json`) cannot bring them in. Adoption reads the transcript store for a registered project and registers chosen sessions, each started with `--resume`. The roadmap explicitly places adoption in M6, not M4.

**Design decisions locked in brainstorming:**
- Detach layer is **dtach via Homebrew**. It is not installed on this machine; when absent, omatty logs a warning and behaves exactly as today (no persistence, no hard dependency).
- Quit **always detaches**. Killing a claude is only `ctrl+o x` (archive).
- Adoption UI is a new picker on **`ctrl+o A`** (shift-A; `ctrl+o a` is already discovery) plus an `omatty adopt <project>` CLI twin.

**Spec:** this document is self-contained. A design spec is written as the first task to `docs/superpowers/specs/2026-09-05-m6-persistence-design.md` and committed, per the repository's superpowers convention (`AGENTS.md` documentation map).

## Global Constraints

Copied from `AGENTS.md`; every task's requirements include these.

- **Functions 4-20 lines** (`funlen`: 20 lines, 15 statements). `gocyclo` 10. `gocognit` 15. `dupl` threshold 100. Files under 500 lines. Max 2 levels of indentation inside a function.
- **Names specific and unique** (fewer than 5 grep hits). Banned: `data`, `handler`, `manager`, `util`, `helper`, `process`, `info`, `obj`.
- **Explicit types.** No `any` / `interface{}` / `map[string]any` crossing a package boundary. Parse untyped input into a struct at the edge, once.
- **Error messages carry the offending value and expected shape**, e.g. `fmt.Errorf("session %s: socket path %q exceeds the %d-byte limit", id, path, max)`. Never a bare `errors.New`.
- **Keep existing comments.** Write WHY not WHAT. Doc comment on every exported identifier with one usage example. Reference the issue number where a line exists because of a bug or constraint.
- **Dependency injection** through constructor or parameter. No package-level mutable state, no `init()` side effects, no global singletons.
- **Wrap third-party/OS behind a thin interface omatty owns.** dtach is a new external binary and MUST be reachable only through the new `internal/detach` package, the same rule that keeps git inside `internal/vcs` and bubbleterm inside `internal/termwrap` (invariant 4).
- **`stdout` belongs to the TUI** (invariant 5). `fmt.Print*` banned by `forbidigo`; use `slog` to the file handler. CLI subcommands use the existing `report` writer.
- **Invariants that this milestone touches, argued in commit messages:**
  - Inv. 1 (modal key routing, never heuristic): dtach's `-E -z` keep every key reaching claude; the adoption picker reuses the existing modal layer.
  - Inv. 2 (status from JSONL/hooks, never the screen): a reattached session's status still comes from the tailer, which replays the transcript.
  - Inv. 4 (third-party behind one package): dtach lives only in `internal/detach`.
  - Inv. 9 (`state.json` suffices to relaunch every session): no new persisted field — every dtach path is derived from the session uuid. Adopted sessions persist `{ID, Project, Title, Dir}` like any other.
- **Tests:** failing test first, run it and read the failure, then implement, run again. Name regression tests `TestX_describesTheBehaviour_issueNN`. No `time.Sleep` for synchronization. Filesystem tests use `t.TempDir()`. Named fake types in `fakes_test.go` / same-package `*_test.go`, never inline closures.
- **Coverage gate 90%** over `./internal/...`. Does not move.
- **The gate before any branch is ready** (golangci-lint at `$(go env GOPATH)/bin`):
  ```bash
  gofmt -l .
  go vet ./...
  $(go env GOPATH)/bin/golangci-lint run
  go test ./... -race
  ./scripts/check-coverage.sh 90
  ```
- **Every milestone ends with a real-binary smoke test in a sized PTY** via `testdata/ptyrun` (roadmap rule 2). For M6 that test is the persistence proof, and it is the milestone's definition of done.
- **Commit messages** `type(#issue): message`, ending with:
  ```
  Co-Authored-By: Claude Opus 5 (1M context) <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5
  ```
- **Git workflow:** one issue per PR (`Closes #A, #B` auto-closes only `#A` — the M1/M2 trap). Open the PR to `develop`, move issue + PR to the Review column, and stop. Never merge without approval.
- **Board:** project 13, status field `PVTSSF_lAHOBTZlyM4BiIw3zhhB38g`; option ids Review `585b7724`, Sprint Backlog `6c6ffcfe`, Done `fca54811`. Fetch item ids with gh's own `--jq`, never a shell variable (issue bodies carry control characters).

---

## Execution shape

Three branches, sequential, each its own PR to `develop`.

| PR | Branch | Closes | Introduces (later work consumes) |
|---|---|---|---|
| 0 | `docs/m6-persistence-plan` | — | the spec + this plan |
| 1 | `feat/43-sessions-survive-quit` | #43 | `internal/detach` (`Holder`, `Dtach`, `Plain`), `supervisor.Launcher` holder wiring, `Stop` on archive, missing-dtach status line |
| 2 | `feat/NN-adopt-a-session` | new issue | `discover.ProposeSessions`, `discover.ChooseSessions`, `registry.AdoptSession`, `omatty adopt`, `ctrl+o A` picker |

Before PR 2, open the adoption issue: `feat` type, `M6` + `area:discover` (create the label if absent) + `area:registry` + `area:ui` labels, on the board in Sprint Backlog. Title: "Adopt existing claude sessions into omatty". Body references this plan and the roadmap's "Session adoption belongs here" paragraph.

## File structure

**New package `internal/detach`** — omatty's only route to the dtach binary (invariant 4).
- `detach.go` — the `Holder` interface, `Dtach` and `Plain` implementations, `New(home)` selecting between them by looking dtach up on PATH.
- `paths.go` — per-session socket and pidfile paths derived from the uuid, with the 104-byte socket-path cap enforced.
- `stop.go` — `Dtach.Stop`: read pidfile, SIGTERM, bounded wait, SIGKILL.
- test files alongside.

**Modified `internal/paths/paths.go`** — add `SessionDir(home)` = `~/.omatty/s` (short, for the socket-length budget). detach's paths.go builds on it.

**Modified `internal/supervisor/launch.go`** — `Launcher` gains a `detach.Holder`; `Command` wraps its result through the holder. `Start` unchanged in shape.

**Modified `cmd/omatty/main.go`** — construct the holder in `tuiDeps`; pass a `Persist` capability flag / status string into `ui.RunDeps` so the UI can show the missing-dtach warning. Add the `adopt` subcommand and its `discoverSessions` helper (mirrors `discoverProjects`).

**Modified `internal/discover/`** — new `sessions.go` (`ProposeSessions`, `SessionCandidate`), and `choose.go` grows `ChooseSessions` (or `Choose` is generalised — see Task 8). Reuses `readCwd`, `transcripts`, the byte/line caps.

**Modified `internal/registry/commands.go`** — `AdoptSession(store, project, id, title, dir)`.

**Modified `internal/ui/`** — new `adopt.go` (the `ctrl+o A` flow, `SessionCandidatesMsg`, `openAdoption`, `onSessionsProposed`, `commitAdoption`), a `modalAdopt` kind in `modal.go` (or reuse `modalPicker` with a second commit branch — see Task 10), keymap row in `modalview.go`'s `leaderKeys`, dispatch in `routing.go`'s `modalCommand`.

---

## PR 1 — Persistence (#43)

### Task 1: Design spec

**Files:**
- Create: `docs/superpowers/specs/2026-09-05-m6-persistence-design.md`

- [ ] **Step 1:** Write the spec. Sections: Context (the SIGHUP-on-quit problem), the dtach command line and why each flag, per-session paths and the socket-length cap, the `Plain` fallback, the `Stop` teardown, adoption (candidates, `AdoptSession`, the `ctrl+o A` picker), testing strategy, and the smoke test as definition of done. Copy the dtach command line verbatim from Task 4. Note the invariants touched (1, 2, 4, 9) and why each holds.
- [ ] **Step 2:** Commit.

```bash
git checkout -b docs/m6-persistence-plan
git add docs/superpowers/specs/2026-09-05-m6-persistence-design.md docs/superpowers/plans/2026-09-05-m6-persistence.md
git commit -m "docs(#43): M6 persistence and adoption spec and plan"
```

(Copy this plan file to `docs/superpowers/plans/2026-09-05-m6-persistence.md` as part of this commit.)

### Task 2: Session directory path

**Files:**
- Modify: `internal/paths/paths.go`
- Test: `internal/paths/paths_test.go`

**Interfaces:**
- Produces: `paths.SessionDir(home string) string` → `<home>/.omatty/s`

- [ ] **Step 1: Write the failing test**

```go
func TestSessionDir_IsShortToFitTheSocketLengthLimit(t *testing.T) {
	got := paths.SessionDir("/home/u")
	want := "/home/u/.omatty/s"
	if got != want {
		t.Errorf("SessionDir() = %q, want %q", got, want)
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/paths/ -run TestSessionDir -v` — expect FAIL (undefined).
- [ ] **Step 3: Implement**

```go
// SessionDir returns where omatty keeps a dtach socket and pidfile per
// session. The name is one letter deliberately: a unix socket path caps at
// 104 bytes on macOS, so every character in the fixed prefix is budget the
// uuid needs (#43).
func SessionDir(home string) string { return filepath.Join(Root(home), "s") }
```

- [ ] **Step 4:** Run the test — expect PASS.
- [ ] **Step 5:** Commit `feat(#43): a short per-session directory for dtach sockets`.

### Task 3: detach paths with the socket-length cap

**Files:**
- Create: `internal/detach/paths.go`, `internal/detach/paths_test.go`

**Interfaces:**
- Produces:
  - `detach.SocketPath(home, id string) (string, error)` → `<SessionDir>/<id>.sock`, error if the result exceeds `maxSocketPath` (104).
  - `detach.PidPath(home, id string) string` → `<SessionDir>/<id>.pid`
  - `const maxSocketPath = 104`

- [ ] **Step 1: Write the failing tests**

```go
func TestSocketPath_UnderTheLimitReturnsTheSocket(t *testing.T) {
	got, err := detach.SocketPath("/home/u", "abc-123")
	if err != nil {
		t.Fatalf("SocketPath() error = %v, want nil", err)
	}
	if got != "/home/u/.omatty/s/abc-123.sock" {
		t.Errorf("SocketPath() = %q", got)
	}
}

func TestSocketPath_OverTheLimitIsRefusedByName(t *testing.T) {
	long := "/" + strings.Repeat("d", 200)
	_, err := detach.SocketPath(long, "abc-123")
	if err == nil || !strings.Contains(err.Error(), "104") {
		t.Fatalf("SocketPath() error = %v, want it to name the 104-byte limit", err)
	}
}
```

- [ ] **Step 2:** Run — expect FAIL (undefined package).
- [ ] **Step 3: Implement** `paths.go` (doc comments with usage examples; the error names the path and the cap). `PidPath` needs no cap: only the socket is passed to `bind(2)`.
- [ ] **Step 4:** Run — expect PASS.
- [ ] **Step 5:** Commit `feat(#43): derive dtach socket and pid paths, refusing an over-long socket`.

### Task 4: the Holder interface, Plain, and Dtach.Wrap

**Files:**
- Create: `internal/detach/detach.go`, `internal/detach/detach_test.go`

**Interfaces:**
- Produces:
  - `type Holder interface { Wrap(id string, cmd *exec.Cmd) (*exec.Cmd, error); Stop(id string) error; Persists() bool }`
  - `func New(home string) Holder` — returns `*Dtach` when `exec.LookPath("dtach")` succeeds, else `*Plain`.
  - `type Plain struct{}` — `Wrap` returns cmd unchanged; `Stop` is a no-op; `Persists()` false.
  - `type Dtach struct{ home, bin string }` — `Wrap` rebuilds cmd as the dtach line below; `Persists()` true. (`Stop` is Task 5.)

The dtach command line `Dtach.Wrap` produces, for a `claude ... ` cmd:

```
dtach -A <sock> -E -z -r winch \
  sh -c 'echo $$ > "$0"; exec "$@"' <pidfile> \
  claude <flag> <uuid> --settings <hooks>
```

Why each piece:
- `-A <sock>` attach-or-create: one path for first launch and reattach; on reboot the socket is gone and dtach recreates it, re-running the wrapper.
- `-E` disables dtach's detach key, `-z` its suspend key — every keystroke still reaches claude (invariant 1).
- `-r winch` redraws by sending SIGWINCH on attach, which forces Claude Code to repaint into the reattached pane.
- The `sh -c 'echo $$ > "$0"; exec "$@"' <pidfile> claude ...` wrapper records claude's own pid (`exec` replaces the shell, so `$$` is claude), because dtach exposes neither its pid nor its child's. `$0` is the pidfile; `$@` is `claude` onward. Written once, at socket creation.

`Wrap` preserves `cmd.Dir` and `cmd.Env` onto the new `*exec.Cmd`.

- [ ] **Step 1: Write the failing tests**

```go
func TestPlain_WrapReturnsTheCommandUnchanged(t *testing.T) {
	in := exec.Command("claude", "--resume", "abc")
	in.Dir = "/w"
	out, err := (&detach.Plain{}).Wrap("abc", in)
	if err != nil || out != in {
		t.Fatalf("Plain.Wrap() = %v, %v; want the same command and nil", out, err)
	}
}

func TestPlain_DoesNotPersist(t *testing.T) {
	if (&detach.Plain{}).Persists() {
		t.Error("Plain.Persists() = true, want false")
	}
}

func TestDtach_WrapBuildsTheAttachOrCreateLine_issue43(t *testing.T) {
	d := detach.NewDtach("/home/u", "dtach") // test constructor; New() uses LookPath
	in := exec.Command("claude", "--resume", "abc-123", "--settings", "/h.json")
	in.Dir = "/w/parser-fix"

	out, err := d.Wrap("abc-123", in)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Join(out.Args, " ")
	for _, want := range []string{
		"dtach -A", "/home/u/.omatty/s/abc-123.sock", "-E", "-z", "-r winch",
		"/home/u/.omatty/s/abc-123.pid", "claude --resume abc-123 --settings /h.json",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("dtach line %q missing %q", line, want)
		}
	}
	if out.Dir != "/w/parser-fix" {
		t.Errorf("Dir = %q, want it preserved", out.Dir)
	}
}

func TestDtach_KeepsEveryKeyReachingClaude_invariant1(t *testing.T) {
	d := detach.NewDtach("/home/u", "dtach")
	out, _ := d.Wrap("abc", exec.Command("claude"))
	line := strings.Join(out.Args, " ")
	if !strings.Contains(line, "-E") || !strings.Contains(line, "-z") {
		t.Errorf("line %q must disable dtach's detach and suspend keys (invariant 1)", line)
	}
}

func TestDtach_Persists(t *testing.T) {
	if !detach.NewDtach("/h", "dtach").Persists() {
		t.Error("Dtach.Persists() = false, want true")
	}
}
```

- [ ] **Step 2:** Run `go test ./internal/detach/ -v` — expect FAIL (undefined).
- [ ] **Step 3: Implement** `detach.go`. Provide an unexported-field `Dtach` plus an exported `NewDtach(home, bin)` used by tests and by `New`; `New(home)` does `exec.LookPath("dtach")` and returns `&Plain{}` on error (logging a warning via slog), else `NewDtach(home, path)`. `Wrap` calls `SocketPath` (propagating its error), builds the arg slice, and returns `exec.Command(d.bin, args...)` with `Dir`/`Env` copied. Keep `Wrap` under 20 lines by extracting `dtachArgs(sock, pid string, inner []string) []string`.
- [ ] **Step 4:** Run — expect PASS.
- [ ] **Step 5:** Commit `feat(#43): a dtach holder that wraps claude for attach-or-create`.

### Task 5: Dtach.Stop

**Files:**
- Create: `internal/detach/stop.go`, `internal/detach/stop_test.go`

**Interfaces:**
- Consumes: `PidPath` (Task 3).
- Produces: `func (d *Dtach) Stop(id string) error` — reads the pidfile, `SIGTERM`, waits up to `stopGrace` (2s) polling `signal 0`, then `SIGKILL`. A missing pidfile is not an error (nothing to stop). Also removes the pidfile on success.
- `Plain.Stop(id string) error { return nil }`.

- [ ] **Step 1: Write the failing test** (signal a real child the test owns — no `time.Sleep` for synchronization; use a process that blocks, then assert it is gone):

```go
func TestDtachStop_SignalsThePidInTheFile_issue43(t *testing.T) {
	home := t.TempDir()
	proc := exec.Command("sleep", "60")
	if err := proc.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = proc.Process.Kill() })
	writePid(t, home, "abc", proc.Process.Pid) // helper writes SessionDir/abc.pid

	if err := detach.NewDtach(home, "dtach").Stop("abc"); err != nil {
		t.Fatalf("Stop() error = %v, want nil", err)
	}
	if err := proc.Wait(); err == nil {
		t.Error("process still ran after Stop(); want it signalled dead")
	}
}

func TestDtachStop_MissingPidfileIsNotAnError(t *testing.T) {
	if err := detach.NewDtach(t.TempDir(), "dtach").Stop("nope"); err != nil {
		t.Errorf("Stop() with no pidfile = %v, want nil", err)
	}
}
```

- [ ] **Step 2:** Run — expect FAIL.
- [ ] **Step 3: Implement** `stop.go`. Parse the pid, `os.FindProcess`, `Signal(SIGTERM)`; poll `Signal(syscall.Signal(0))` on a short ticker up to `stopGrace`; on timeout `Signal(SIGKILL)`. Errors name the id and pid. Keep each function ≤20 lines (split parse / signal / wait).
- [ ] **Step 4:** Run — expect PASS.
- [ ] **Step 5:** Commit `feat(#43): stop a detached claude by pidfile, SIGTERM then SIGKILL`.

### Task 6: Launcher wraps through the holder

**Files:**
- Modify: `internal/supervisor/launch.go`
- Test: `internal/supervisor/launch_test.go`

**Interfaces:**
- `NewLauncher(bin, hooksFile, home string, holder detach.Holder) *Launcher` — one new parameter.
- `Command(sessionID, dir string) (*exec.Cmd, error)` — now returns an error (from `holder.Wrap`). Callers in `Start` and tests updated.

- [ ] **Step 1: Write the failing test** — a fake holder records the id and returns a sentinel command; assert `Command` returns it. Also update `TestLauncher_CommandPassesSessionIDAndOwnSettings` to pass a `&detach.Plain{}` holder and unwrap the two-value return (Plain leaves the claude line intact, so its existing assertions hold).

```go
func TestLauncher_CommandWrapsThroughTheHolder_issue43(t *testing.T) {
	h := &fakeHolder{wrapped: exec.Command("dtach", "-A")}
	l := supervisor.NewLauncher("claude", "/h.json", t.TempDir(), h)
	cmd, err := l.Command("abc-123", "/w")
	if err != nil {
		t.Fatal(err)
	}
	if h.gotID != "abc-123" {
		t.Errorf("holder saw id %q, want abc-123", h.gotID)
	}
	if cmd.Args[0] != "dtach" {
		t.Errorf("Command() = %v, want the holder's wrapped command", cmd.Args)
	}
}
```

Add `fakeHolder` to `internal/supervisor/fakes_test.go` (create the file if absent): records `gotID`, returns `wrapped, nil` from `Wrap`, no-op `Stop`, `Persists()` false.

- [ ] **Step 2:** Run — expect FAIL (signature mismatch / undefined).
- [ ] **Step 3: Implement.** Add the `holder` field; `Command` builds the claude cmd as today, then `return l.holder.Wrap(sessionID, cmd)`. `Start` becomes: `cmd, err := l.Command(...)`, propagate, then `f(w, h, cmd)`. Update the doc comment to note the holder (invariant 4).
- [ ] **Step 4:** Run `go test ./internal/supervisor/ -race` — expect PASS.
- [ ] **Step 5:** Commit `feat(#43): launch every claude through the detach holder`.

### Task 7: wire the holder in cmd, surface a missing-dtach warning, call Stop on archive

**Files:**
- Modify: `cmd/omatty/main.go` (construct `detach.New(home)`, pass to `NewLauncher`; add a `Persist bool` / `PersistWarning string` to `ui.RunDeps`).
- Modify: `internal/ui/run.go` (`RunDeps` gains the persist fields; pass into `Deps`).
- Modify: `internal/ui/model.go` (`Deps` gains the fields; store a `persistWarning string`), `internal/ui/archive.go` (`dropSession` calls a new injected `Stop(id)` after closing the terminal), `internal/ui/render.go` (show the warning in the status/error line when set).
- Test: `internal/ui/archive_test.go`, `cmd/omatty/main_test.go`, a new `internal/ui/persist_test.go`.

**Interfaces:**
- `ui.RunDeps` / `ui.Deps` gain `Stop func(sessionID string) error` and `PersistWarning string`.
- Default `Stop` is a no-op named function `noStop` (like `noTailStop`).

- [ ] **Step 1: Write the failing tests**
  - `TestModel_archiveStopsTheDetachedProcess_issue43`: a `recordStop` fake records the id; open archive on `s2`, press `y`, assert the id was stopped. (Model test; extends the archive fakes.)
  - `TestModel_showsAWarningWhenPersistenceIsOff_issue43`: `NewModel` with `PersistWarning: "dtach not found: ..."`; assert `m.View().Content` contains "dtach not found".
  - `cmd`: `TestPersistWarning_NamesTheBrewInstall_issue43` — a helper `persistWarning(false)` returns a string containing `brew install dtach`; `persistWarning(true)` returns `""`.
- [ ] **Step 2:** Run — expect FAIL.
- [ ] **Step 3: Implement.**
  - In `main.go`: `holder := detach.New(home)`; `supervisor.NewLauncher("claude", hooksFile, home, holder)`; set `PersistWarning: persistWarning(holder.Persists())` and `Stop: holder.Stop` on `RunDeps`. Add `persistWarning(persists bool) string`.
  - `dropSession`: after `term.Close()`, call `m.stop(sess.ID)` and log a warning on error (the process outliving its row is the leak this prevents). Add `stop` field + `noStop` default in `withDefaults`/`withLifecycleDefaults`.
  - `render.go`: when `m.persistWarning != ""` and no `lastErr`, render it in the same muted line the error uses. Keep it low-key — it is a once-per-session notice, not an error.
- [ ] **Step 4:** Run the full gate — expect PASS and coverage ≥90%.
- [ ] **Step 5:** Commit `feat(#43): wire the holder, warn when dtach is missing, stop on archive`.

### Task 8: PR 1 gate + smoke test + open PR

- [ ] **Step 1:** Run the full gate (all five commands). Fix anything.
- [ ] **Step 2:** `brew install dtach`. Run the persistence smoke test by hand and read the frame:
  ```bash
  go run ./testdata/ptyrun omatty            # start; note a session mid-turn
  # then in a second run, quit with ctrl+o q while a turn is in flight,
  # relaunch, and confirm the turn finished — reattach repaint is the risk.
  ```
  Because `ptyrun` is scripted, drive it with `PTY_KEYS`/`PTY_KEYS2` to send a prompt, `\x0fq` to quit, then a second `ptyrun` invocation to reattach; read both frames with `./testdata/screen`. Record what you saw in the PR body.
- [ ] **Step 3:** Push, open PR `feat(#43): sessions survive quitting omatty` to `develop`, body linking #43, stating the dtach command line and the smoke-test result. Move #43 and the PR to Review. Stop.

---

## PR 2 — Session adoption (new issue)

### Task 9: discover.ProposeSessions

**Files:**
- Create: `internal/discover/sessions.go`, `internal/discover/sessions_test.go`

**Interfaces:**
- Consumes: existing `readCwd`, `transcripts`, `resolveRoot`, caps `maxHeadLines`/`maxHeadBytes`.
- Produces:
  - `type SessionCandidate struct { ID, Title, Dir string; LastUsed time.Time }`
  - `func ProposeSessions(storeRoot string, git Git, projectRoot string, known []string) ([]SessionCandidate, error)` — every transcript whose recorded cwd resolves (via `git.MainCheckout`) to `projectRoot`, whose uuid (the filename without `.jsonl`) is not in `known`, newest first. Title is the first typed user prompt, trimmed to one line and 60 runes; empty when none (falls back to the uuid's first 8 chars at display).
  - `func firstPrompt(path string) string` — bounded read (same caps), returns the first `type:"user"` record's text content, newlines stripped. Untrusted: it becomes a display title only.

- [ ] **Step 1: Write the failing tests** (reuse the `store`/`writeTranscript`/`FakeGit` helpers already in `discover_test.go` — extend `writeTranscript` or add `writeTranscriptWithPrompt` to control the prompt text and the uuid filename):
  - `TestProposeSessions_ListsSessionsForTheProject`: two transcripts under one cwd resolving to the project root → two candidates, newest first.
  - `TestProposeSessions_SkipsSessionsInAnotherProject`: a transcript whose cwd resolves elsewhere is excluded.
  - `TestProposeSessions_SkipsAlreadyRegisteredIDs_issueNN`: an id in `known` is excluded.
  - `TestProposeSessions_TitlesFromTheFirstPrompt`: the candidate title is the first user prompt, truncated.
  - `TestProposeSessions_TreatsTranscriptTextAsUntrusted`: a prompt with a newline / control chars is flattened, not executed or passed through raw.
- [ ] **Step 2:** Run — expect FAIL.
- [ ] **Step 3: Implement.** `candidateSessions(slugDir, git, projectRoot, known)` walks `transcripts(slugDir)`; for each, resolve the slug's cwd once (the whole slug dir shares a cwd) and skip the dir if it does not resolve to `projectRoot`; then per transcript, derive the id from the filename and read `firstPrompt`. Keep functions ≤20 lines and ≤2 nesting levels (early-continue). Reuse `resolveRoot`.
- [ ] **Step 4:** Run — expect PASS.
- [ ] **Step 5:** Commit `feat(#NN): propose adoptable sessions from a project's transcripts`.

### Task 10: registry.AdoptSession

**Files:**
- Modify: `internal/registry/commands.go`
- Test: `internal/registry/adopt_test.go` (new)

**Interfaces:**
- Produces: `func AdoptSession(s *Store, id, project, title, dir string) (Session, error)` — registers `Session{ID: id, Project: project, Title: title, Dir: dir, Worktree: false}`. Refuses a duplicate id (names it) and a blank title (reuse the `RenameSession` trim guard). `Worktree` is false because omatty did not create `dir`, so archive may never delete it (the invariant behind #40's main-checkout rule).

- [ ] **Step 1: Write the failing tests**
  - `TestAdoptSession_RegistersWithWorktreeFalse_issueNN`: adopt, then load and assert the session is present, `Worktree == false`, `Dir` as given.
  - `TestAdoptSession_RefusesADuplicateID`: adopting an existing id errors and names the id; state unchanged.
  - `TestAdoptSession_RefusesABlankTitle`: a whitespace title errors.
- [ ] **Step 2:** Run — expect FAIL.
- [ ] **Step 3: Implement.** Load, guard the trimmed title, `indexOfSession` to detect a duplicate (a *found* index is the error here), append, save. Doc comment with usage example and the `Worktree:false` rationale referencing #40.
- [ ] **Step 4:** Run — expect PASS.
- [ ] **Step 5:** Commit `feat(#NN): adopt a session into the registry without owning its directory`.

### Task 11: discover.ChooseSessions + `omatty adopt`

**Files:**
- Modify: `internal/discover/choose.go` (add `ChooseSessions(cands []SessionCandidate, selection string) ([]SessionCandidate, error)` and `ListSessions(cands, now) []string`), `internal/discover/choose_test.go`.
- Modify: `cmd/omatty/main.go` (add `adopt` to `dispatch`; `adoptSessions(store, home, in)` mirroring `discoverProjects`).
- Test: `cmd/omatty/main_test.go`.

**Interfaces:**
- `discover.ChooseSessions` / `ListSessions` mirror `Choose` / `List` but over `SessionCandidate`. To avoid a `dupl` hit (threshold 100), extract the shared numeric-selection parser: `parseSelection(selection string, n int) ([]int, error)` returning 0-based indices (or a distinguished "all"), and have both `Choose` and `ChooseSessions` map indices to their own slice. Refactor the existing `Choose` onto it in the same commit so the two are one algorithm.
- `cmd`: `adoptSessions` loads the project's root from the store, calls `ProposeSessions`, prints numbered `ListSessions`, reads a line, `ChooseSessions`, then `registry.AdoptSession` per pick, reporting each. Requires the project name as `args[0]`.

- [ ] **Step 1: Write the failing tests**
  - `discover`: `TestChooseSessions_Selection` (table like `TestChoose_Selection`), `TestParseSelection_RejectsANonNumber`, `TestParseSelection_RejectsOutOfRange` — and keep the existing `Choose` tests green after the refactor.
  - `cmd`: `TestAdoptSessions_RegistersThePickedSessions_issueNN` — a temp store with one project, a fixture transcript store, stdin "1", assert the session lands in state.json. (Follow the `main_test.go` pattern: `storeIn`, a `FakeGit` with `Roots`.)
- [ ] **Step 2:** Run — expect FAIL.
- [ ] **Step 3: Implement** the parser extraction, `ChooseSessions`, `ListSessions`, and the `adopt` subcommand + helper. `dispatch` gains `case "adopt": return adoptSessions(store, home, os.Stdin)`. Update `main.go`'s usage doc comment.
- [ ] **Step 4:** Run — expect PASS.
- [ ] **Step 5:** Commit `feat(#NN): omatty adopt registers chosen sessions from a project's transcripts`.

### Task 12: the `ctrl+o A` adoption picker

**Files:**
- Create: `internal/ui/adopt.go`, `internal/ui/adopt_test.go`
- Modify: `internal/ui/modal.go` (add `modalAdopt` kind), `internal/ui/switcher.go` (`commitList` branch), `internal/ui/modalview.go` (`onModalKey`/`modalFooter` cases, `leaderKeys` row), `internal/ui/routing.go` (`modalCommand` case), `internal/ui/model.go` + `run.go` (inject `AdoptPropose`/`AdoptCommit` funcs), `cmd/omatty/main.go` (adapt them like `projectProposer`/`projectRegistrar`).

**Interfaces:**
- `ui.AdoptFunc = func(projectRoot string) ([]ui.SessionProposal, error)` and `ui.AdoptCommitFunc = func(picks []ui.SessionProposal) []error` (registers each via `registry.AdoptSession` and returns per-pick errors), plus a `ui.SessionProposal{ID, Title, Dir string; LastUsed time.Time}`. Defaults `noAdopt`/`noAdoptCommit` name the missing wiring (pattern of `noDiscover`).
- `SessionCandidatesMsg{Token int; Proposals []SessionProposal; Err error}` — background scan result, token-guarded exactly like `ProjectsProposedMsg`.
- `modalAdopt` is a distinct kind so `commitList`/`commit*` picks the adopt commit; it reuses `pickList` (multi-mark) and every list key, the same way `modalPicker` does. A commit starts each adopted session's terminal (`m.start`) and tailer (`m.tailStart`) and rebuilds the sidebar — reuse the body of `addSession` by extracting `foldInSession(sess)` shared between `addSession` and adoption, so the start/tailer/sidebar/follow sequence lives once.

- [ ] **Step 1: Write the failing tests** (extend the `discovery_test.go` harness — a `recordAdopt` fake):
  - `TestModel_adoptionPickerListsTheProjectsSessions_issueNN`: `ctrl+o A`, deliver the scan, assert the view lists the session titles.
  - `TestModel_adoptionRegistersAndStartsMarkedSessions_issueNN`: mark one, enter, assert the commit fake saw it and a terminal/tailer were started (the fake `Start`/`tailStart` record ids).
  - `TestModel_adoptionScopesToTheSelectedProject_issueNN`: the proposer is called with the cursor's project root.
  - `TestLeaderKeys_documentAdoption_issueNN`: `ui.LeaderKeys()` contains `A` (guards the #103 class — a key in the router and in no keymap).
- [ ] **Step 2:** Run — expect FAIL.
- [ ] **Step 3: Implement** `adopt.go` (`openAdoption`, `onSessionsProposed`, `commitAdoption`) mirroring `discovery.go`; the `modalCommand` case:
  ```go
  case "shift+A", "A":
      return m.openAdoption()
  ```
  (two spellings, like `R`/`shift+R`), a `leaderKeys` row `{"A", "adopt a session claude already knows"}`, the `modalAdopt` arms in `onModalKey`/`modalFooter`/`commitList`, and the `onDataMsg` case for `SessionCandidatesMsg`. Wire `AdoptPropose`/`AdoptCommit` through `RunDeps`→`Deps`→`Model` and adapt them in `main.go` closing over the store, `discover.ProposeSessions`, and `registry.AdoptSession`. Extract `foldInSession`.
- [ ] **Step 4:** Run the full gate — expect PASS, coverage ≥90%.
- [ ] **Step 5:** Commit `feat(#NN): adopt sessions from inside omatty with ctrl+o A`.

### Task 13: PR 2 gate + smoke test + open PR + roadmap update

- [ ] **Step 1:** Full gate.
- [ ] **Step 2:** Smoke test: with a project registered, run omatty, press `ctrl+o A`, confirm the picker lists real sessions from the transcript store, adopt one, and confirm its pane starts and resumes. Read the frame with `testdata/screen`.
- [ ] **Step 3:** Update `docs/ROADMAP.md`: mark M6 done, note the dtach mechanism shipped and that adoption landed here as planned. Commit `docs(#NN): mark M6 done`.
- [ ] **Step 4:** Push, open PR `feat(#NN): adopt existing claude sessions` to `develop`, linking the issue, stating the smoke-test result. Move the issue and PR to Review. Stop.

---

## Verification (end to end)

The milestone is done when both hold, verified with the real binary in a sized PTY (roadmap rule 2), not only the coverage gate:

1. **Persistence:** with dtach installed, start a session, send a prompt so a turn is in flight, quit omatty with `ctrl+o q`, relaunch, and the turn is still running / finishes on screen. Without dtach, omatty starts, shows the "dtach not found … brew install dtach" line, and otherwise behaves as today.
2. **Adoption:** `ctrl+o A` over a registered project lists the sessions claude has in that directory that omatty does not yet track; adopting one starts its pane and `--resume`s it. `omatty adopt <project>` does the same from the CLI.

Gate for both PRs:
```bash
gofmt -l .
go vet ./...
$(go env GOPATH)/bin/golangci-lint run
go test ./... -race
./scripts/check-coverage.sh 90
```

## Self-review notes

- **Spec coverage:** persistence (Tasks 2-8), the missing-dtach fallback (Tasks 4, 7), Stop-on-archive (Tasks 5, 7), adoption proposal/registry/CLI/UI (Tasks 9-12), smoke tests (8, 13) — every design section maps to a task.
- **`dupl` risk** is called out and pre-empted in Task 11 (shared `parseSelection`) and Task 12 (shared `foldInSession`); both are the kind of copy `dupl` flags at threshold 100.
- **Invariant 9:** no new `state.json` field — every dtach path derives from the uuid, and adopted sessions carry only the existing fields. Stated in commit messages for Tasks 4 and 10.
- **Type consistency:** `Holder{Wrap,Stop,Persists}` is used identically in Tasks 4-7; `SessionCandidate` (discover) vs `SessionProposal` (ui) are deliberately distinct types at the package boundary, adapted in `main.go` exactly as `Candidate`→`Proposal` is today.
