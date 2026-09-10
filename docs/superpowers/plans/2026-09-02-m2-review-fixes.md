# M2 Review Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix the eight critical and the important findings from the 2026-09-02 review of the M2 merge and PR #52, each with a failing regression test first, in six independent branches that open PRs to `develop`.

**Architecture:** Nothing structural changes in branches A–E; each fix is local to the package that owns the bug and is landed under its GitHub issue number. Branch F (structural refactors) is written here but is blocked until A–E are merged, because it moves code the other branches edit.

**Tech Stack:** Go 1.26, `charm.land/bubbletea/v2`, `github.com/creack/pty`, stdlib `testing`.

**Spec:** The review findings, recorded as issues #54–#80 on github.com/WilsonSousajr/omatty. Each task names its issue; read the issue before starting the task.

## Global Constraints

Copied from `AGENTS.md`; every task's requirements include these.

- Functions 4–20 lines. Files under 500 lines. Max 2 levels of indentation.
- Names specific and unique. Banned: `data`, `handler`, `manager`, `util`, `helper`, `process`, `info`, `obj`.
- No `any` / `map[string]any` across a package boundary.
- Error messages carry the offending value and the expected shape. Never a bare `errors.New("invalid input")`.
- Keep existing comments. Write WHY, not WHAT. Doc comment on every exported identifier with one usage example. Reference the issue number where a line exists because of a bug.
- No package-level mutable state, no `init()` side effects. Inject through constructor or parameter.
- `stdout` belongs to the TUI (invariant 5). Diagnostics go to `slog`.
- Tests: write the failing test first, run it and read the failure, fix, run it again. Name regression tests after the bug: `TestX_describesTheBug_issueNN`. Never `time.Sleep` for synchronisation. Filesystem tests use `t.TempDir()`; never touch the real `~/.claude` or `~/.omatty`. Named fake types, not inline closures, when a new fake is introduced.
- Commit messages: `type(#issue): message`. End every commit with:
  ```
  Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
  Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5
  ```
- The gate, before claiming a branch ready (`golangci-lint` is not installed on this machine; CI runs it, so keep functions short and names clean by inspection):
  ```bash
  gofmt -l .                     # must print nothing
  go vet ./...
  go test ./... -race
  ./scripts/check-coverage.sh 90
  ```
- Never merge. Open the PR to `develop`, put the issue and PR in the Review column, and stop.

## Branch layout

Each branch gets its own worktree under the job's temp directory so they run in parallel without touching the main checkout at `/Users/will/Documents/Projects/omatty`, which stays on `fix/terminal-birth-size`.

| Branch | Base | Worktree | Issues |
|---|---|---|---|
| A `fix/hook-invariant-11` | `origin/develop` | `$TMP/branch-a` | #54 #55 #56 #57 #58 |
| B `fix/watcher-transcript` | `origin/develop` | `$TMP/branch-b` | #59 #60 #61 #62 #63 #64 #65 #66 (+#78 partial) |
| C `fix/hook-listener` | `origin/develop` | `$TMP/branch-c` | #67 #68 |
| D `fix/ui-notify-tick` | `origin/develop` | `$TMP/branch-d` | #69 #70 #71 #72 |
| E `fix/terminal-birth-size` | existing PR #52 branch | `$TMP/branch-e` | #73 #74 #75 |
| F `refactor/m2-structure` | `origin/develop` after A–E merge | later | #76 #77 #78 #79 #80 |

`$TMP` is `/Users/will/.claude/jobs/39c471e0/tmp`. Create a worktree with:

```bash
git -C /Users/will/Documents/Projects/omatty fetch origin
git -C /Users/will/Documents/Projects/omatty worktree add $TMP/branch-a -b fix/hook-invariant-11 origin/develop
# branch E instead:
git -C /Users/will/Documents/Projects/omatty worktree add $TMP/branch-e fix/terminal-birth-size
```

Opening the PR at the end of a branch (replace the numbers):

```bash
git push -u origin <branch>
gh pr create --base develop --head <branch> --title "fix(#54): <one line>" --body "$(cat <<'EOF'
Closes #54, #55, #56, #57, #58.

## What changed
<one bullet per issue>

## Why
<one sentence per issue, from the issue body>

## Verified
Failing test first for each issue (names below), then the gate: gofmt, vet, `go test ./... -race`, coverage ≥ 90%.
<test names>

🤖 Generated with [Claude Code](https://claude.com/claude-code)

https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5
EOF
)"
for n in 54 55 56 57 58; do gh issue edit $n --add-label "" >/dev/null; done
```

To move an issue or PR to the Review column: `gh project item-list 13 --owner WilsonSousajr --format json --jq '.items[] | select(.content.number==<N>) | .id'` gives the item id, then `gh project item-edit --project-id PVT_kwHOBTZlyM4BiIw3 --id <item> --field-id PVTSSF_lAHOBTZlyM4BiIw3zhhB38g --single-select-option-id 585b7724`. (Get the project id with `gh project view 13 --owner WilsonSousajr --format json --jq .id` if that one is stale.)

---

## Branch A — `fix/hook-invariant-11`

### Task A1: Dispatch `omatty hook` before anything that can fail (#54)

**Files:**
- Modify: `cmd/omatty/main.go:40-81`
- Test: `internal/watcher/e2e/hook_socket_test.go`

**Interfaces:**
- Consumes: `hooks.Report(stdin io.Reader, socketPath string, dialTimeout time.Duration) error`, `paths.HookSocket(home string) string`.
- Produces: `runHook()` in `cmd/omatty/main.go`; `buildOmatty(t)` in the e2e package now returns a binary built once in `TestMain`.

- [ ] **Step 1: Hoist the binary build into `TestMain` (part of #80, needed here so the new tests do not add two more `go build`s)**

Replace `buildOmatty` at the bottom of `internal/watcher/e2e/hook_socket_test.go` with:

```go
// omattyBin is the binary under test, built once for the package (issue #80:
// building it per test cost a second each).
var omattyBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "om-bin")
	if err != nil {
		panic(err)
	}
	omattyBin = filepath.Join(dir, "omatty")
	build := exec.Command("go", "build", "-o", omattyBin, "../../../cmd/omatty")
	if out, err := build.CombinedOutput(); err != nil {
		panic(fmt.Sprintf("building omatty: %v\n%s", err, out))
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func buildOmatty(t *testing.T) string {
	t.Helper()
	return omattyBin
}
```

Add `"fmt"` to the imports.

- [ ] **Step 2: Write the failing tests**

Append to `internal/watcher/e2e/hook_socket_test.go`:

```go
// Regression, issue #54: the hook subcommand ran after the log file was
// opened, so an unwritable ~/.omatty/logs made every hook on the machine
// exit 1 with two lines on stderr (invariant 11).
func TestOmattyHook_ExitsZeroWhenTheLogDirIsUnwritable_issue54(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir, err := os.MkdirTemp("", "om")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	locked := filepath.Join(dir, ".omatty")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(locked, 0o700) }()

	out, err := runHook(t, []string{"HOME=" + dir}, `{"session_id":"x","hook_event_name":"Stop"}`)

	if err != nil || len(out) != 0 {
		t.Errorf("omatty hook exited %v with output %q, want exit 0 and no output (invariant 11)", err, out)
	}
}

// Same bug, second trigger: os.UserHomeDir fails without HOME and the error
// reached main's stderr path.
func TestOmattyHook_ExitsZeroWithoutHOME_issue54(t *testing.T) {
	out, err := runHook(t, nil, `{"session_id":"x","hook_event_name":"Stop"}`)

	if err != nil || len(out) != 0 {
		t.Errorf("omatty hook without HOME exited %v with output %q, want exit 0 and no output (invariant 11)", err, out)
	}
}

// runHook runs the built binary's hook subcommand with the test's own
// environment minus HOME, plus env.
func runHook(t *testing.T, env []string, stdin string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(buildOmatty(t), "hook")
	cmd.Env = append(withoutHome(os.Environ()), env...)
	cmd.Stdin = strings.NewReader(stdin)
	return cmd.CombinedOutput()
}

func withoutHome(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, "HOME=") {
			out = append(out, kv)
		}
	}
	return out
}
```

- [ ] **Step 3: Run them and read the failure**

Run: `go test ./internal/watcher/e2e/ -run issue54 -v`
Expected: both FAIL with `exited exit status 1 with output "…ERROR omatty exited…\nomatty: …"`.

- [ ] **Step 4: Fix `main`**

In `cmd/omatty/main.go`, replace `main` and remove the `case "hook":` arm from `dispatch`:

```go
func main() {
	// Invariant 11: the hook runs before anything that can fail or print. A
	// missing HOME or an unwritable log directory must not reach claude as a
	// non-zero exit or a byte of output (issue #54).
	if len(os.Args) > 1 && os.Args[1] == "hook" {
		runHook()
		return
	}
	if err := run(); err != nil {
		slog.Error("omatty exited", "err", err)
		// Subcommands report to the operator; the TUI owns stdout only while
		// it is running, and by here it has stopped.
		_, _ = fmt.Fprintln(os.Stderr, "omatty:", err)
		os.Exit(1)
	}
}

// runHook is the whole of `omatty hook`. Every error and panic is swallowed
// here rather than logged: the log file is the one thing this path must not
// depend on.
func runHook() {
	defer func() { _ = recover() }()
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	_ = hooks.Report(os.Stdin, paths.HookSocket(home), time.Second)
}
```

In `dispatch`, delete the three-line `case "hook":` block and its comment. Add `omatty hook` to the package doc's usage block: `//	omatty hook                       forward a claude hook event (internal)`.

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/watcher/e2e/ -race -v`
Expected: all PASS, including the two existing hook tests.

- [ ] **Step 6: Commit**

```bash
git add cmd/omatty/main.go internal/watcher/e2e/hook_socket_test.go
git commit -m "fix(#54): dispatch omatty hook before anything that can fail

Invariant 11: the hook must exit 0 and print nothing in every case. It ran
after openLog, so an unwritable ~/.omatty/logs or an unset HOME made every
hook on the machine exit 1 with stderr output. The e2e build now happens once
per package (#80).

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task A2: Scan the routable fields instead of truncating the payload (#55, #57)

**Files:**
- Modify: `internal/hooks/report.go`
- Test: `internal/hooks/report_test.go`

**Interfaces:**
- Produces: `hooks.ParsePayload(stdin io.Reader) (Payload, bool)` (exported; was `parsePayload`).

- [ ] **Step 1: Write the failing tests**

Append to `internal/hooks/report_test.go`:

```go
// Regression, issue #55: the payload was read through a 64 KiB limit and then
// unmarshalled whole, so a PostToolUse carrying a big tool_response was cut
// mid-object, failed to parse, and was silently dropped.
func TestParsePayload_SkipsAHugeToolResponse_issue55(t *testing.T) {
	in := `{"session_id":"abc","hook_event_name":"PostToolUse","tool_name":"Read",` +
		`"tool_input":{"file_path":"/f"},"tool_response":"` + strings.Repeat("x", 200<<10) + `"}`

	p, ok := hooks.ParsePayload(strings.NewReader(in))

	if !ok || p.SessionID != "abc" || p.HookEventName != "PostToolUse" || p.ToolName != "Read" {
		t.Errorf("ParsePayload = (%+v, %v), want the routable fields of a 200 KiB PostToolUse", p, ok)
	}
}

// The cap still exists for a runaway producer; the routable fields come first
// in claude's payloads, so they survive the cut.
func TestParsePayload_KeepsTheRoutableFieldsPastTheCap_issue55(t *testing.T) {
	in := `{"session_id":"abc","hook_event_name":"PostToolUse","tool_response":"` +
		strings.Repeat("x", 5<<20) + `"}`

	p, ok := hooks.ParsePayload(strings.NewReader(in))

	if !ok || p.SessionID != "abc" || p.HookEventName != "PostToolUse" {
		t.Errorf("ParsePayload = (%+v, %v), want session abc PostToolUse from before the cap", p, ok)
	}
}

// Replaces the socket half of TestReport_OversizedStdinIsBounded_issue18,
// whose empty timeout branch passed when nothing arrived at all.
func TestParsePayload_RejectsAnOversizedSessionID_issue18(t *testing.T) {
	in := `{"session_id":"` + strings.Repeat("A", 2<<20) + `","hook_event_name":"Stop"}`

	if p, ok := hooks.ParsePayload(strings.NewReader(in)); ok {
		t.Errorf("a 2 MiB session id was accepted as %q…, want it dropped", p.SessionID[:16])
	}
}

func TestReport_ForwardsAPostToolUseWithAHugeResponse_issue55(t *testing.T) {
	path, got := listen(t)
	in := `{"session_id":"abc","hook_event_name":"PostToolUse","tool_response":"` +
		strings.Repeat("x", 200<<10) + `"}`

	if err := hooks.Report(strings.NewReader(in), path, time.Second); err != nil {
		t.Fatal(err)
	}

	select {
	case line := <-got:
		var p hooks.Payload
		if err := json.Unmarshal([]byte(line), &p); err != nil || p.SessionID != "abc" || p.HookEventName != "PostToolUse" {
			t.Errorf("forwarded %q (err %v), want session abc PostToolUse", line, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the big PostToolUse never reached the socket")
	}
}
```

In `TestReport_OversizedStdinIsBounded_issue18`, delete the second `select` (lines 105–112, the one with the empty `time.After` branch) and its comment. Keep the first half, which asserts `Report` returns promptly.

- [ ] **Step 2: Run them and read the failure**

Run: `go test ./internal/hooks/ -run 'issue55|issue18' -v`
Expected: the three `ParsePayload` tests fail to compile (`undefined: hooks.ParsePayload`). Temporarily add `func ParsePayload(r io.Reader) (Payload, bool) { return parsePayload(r) }` to `report.go`, rerun: `SkipsAHugeToolResponse` and `KeepsTheRoutableFieldsPastTheCap` and `ForwardsAPostToolUse…` FAIL (`ok` false / nothing reached the socket); `RejectsAnOversizedSessionID` passes by accident (the truncated JSON fails to parse). That is the bug: total loss looks like bounding.

- [ ] **Step 3: Replace the parser**

Replace everything from `maxPayload` to the end of `internal/hooks/report.go` with:

```go
// maxPayload bounds what a hook reads. A PostToolUse carries the whole
// tool_response, which is routinely over 64 KiB and was dropped at that cap
// (issue #55): the routable fields are now scanned out and every other value
// is skipped token by token, so the cap guards only a runaway producer
// (invariant 11).
const maxPayload = 4 << 20

// maxField bounds any routable string. A session id or event name longer than
// this is not one claude wrote.
const maxField = 1024

// Payload is the slice of a hook's stdin that status needs.
type Payload struct {
	SessionID        string `json:"session_id"`
	HookEventName    string `json:"hook_event_name"`
	NotificationType string `json:"notification_type,omitempty"`
	ToolName         string `json:"tool_name,omitempty"`
}

// Report reads a hook payload from stdin and forwards it to omatty's socket as
// one JSON line. It is the whole of `omatty hook`.
//
// Invariant 11: a hook must never block or fail claude. Every failure — no
// socket (omatty closed), refused connection, malformed input — returns nil so
// the command exits 0. The error return exists only so tests can assert the
// forwarding path; cmd discards it.
func Report(stdin io.Reader, socketPath string, dialTimeout time.Duration) error {
	p, ok := ParsePayload(stdin)
	if !ok {
		return nil
	}
	conn, err := net.DialTimeout("unix", socketPath, dialTimeout)
	if err != nil {
		return nil // omatty is not listening; that is fine
	}
	defer func() { _ = conn.Close() }()
	// A peer that accepts and never reads must not hold the hook past
	// claude's own timeout (issue #57).
	_ = conn.SetWriteDeadline(time.Now().Add(dialTimeout))
	if line, err := json.Marshal(p); err == nil {
		_, _ = fmt.Fprintf(conn, "%s\n", line)
	}
	return nil
}

// ParsePayload scans the routable fields out of a hook's stdin. Values it
// does not need - tool_input, tool_response - pass through the decoder
// without being held, so their size never matters. ok is false for
// unreadable, malformed, or session-less input, all dropped silently
// (invariant 11).
//
//	p, ok := hooks.ParsePayload(os.Stdin)
func ParsePayload(stdin io.Reader) (Payload, bool) {
	dec := json.NewDecoder(io.LimitReader(stdin, maxPayload))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return Payload{}, false
	}
	var p Payload
	if !scanFields(dec, &p) {
		return Payload{}, false
	}
	return p, p.SessionID != "" && p.HookEventName != ""
}

// scanFields walks the top-level object. It stops quietly where the cap cut
// the input - the routable fields come first in claude's payloads - and
// reports false only for a routable field that is not a sane string.
func scanFields(dec *json.Decoder, p *Payload) bool {
	for dec.More() {
		key, err := dec.Token()
		if err != nil {
			return true
		}
		name, _ := key.(string)
		if dst := routableField(name, p); dst != nil {
			if !readString(dec, dst) {
				return false
			}
			continue
		}
		if !skipValue(dec) {
			return true
		}
	}
	return true
}

func routableField(name string, p *Payload) *string {
	switch name {
	case "session_id":
		return &p.SessionID
	case "hook_event_name":
		return &p.HookEventName
	case "notification_type":
		return &p.NotificationType
	case "tool_name":
		return &p.ToolName
	}
	return nil
}

// readString decodes one routable value, refusing anything that is not a
// string of sane length: a 2 MiB session id is a runaway producer, not a
// session (issue #18).
func readString(dec *json.Decoder, dst *string) bool {
	tok, err := dec.Token()
	s, ok := tok.(string)
	if err != nil || !ok || len(s) > maxField {
		return false
	}
	*dst = s
	return true
}

// skipValue consumes one value of any size token by token, so a large
// tool_response flows through the decoder's buffer without being kept.
func skipValue(dec *json.Decoder) bool {
	depth := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		switch tok {
		case json.Delim('{'), json.Delim('['):
			depth++
		case json.Delim('}'), json.Delim(']'):
			depth--
		}
		if depth == 0 {
			return true
		}
	}
}
```

Remove the temporary `ParsePayload` shim from Step 2.

- [ ] **Step 4: Run the tests**

Run: `go test ./internal/hooks/ -race -v`
Expected: all PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/report.go internal/hooks/report_test.go
git commit -m "fix(#55): scan the routable hook fields instead of truncating the payload

A PostToolUse with a tool_response over 64 KiB was cut mid-object and dropped
whole. The parser now walks the top-level object, keeps the four routable
strings (each capped at 1 KiB), and skips everything else token by token. The
write to the socket carries a deadline so a peer that never reads cannot hold
the hook (#57).

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task A3: Quote the binary path in hooks.json (#56)

**Files:**
- Modify: `internal/hooks/settings.go`
- Test: `internal/hooks/settings_test.go`

- [ ] **Step 1: Write the failing test and update the two expectations**

Append to `internal/hooks/settings_test.go`:

```go
// Regression, issue #56: claude runs a command hook through a shell, so an
// unquoted install path with a space split into a command that did not exist
// and every hook failed with 127.
func TestRender_QuotesTheBinaryPathForTheShell_issue56(t *testing.T) {
	out, err := hooks.Render(`/Users/w/My Tools/it's $HOME/omatty`)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Hooks map[string][]struct {
			Hooks []struct {
				Command string `json:"command"`
			} `json:"hooks"`
		} `json:"hooks"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}

	got := parsed.Hooks["Stop"][0].Hooks[0].Command
	want := `'/Users/w/My Tools/it'\''s $HOME/omatty' hook`
	if got != want {
		t.Errorf("command = %q, want %q", got, want)
	}
}
```

In `checkHook`, change the expected command to `'/Users/w/go/bin/omatty' hook` (both in the comparison and the message). In `TestRender_UsesTheAbsoluteBinaryPath_issue17`, change the contained string to `"'/opt/homebrew/bin/omatty' hook"`.

- [ ] **Step 2: Run and read the failure**

Run: `go test ./internal/hooks/ -run Render -v`
Expected: three FAIL (`command = "/Users/w/My Tools/it's $HOME/omatty hook"`, and the two updated expectations).

- [ ] **Step 3: Implement**

In `internal/hooks/settings.go`, add `"strings"` to the imports, rename `handler` to `hookCommand` (banned name), and change `Render`:

```go
type hookCommand struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

type group struct {
	Hooks []hookCommand `json:"hooks"`
}

// Render returns the JSON for ~/.omatty/hooks.json: every status event runs
// `<binPath> hook`. binPath must be absolute — claude runs hooks with
// whatever PATH it inherited, which need not include omatty's directory.
//
//	content, _ := hooks.Render("/Users/w/go/bin/omatty")
func Render(binPath string) ([]byte, error) {
	h := hookCommand{Type: "command", Command: shellQuote(binPath) + " hook", Timeout: 5}
	events := make(map[string][]group, len(statusEvents))
	for _, name := range statusEvents {
		events[name] = []group{{Hooks: []hookCommand{h}}}
	}
	return json.MarshalIndent(settings{Hooks: events}, "", "  ")
}

// shellQuote wraps s in single quotes for a POSIX shell, escaping any single
// quote inside it. claude runs command hooks through a shell, so a path with
// a space or a metacharacter was split or expanded (issue #56).
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
```

- [ ] **Step 4: Run, then grep for other callers expecting the bare form**

Run: `go test ./internal/hooks/ -race -v && grep -rn 'omatty hook"' --include='*.go' .`
Expected: PASS; the grep prints nothing outside `internal/hooks`.

- [ ] **Step 5: Commit**

```bash
git add internal/hooks/settings.go internal/hooks/settings_test.go
git commit -m "fix(#56): shell-quote the omatty path in hooks.json

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task A4: Write hooks.json atomically and refuse a symlink (#58)

**Files:**
- Modify: `internal/supervisor/hooks.go`
- Test: `internal/supervisor/hooks_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/supervisor/hooks_test.go`:

```go
// Regression, issue #58 (invariant 3): a symlink at the hooks path was
// followed, so a link planted at ~/.omatty/hooks.json pointing at the user's
// ~/.claude/settings.json made omatty overwrite that file on its next start.
func TestWriteHooksFile_RefusesASymlinkAndLeavesTheTargetAlone_issue58(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(target, []byte(`{"theirs":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "hooks.json")
	if err := os.Symlink(target, path); err != nil {
		t.Fatal(err)
	}

	err := supervisor.WriteHooksFile(path, []byte(`{"hooks":{}}`))

	if err == nil || !strings.Contains(err.Error(), path) {
		t.Errorf("WriteHooksFile over a symlink = %v, want an error naming %s", err, path)
	}
	got, _ := os.ReadFile(target)
	if string(got) != `{"theirs":true}` {
		t.Errorf("the symlink target was rewritten to %q (invariant 3)", got)
	}
}

// The file is renamed into place, so a claude reading --settings at that
// instant never sees a truncated file (the #31 failure) and no temp file is
// left behind.
func TestWriteHooksFile_LeavesNoTempFileBehind_issue58(t *testing.T) {
	dir := t.TempDir()

	if err := supervisor.WriteHooksFile(filepath.Join(dir, "hooks.json"), []byte("{}")); err != nil {
		t.Fatal(err)
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 || entries[0].Name() != "hooks.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("dir holds %v, want only hooks.json", names)
	}
}
```

- [ ] **Step 2: Run and read the failure**

Run: `go test ./internal/supervisor/ -run issue58 -v`
Expected: `RefusesASymlink…` FAILS with `WriteHooksFile over a symlink = <nil>` and `the symlink target was rewritten to "{\"hooks\":{}}"`. `LeavesNoTempFileBehind` passes already (nothing to prove yet; it guards the new code).

- [ ] **Step 3: Implement**

Replace the body of `internal/supervisor/hooks.go` below the package doc with:

```go
import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteHooksFile writes omatty's settings file, overwriting any existing one.
//
// This reverses #31's "never overwrite": the file names the omatty binary by
// absolute path, which changes with `go install`, so it must be regenerated on
// every start. The file is ~/.omatty/hooks.json, documented as omatty's own —
// invariant 3 is about the user's ~/.claude/settings.json, which is untouched.
//
//	content, _ := hooks.Render(binPath)
//	supervisor.WriteHooksFile(paths.HooksFile(home), content)
func WriteHooksFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("supervisor: creating hooks directory %q: %w", dir, err)
	}
	if err := refuseSpecialFile(path); err != nil {
		return err
	}
	return replaceAtomically(path, content)
}

// refuseSpecialFile rejects a symlink or other non-regular file at path.
// os.WriteFile followed a planted symlink straight into the user's own
// ~/.claude/settings.json (issue #58, invariant 3). A rename would merely
// replace the link, but a file omatty expects to own must not be a link at all.
func refuseSpecialFile(path string) error {
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("supervisor: inspecting hooks file %q: %w", path, err)
	}
	if !fi.Mode().IsRegular() {
		return fmt.Errorf("supervisor: hooks file %q is a %v, want a regular file", path, fi.Mode().Type())
	}
	return nil
}

// replaceAtomically writes beside path and renames into place, so a claude
// reading --settings at the same instant never sees a truncated file (the
// #31 failure). Same pattern as registry's state.json.
func replaceAtomically(path string, content []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".hooks-*.tmp")
	if err != nil {
		return fmt.Errorf("supervisor: creating a temp file beside %q: %w", path, err)
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		return fmt.Errorf("supervisor: writing %q: %w", f.Name(), err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("supervisor: closing %q: %w", f.Name(), err)
	}
	if err := os.Rename(f.Name(), path); err != nil {
		return fmt.Errorf("supervisor: renaming %q to %q: %w", f.Name(), path, err)
	}
	return nil
}
```

- [ ] **Step 4: Run**

Run: `go test ./internal/supervisor/ -race -v`
Expected: all PASS, including `TestWriteHooksFile_UnwritableDirectoryNamesThePath_issue17` (the temp-file error names the path).

- [ ] **Step 5: Commit**

```bash
git add internal/supervisor/hooks.go internal/supervisor/hooks_test.go
git commit -m "fix(#58): write hooks.json atomically and refuse a symlink at its path

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task A5: Gate, push, PR

- [ ] Run the full gate (see Global Constraints). Fix anything it reports.
- [ ] Push and open the PR titled `fix(#54): keep omatty hook silent and unfailing in every case` with `Closes #54, #55, #56, #57, #58.`
- [ ] Move #54–#58 and the PR to the Review column.

---

## Branch B — `fix/watcher-transcript`

### Task B1: Match Claude's transcript slug exactly (#60)

**Files:**
- Modify: `internal/paths/paths.go:34-41`
- Test: `internal/paths/paths_test.go`

- [ ] **Step 1: Add the failing cases**

In `TestTranscriptSlug`, append two rows to the table:

```go
		{
			// Regression, issue #60: claude replaces every character outside
			// [a-zA-Z0-9]; omatty mapped only '/' and '.', so this project's
			// transcript was never found and its status stayed blank forever.
			name: "space and underscore become dashes",
			dir:  "/Users/will/Documents/University/2026.1/LAB SD/my_proj",
			want: "-Users-will-Documents-University-2026-1-LAB-SD-my-proj",
		},
		{
			name: "unicode and punctuation become dashes",
			dir:  "/Users/will/Ação (v2)+notes",
			want: "-Users-will-A----o--v2--notes",
		},
```

(`ç` is two UTF-8 bytes; claude's regex runs on JavaScript code units, where `ç` is one, and `ã` is one. Use `"-Users-will-A---o--v2--notes"` if the Go regexp treats each rune as one match — it does, so the expected value is `-Users-will-A--o--v2--notes`: `ç` one dash, `ã` one dash, space one dash, `(` one, `)` one, `+` one. Compute it by hand before running: `/Users/will/Ação (v2)+notes` → `-` `Users` `-` `will` `-` `A` `-`(ç) `-`(ã) `o` `-`(space) `-`(`(`) `v2` `-`(`)`) `-`(`+`) `notes` = `-Users-will-A--o--v2--notes`. Put that in `want`.)

- [ ] **Step 2: Run and read the failure**

Run: `go test ./internal/paths/ -run TranscriptSlug -v`
Expected: the two new subtests FAIL with the space and underscore preserved.

- [ ] **Step 3: Implement**

Replace `slugReplacer` and `TranscriptSlug` in `internal/paths/paths.go`:

```go
// slugPattern matches every character Claude Code replaces when it names a
// transcript directory: anything outside [a-zA-Z0-9] becomes '-'. Taken from
// the claude binary's own transform. Issue #60 was a path with a space that
// the old '/'-and-'.'-only mapping missed, leaving the tailer blind and every
// restart using --session-id (the #36 failure). Invariant 2 depends on this
// being exact.
var slugPattern = regexp.MustCompile(`[^a-zA-Z0-9]`)

// TranscriptSlug converts an absolute working directory into the directory
// name Claude Code uses under ~/.claude/projects.
//
//	paths.TranscriptSlug("/Users/w/LAB SD") // "-Users-w-LAB-SD"
func TranscriptSlug(dir string) string { return slugPattern.ReplaceAllString(dir, "-") }
```

Replace the `"strings"` import with `"regexp"`.

- [ ] **Step 4: Run**

Run: `go test ./internal/paths/ -race -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/paths/paths.go internal/paths/paths_test.go
git commit -m "fix(#60): replace every non-alphanumeric in the transcript slug, as claude does

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task B2: Realistic fixtures and the three parser fixes (#61, #62, #63)

**Files:**
- Modify: `internal/watcher/transcript.go`
- Modify: `internal/watcher/testdata/transcripts/tool-running.jsonl`
- Create: `internal/watcher/testdata/transcripts/injected-after-done.jsonl`, `list-text-prompt.jsonl`, `stopped-at-max-tokens.jsonl`
- Test: `internal/watcher/transcript_test.go`

**Interfaces:**
- Produces: `Entry.MessageID string`; `rawEntry.IsMeta`, `rawEntry.Message.ID`; `turnEnded(e Entry) bool`; `isInjected(s string) bool`.

- [ ] **Step 1: Write the fixtures**

`internal/watcher/testdata/transcripts/injected-after-done.jsonl`:

```
{"type":"user","timestamp":"2026-09-02T12:00:01.000Z","message":{"role":"user","content":"hi"}}
{"type":"assistant","timestamp":"2026-09-02T12:00:04.000Z","message":{"id":"msg_a","role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":50,"output_tokens":10}}}
{"type":"user","timestamp":"2026-09-02T12:00:10.000Z","isMeta":false,"message":{"role":"user","content":"<local-command-stdout>Set model to opus</local-command-stdout>"}}
{"type":"user","timestamp":"2026-09-02T12:00:11.000Z","message":{"role":"user","content":"<task-notification>\n<task-id>abc</task-id>\n</task-notification>"}}
{"type":"user","timestamp":"2026-09-02T12:00:12.000Z","isMeta":true,"message":{"role":"user","content":"Caveat: injected context"}}
```

`internal/watcher/testdata/transcripts/list-text-prompt.jsonl`:

```
{"type":"assistant","timestamp":"2026-09-02T12:00:04.000Z","message":{"id":"msg_a","role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":50,"output_tokens":10}}}
{"type":"user","timestamp":"2026-09-02T12:05:00.000Z","message":{"role":"user","content":[{"type":"image","source":{"type":"base64","media_type":"image/png","data":""}},{"type":"text","text":"and this?"}]}}
```

`internal/watcher/testdata/transcripts/stopped-at-max-tokens.jsonl`:

```
{"type":"user","timestamp":"2026-09-02T12:00:01.000Z","message":{"role":"user","content":"write a novel"}}
{"type":"assistant","timestamp":"2026-09-02T12:00:05.000Z","message":{"id":"msg_a","role":"assistant","stop_reason":"max_tokens","content":[{"type":"text","text":"Chapter 1"}],"usage":{"input_tokens":50,"output_tokens":4096}}}
```

Replace `tool-running.jsonl` with the real one-block-per-line shape (a response's blocks are separate lines sharing an id; the thinking line has a null stop_reason):

```
{"type":"user","timestamp":"2026-09-02T12:00:01.000Z","message":{"role":"user","content":"do it"}}
{"type":"assistant","timestamp":"2026-09-02T12:00:01.500Z","message":{"id":"msg_a","role":"assistant","stop_reason":null,"content":[{"type":"thinking","thinking":"..."}],"usage":{"input_tokens":100,"output_tokens":20,"cache_read_input_tokens":5,"cache_creation_input_tokens":3}}}
{"type":"assistant","timestamp":"2026-09-02T12:00:02.000Z","message":{"id":"msg_a","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"t1","name":"Edit","input":{}}],"usage":{"input_tokens":100,"output_tokens":20,"cache_read_input_tokens":5,"cache_creation_input_tokens":3}}}
```

- [ ] **Step 2: Write the failing tests**

Append to `internal/watcher/transcript_test.go`:

```go
func kindAt(t *testing.T, fixture string) (watcher.Kind, time.Time, bool) {
	t.Helper()
	return watcher.DeriveKind(loadFixture(t, fixture))
}

func at(s string) time.Time {
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return tm
}

// Regression, issue #61: entries claude injects as the user - a finished
// background task, a local command and its output, isMeta context - were
// read as typed prompts and flipped a finished session back to thinking.
func TestDeriveKind_IgnoresInjectedUserEntries_issue61(t *testing.T) {
	kind, ts, ok := kindAt(t, "injected-after-done.jsonl")

	if !ok || kind != watcher.TurnEnded || !ts.Equal(at("2026-09-02T12:00:04Z")) {
		t.Errorf("DeriveKind = (%v, %v, %v), want (TurnEnded, 12:00:04, true): injected entries are not prompts", kind, ts, ok)
	}
}

// Regression, issue #62: a prompt sent as a list of text (and image) blocks
// set neither flag, so the tail skipped it and status and age stayed on the
// previous turn.
func TestDeriveKind_ListOfTextIsAPrompt_issue62(t *testing.T) {
	kind, ts, ok := kindAt(t, "list-text-prompt.jsonl")

	if !ok || kind != watcher.PromptSubmitted || !ts.Equal(at("2026-09-02T12:05:00Z")) {
		t.Errorf("DeriveKind = (%v, %v, %v), want (PromptSubmitted, 12:05:00, true)", kind, ts, ok)
	}
}

// Regression, issue #63: only end_turn counted as a finished turn, so a
// response stopped at max_tokens left the session at thinking forever.
func TestDeriveKind_AnyStopReasonButToolUseEndsTheTurn_issue63(t *testing.T) {
	kind, ts, ok := kindAt(t, "stopped-at-max-tokens.jsonl")

	if !ok || kind != watcher.TurnEnded || !ts.Equal(at("2026-09-02T12:00:05Z")) {
		t.Errorf("DeriveKind = (%v, %v, %v), want (TurnEnded, 12:00:05, true)", kind, ts, ok)
	}
}

func TestParseEntry_CarriesTheMessageID_issue59(t *testing.T) {
	e, ok := watcher.ParseEntry([]byte(
		`{"type":"assistant","timestamp":"2026-09-02T12:00:00Z","message":{"id":"msg_x","role":"assistant","content":[{"type":"text","text":"hi"}]}}`))
	if !ok || e.MessageID != "msg_x" {
		t.Errorf("entry = %+v, want MessageID msg_x", e)
	}
}
```

Also rewrite `TestDeriveFromTail_Fixtures_issue19` as a `DeriveKind` table (this test is retargeted in Task B3 when `DeriveFromTail` is deleted; do it now so the file compiles against the fixture change):

```go
func TestDeriveKind_Fixtures_issue19(t *testing.T) {
	tests := []struct {
		fixture string
		want    watcher.Kind
		wantAt  time.Time
		ok      bool
	}{
		{"prompt-sent.jsonl", watcher.PromptSubmitted, at("2026-09-02T12:00:01Z"), true},
		{"tool-running.jsonl", watcher.ToolStarted, at("2026-09-02T12:00:02Z"), true},
		{"tool-returned.jsonl", watcher.PromptSubmitted, at("2026-09-02T12:00:03Z"), true},
		{"turn-ended.jsonl", watcher.TurnEnded, at("2026-09-02T12:00:04Z"), true},
		{"noise-only.jsonl", 0, time.Time{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			kind, ts, ok := watcher.DeriveKind(loadFixture(t, tt.fixture))
			if ok != tt.ok || (ok && (kind != tt.want || !ts.Equal(tt.wantAt))) {
				t.Errorf("DeriveKind = (%v, %v, %v), want (%v, %v, %v)", kind, ts, ok, tt.want, tt.wantAt, tt.ok)
			}
		})
	}
}
```

Delete the old `TestDeriveFromTail_Fixtures_issue19` and the local `at` closure it declared.

- [ ] **Step 3: Run and read the failure**

Run: `go test ./internal/watcher/ -run 'DeriveKind|issue59' -v`
Expected: `issue61` FAILS with `(PromptSubmitted, 12:00:12…)`; `issue62` FAILS with `(TurnEnded, 12:00:04…)`; `issue63` FAILS with `(PromptSubmitted, 12:00:01…)`; `CarriesTheMessageID` FAILS with an empty MessageID. `Fixtures_issue19` passes.

- [ ] **Step 4: Implement**

In `internal/watcher/transcript.go`:

Add `MessageID string // assistant: one API response spans several lines under one id` to `Entry` after `Type`. Add `IsMeta bool \`json:"isMeta"\`` to `rawEntry` after `Timestamp`, and `ID string \`json:"id"\`` as the first field of `Message`. Add `"strings"` to the imports.

Replace `parseUser`:

```go
// injectedPrefixes open the user-role entries Claude Code writes itself: a
// finished background task, a local slash command and its output. None is a
// typed prompt, so none may move the status to thinking (issue #61).
var injectedPrefixes = []string{"<task-notification", "<command-", "<local-command-"}

func parseUser(r rawEntry) Entry {
	e := Entry{Type: "user", At: r.Timestamp}
	if r.IsMeta {
		return e // context claude injected, not something the operator typed
	}
	var s string
	if json.Unmarshal(r.Message.Content, &s) == nil {
		e.UserIsPrompt = !isInjected(s)
		return e
	}
	e.ToolResult = hasBlock(r.Message.Content, "tool_result")
	// A prompt with an attachment is a list of text and image blocks, not a
	// string (issue #62).
	e.UserIsPrompt = !e.ToolResult &&
		(hasBlock(r.Message.Content, "text") || hasBlock(r.Message.Content, "image"))
	return e
}

func isInjected(s string) bool {
	for _, p := range injectedPrefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}
```

In `parseAssistant`, add `MessageID: r.Message.ID,` after `Type`.

Add, and use it in place of `e.Type == "assistant" && e.StopReason == "end_turn"` in both `DeriveFromTail` and `DeriveKind`:

```go
// turnEnded reports an assistant entry that closed its turn. Claude stops at
// end_turn normally, but also at max_tokens, stop_sequence, refusal and
// pause_turn; only tool_use means the turn continues, and a null stop_reason
// is a mid-response line (issue #63).
func turnEnded(e Entry) bool {
	return e.Type == "assistant" && e.StopReason != "" && e.StopReason != "tool_use"
}
```

- [ ] **Step 5: Run**

Run: `go test ./internal/watcher/ -race`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/watcher/transcript.go internal/watcher/transcript_test.go internal/watcher/testdata/transcripts/
git commit -m "fix(#61): read claude's injected user entries and every stop reason correctly

Three parser bugs found against real transcripts: injected user-role entries
(task notifications, local command output, isMeta context) counted as typed
prompts (#61); a prompt written as a list of text and image blocks was
invisible (#62); only end_turn ended a turn, so max_tokens and stop_sequence
left a session at thinking forever (#63). Fixtures now use the real
one-block-per-line shape with message ids.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task B3: Count usage once per response; delete the dead derivation (#59, #78 partial)

**Files:**
- Modify: `internal/watcher/tailer.go:104-117`, `internal/watcher/event.go`, `internal/watcher/transcript.go` (delete `DeriveFromTail`, `SumUsage`)
- Modify: `internal/watcher/testdata/transcripts/usage.jsonl`
- Test: `internal/watcher/tailer_test.go`, `internal/watcher/transcript_test.go`

**Interfaces:**
- Produces: `func (t *Tokens) add(u Tokens)` in `event.go`; `Tailer.lastUsageID string`.

- [ ] **Step 1: Rewrite the usage fixture in the real shape**

`internal/watcher/testdata/transcripts/usage.jsonl` (one response `msg_a` across three lines, each repeating its usage, then a second response):

```
{"type":"assistant","timestamp":"2026-09-02T12:00:01.000Z","message":{"id":"msg_a","role":"assistant","stop_reason":null,"content":[{"type":"thinking","thinking":"..."}],"usage":{"input_tokens":10,"output_tokens":1,"cache_read_input_tokens":2,"cache_creation_input_tokens":3}}}
{"type":"assistant","timestamp":"2026-09-02T12:00:01.100Z","message":{"id":"msg_a","role":"assistant","stop_reason":null,"content":[{"type":"text","text":"on it"}],"usage":{"input_tokens":10,"output_tokens":1,"cache_read_input_tokens":2,"cache_creation_input_tokens":3}}}
{"type":"assistant","timestamp":"2026-09-02T12:00:01.200Z","message":{"id":"msg_a","role":"assistant","stop_reason":"tool_use","content":[{"type":"tool_use","id":"a","name":"Read","input":{}}],"usage":{"input_tokens":10,"output_tokens":1,"cache_read_input_tokens":2,"cache_creation_input_tokens":3}}}
{"type":"user","timestamp":"2026-09-02T12:00:02.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"a","content":"ok"}]}}
{"type":"assistant","timestamp":"2026-09-02T12:00:03.000Z","message":{"id":"msg_b","role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1000,"output_tokens":200,"cache_read_input_tokens":300,"cache_creation_input_tokens":400}}}
```

- [ ] **Step 2: Write the failing test; delete the `SumUsage` test**

Append to `internal/watcher/tailer_test.go` (add `"os"` is already imported):

```go
// Regression, issue #59: one response is written as one line per content
// block, each repeating the response's usage; summing per line doubled the
// counts in the header. usage.jsonl has msg_a on three lines.
func TestTailer_CountsUsageOncePerResponse_issue59(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "transcripts", "usage.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "s.jsonl")
	_ = os.WriteFile(path, fixture, 0o600)
	sink := make(chan watcher.Event, 8)
	tl := watcher.Tail("s1", path, sink, time.Now, time.Hour)
	defer tl.Close()

	tl.Poll()

	tok, ok := lastUsage(drain(sink))
	want := watcher.Tokens{In: 1010, Out: 201, CacheRead: 302, CacheWrite: 403}
	if !ok || tok != want {
		t.Errorf("usage = %+v (found=%v), want %+v: msg_a's three lines must count once", tok, ok, want)
	}
}
```

Delete `TestSumUsage_AddsAllFourCountersAcrossEntries_issue39` from `transcript_test.go` (its assertion lives in the tailer test above, against the real shape) and the `var _ = registry.StatusIdle` line plus the `registry` import from `tailer_test.go`.

- [ ] **Step 3: Run and read the failure**

Run: `go test ./internal/watcher/ -run issue59 -v`
Expected: FAIL with `usage = {In:1030 Out:203 CacheRead:306 CacheWrite:409}`.

- [ ] **Step 4: Implement**

In `internal/watcher/event.go`, after the `Tokens` type:

```go
// add accumulates one response's counters.
func (t *Tokens) add(u Tokens) {
	t.In += u.In
	t.Out += u.Out
	t.CacheRead += u.CacheRead
	t.CacheWrite += u.CacheWrite
}
```

In `internal/watcher/tailer.go`, add `lastUsageID string // the response whose usage was last counted (issue #59)` to the struct, and replace `ingest`:

```go
func (tl *Tailer) ingest(line []byte) {
	e, ok := ParseEntry(line)
	if !ok {
		return
	}
	// One API response is written as one line per content block, each
	// repeating the same usage under the same message id; count it once. A
	// line without an id (older transcripts, fixtures) still counts (issue #59).
	if e.Type == "assistant" && (e.MessageID == "" || e.MessageID != tl.lastUsageID) {
		tl.usage.add(e.Usage)
		tl.lastUsageID = e.MessageID
	}
	tl.ring = append(tl.ring, e)
	if len(tl.ring) > ringSize {
		tl.ring = tl.ring[len(tl.ring)-ringSize:]
	}
}
```

Reset `tl.lastUsageID = ""` inside `reconcileTruncation` alongside the other fields.

Delete `DeriveFromTail` and `SumUsage` from `transcript.go` (no production caller: `grep -rn "DeriveFromTail\|SumUsage" --include='*.go' .` must print nothing afterwards). Remove the now-unused `registry` import from `transcript.go`.

- [ ] **Step 5: Run**

Run: `go test ./internal/watcher/ -race && go vet ./...`
Expected: PASS.

- [ ] **Step 6: Commit (two commits)**

```bash
git add internal/watcher/tailer.go internal/watcher/event.go internal/watcher/tailer_test.go internal/watcher/testdata/transcripts/usage.jsonl
git commit -m "fix(#59): count a response's usage once, not once per content-block line

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
git add internal/watcher/transcript.go internal/watcher/transcript_test.go
git commit -m "refactor(#78): delete the unused tail derivation and usage sum

DeriveFromTail and SumUsage had no production caller; their tests now run
against DeriveKind and the tailer.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task B4: Bound every read and drop an oversized line (#64)

**Files:**
- Modify: `internal/watcher/tailer.go`
- Test: `internal/watcher/tailer_test.go`

**Interfaces:**
- Produces: constants `maxPollBytes`, `maxLineBytes`; `Tailer.skipping bool`; `func (tl *Tailer) drain(f *os.File) bool`.

- [ ] **Step 1: Write the failing test**

Append to `tailer_test.go` (add `"strings"` to its imports):

```go
// Regression, issue #64: one unterminated or oversized line was read and
// buffered whole, so a giant tool result could take the process down with
// every session in it. A line past the cap is dropped; the lines around it
// still count.
func TestTailer_DropsALineOverTheCapAndKeepsGoing_issue64(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	sink := make(chan watcher.Event, 8)
	tl := watcher.Tail("s1", path, sink, time.Now, time.Hour)
	defer tl.Close()
	huge := `{"type":"user","timestamp":"2026-09-02T12:00:09Z","message":{"content":"` +
		strings.Repeat("x", 2<<20) + `"}}` + "\n"
	_ = os.WriteFile(path, []byte(promptLine+huge), 0o600)

	tl.Poll()

	got := statusEvents(drain(sink))
	want, _ := time.Parse(time.RFC3339, "2026-09-02T12:00:01Z")
	if len(got) == 0 || !got[len(got)-1].At.Equal(want) {
		t.Errorf("derived %+v, want PromptSubmitted at %v: the 2 MiB line must be dropped, not read", got, want)
	}
}
```

- [ ] **Step 2: Run and read the failure**

Run: `go test ./internal/watcher/ -run issue64 -v`
Expected: FAIL with the event at `12:00:09` (the huge line was ingested).

- [ ] **Step 3: Implement**

In `tailer.go`, add after `ringSize`:

```go
// maxPollBytes bounds one read, so a large delta is consumed in chunks
// rather than allocated at once (issue #64).
const maxPollBytes = 1 << 20

// maxLineBytes bounds a single JSONL line. A longer one - a tool returning a
// huge file - is discarded whole rather than buffered without limit.
const maxLineBytes = 1 << 20
```

Add `skipping bool // inside a line over maxLineBytes; discard to the next newline` to the struct. Replace `Poll`, `consume`, and `readFrom`:

```go
// Poll reads whatever has been appended since the last call and emits at most
// one status event and one usage event. It is exported so tests drive it
// directly rather than waiting on a ticker. A missing file is not an error -
// the session has simply not spoken yet.
func (tl *Tailer) Poll() {
	f, err := os.Open(tl.path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	tl.reconcileTruncation(f)
	if tl.drain(f) {
		tl.emit()
	}
}

// drain reads everything appended since the last poll in chunks of at most
// maxPollBytes, so a large delta costs a bounded buffer rather than an
// allocation its own size (issue #64). It reports whether anything was read.
func (tl *Tailer) drain(f *os.File) bool {
	read := false
	for {
		fresh, err := readFrom(f, tl.offset)
		if err != nil || len(fresh) == 0 {
			return read
		}
		read = true
		tl.offset += int64(len(fresh))
		tl.consume(fresh)
		if len(fresh) < maxPollBytes {
			return true
		}
	}
}

// consume parses complete lines out of the fresh bytes, carrying any trailing
// partial line to the next poll so a line split across reads is not lost. A
// line over maxLineBytes, complete or not, is dropped (issue #64).
func (tl *Tailer) consume(fresh []byte) {
	buf := append(tl.partial, fresh...)
	for {
		i := bytes.IndexByte(buf, '\n')
		if i < 0 {
			break
		}
		if !tl.skipping && i <= maxLineBytes {
			tl.ingest(buf[:i])
		}
		tl.skipping = false
		buf = buf[i+1:]
	}
	if len(buf) > maxLineBytes {
		tl.skipping, buf = true, nil
	}
	tl.partial = append([]byte(nil), buf...)
}

func readFrom(f *os.File, offset int64) ([]byte, error) {
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(io.LimitReader(f, maxPollBytes))
}
```

In `reconcileTruncation`, also reset `tl.skipping = false`.

- [ ] **Step 4: Run**

Run: `go test ./internal/watcher/ -race`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/watcher/tailer.go internal/watcher/tailer_test.go
git commit -m "fix(#64): read the transcript in bounded chunks and drop a line over 1 MiB

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task B5: Make Close stop a parked tailer, guard the goroutine, emit only on change (#65, #66)

**Files:**
- Create: `internal/watcher/guard.go`, `internal/watcher/guard_test.go`
- Modify: `internal/watcher/tailer.go`
- Test: `internal/watcher/tailer_test.go`

**Interfaces:**
- Produces: `func (tl *Tailer) Done() <-chan struct{}`; `recoverLoop(role, sessionID string)`.

- [ ] **Step 1: Write the failing tests**

Append to `tailer_test.go`:

```go
// Regression, issue #65: Close only closed the stop channel, so a goroutine
// parked on a full sink never saw it and never exited.
func TestTailer_CloseUnblocksAPollParkedOnTheSink_issue65(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	_ = os.WriteFile(path, []byte(promptLine), 0o600)
	sink := make(chan watcher.Event) // nobody reads: the first emit parks
	tl := watcher.Tail("s1", path, sink, time.Now, time.Hour)
	polled := make(chan struct{})
	go func() { tl.Poll(); close(polled) }()

	tl.Close()

	select {
	case <-polled:
	case <-time.After(2 * time.Second):
		t.Fatal("Poll is still parked on the sink after Close")
	}
}

func TestTailer_DoneClosesWhenTheLoopExits_issue65(t *testing.T) {
	tl := watcher.Tail("s1", filepath.Join(t.TempDir(), "never"), make(chan watcher.Event, 1), time.Now, time.Hour)

	tl.Close()

	select {
	case <-tl.Done():
	case <-time.After(2 * time.Second):
		t.Fatal("the polling goroutine did not exit after Close")
	}
}

// Regression, issue #66: any append re-emitted the last status (same
// timestamp) and the usage, so noise lines doubled the event rate.
func TestTailer_NoiseOnlyAppendEmitsNothing_issue66(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	sink := make(chan watcher.Event, 8)
	tl := watcher.Tail("s1", path, sink, time.Now, time.Hour)
	defer tl.Close()
	_ = os.WriteFile(path, []byte(promptLine), 0o600)
	tl.Poll()
	_ = drain(sink)

	noise := `{"type":"file-history-snapshot","timestamp":"2026-09-02T12:00:02Z"}` + "\n" +
		`{"type":"queue-operation","operation":"dequeue"}` + "\n"
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = f.WriteString(noise)
	_ = f.Close()
	tl.Poll()

	if got := drain(sink); len(got) != 0 {
		t.Errorf("a noise-only append emitted %+v, want nothing: neither status nor usage changed", got)
	}
}
```

Create `internal/watcher/guard_test.go` (package `watcher`, internal):

```go
package watcher

import "testing"

// Invariant 6: a panic in a watcher goroutine must be swallowed, not fatal.
func TestRecoverLoop_SwallowsAPanic_issue65(t *testing.T) {
	func() {
		defer recoverLoop("test", "s1")
		panic("boom")
	}()
	// Reaching this line is the assertion.
}
```

- [ ] **Step 2: Run and read the failure**

Run: `go test ./internal/watcher/ -run 'issue65|issue66' -v`
Expected: `CloseUnblocks…` FAILS (`Poll is still parked`); `DoneCloses…` and the guard test fail to compile (`tl.Done undefined`, `undefined: recoverLoop`); `NoiseOnly…` FAILS with two events.

- [ ] **Step 3: Implement**

Create `internal/watcher/guard.go`:

```go
package watcher

import "log/slog"

// recoverLoop is deferred at the top of every long-lived watcher goroutine.
// Invariant 6: one session's panic must not take the app down. termwrap has
// the same guard for the emulator side; the status side had none (issue #65).
// The session's status simply goes stale.
func recoverLoop(role, sessionID string) {
	if r := recover(); r != nil {
		slog.Error("watcher goroutine panicked; its status is now stale",
			"role", role, "session", sessionID, "panic", r)
	}
}
```

In `tailer.go`, add to the struct:

```go
	last       Event  // the status event most recently sent, to skip repeats (issue #66)
	usageDirty bool   // usage changed since it was last sent
	stop       chan struct{}
	done       chan struct{}
	once       sync.Once
```

(replace the existing `stop` and `once` lines). In `Tail`, construct with `stop: make(chan struct{}), done: make(chan struct{})`. Replace `loop`, add `Done`, replace `emit`, add `send`:

```go
// Done is closed once the polling goroutine has exited, so a caller can prove
// Close actually stopped it (issue #65).
func (tl *Tailer) Done() <-chan struct{} { return tl.done }

func (tl *Tailer) loop(every time.Duration) {
	defer close(tl.done)
	defer recoverLoop("tailer", tl.sessionID)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-tl.stop:
			return
		case <-t.C:
			tl.Poll()
		}
	}
}

// emit sends the derived status if it changed and the usage total if it
// changed. Any append used to re-send both (issue #66).
func (tl *Tailer) emit() {
	kind, at, ok := DeriveKind(tl.ring)
	if ok && (kind != tl.last.Kind || !at.Equal(tl.last.At)) {
		tl.last = Event{Kind: kind, At: at}
		tl.send(Event{SessionID: tl.sessionID, Kind: kind, At: at})
	}
	if tl.usageDirty {
		tl.usageDirty = false
		tl.send(Event{SessionID: tl.sessionID, Kind: UsageUpdated, At: tl.clock(), Tokens: tl.usage})
	}
}

// send delivers ev unless the tailer is closed, so Close never leaves a
// goroutine parked on a full sink (issue #65).
func (tl *Tailer) send(ev Event) {
	select {
	case tl.sink <- ev:
	case <-tl.stop:
	}
}
```

In `ingest`, set `tl.usageDirty = true` inside the `if` that calls `tl.usage.add`. In `reconcileTruncation`, also reset `tl.last = Event{}` and set `tl.usageDirty = true` (the zeroed total must reach the sidebar).

- [ ] **Step 4: Run**

Run: `go test ./internal/watcher/ -race -count=5`
Expected: PASS every time.

- [ ] **Step 5: Commit**

```bash
git add internal/watcher/guard.go internal/watcher/guard_test.go internal/watcher/tailer.go internal/watcher/tailer_test.go
git commit -m "fix(#65): let Close stop a parked tailer and guard its goroutine

The sink sends now select on stop, Done exposes the goroutine's exit, and a
deferred recover keeps a parser panic inside one session (invariant 6). Status
and usage are emitted only when they change (#66).

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task B6: Gate, push, PR

- [ ] Run the full gate. Check `internal/watcher/tailer.go` is still under 500 lines and no function is over 20.
- [ ] Push and open the PR titled `fix(#59): read real transcripts correctly and bound the tailer` with `Closes #59, #60, #61, #62, #63, #64, #65, #66.` and `Part of #78.`
- [ ] Move the issues and PR to Review.

---

## Branch C — `fix/hook-listener`

### Task C1: Serve connections concurrently with a deadline; never block on the sink; stop on Close (#67)

**Files:**
- Modify: `internal/watcher/listener.go`
- Test: `internal/watcher/listener_test.go`

**Interfaces:**
- Produces: `func (l *Listener) Dropped() int64`; constants `readTimeout`, `maxInFlight`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/watcher/listener_test.go`:

```go
// Regression, issue #67: connections were served inline with no read
// deadline, so one peer that connected and sent nothing parked the accept
// loop and every later hook on the machine was never read.
func TestListen_ASilentPeerDoesNotStarveLaterHooks_issue67(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	sink := make(chan watcher.Event, 4)
	l, err := watcher.Listen(path, sink, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	silent, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = silent.Close() }()

	dial(t, path, `{"session_id":"after","hook_event_name":"Stop"}`)

	select {
	case ev := <-sink:
		if ev.SessionID != "after" {
			t.Errorf("got %+v, want the hook sent after the silent peer", ev)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a silent peer starved the next hook; the accept loop is parked on it")
	}
}

func TestListen_CloseReturnsWithASilentPeerConnected_issue67(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	l, err := watcher.Listen(path, make(chan watcher.Event, 1), time.Now)
	if err != nil {
		t.Fatal(err)
	}
	silent, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = silent.Close() }()

	closed := make(chan struct{})
	go func() { _ = l.Close(); close(closed) }()

	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("Close did not return while a silent peer was connected")
	}
}

// A full sink means the UI is behind; the tailer restores the truth within a
// second, so a hook event is dropped and counted rather than blocking the
// listener, which would stall every hook on the machine.
func TestListen_DropsInsteadOfBlockingOnAFullSink_issue67(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	l, err := watcher.Listen(path, make(chan watcher.Event), time.Now) // unbuffered, never read
	if err != nil {
		t.Fatal(err)
	}

	dial(t, path, `{"session_id":"a","hook_event_name":"Stop"}`)
	dial(t, path, `{"session_id":"b","hook_event_name":"Stop"}`)
	_ = l.Close() // waits for both connections to be served

	if got := l.Dropped(); got != 2 {
		t.Errorf("Dropped() = %d, want 2: both events had nowhere to go", got)
	}
}
```

- [ ] **Step 2: Run and read the failure**

Run: `go test ./internal/watcher/ -run issue67 -v`
Expected: the first two FAIL on their timeouts; the third fails to compile (`l.Dropped undefined`).

- [ ] **Step 3: Rewrite the listener**

Replace `internal/watcher/listener.go` from the `Listener` type to the end of the file with:

```go
// readTimeout bounds how long a connected hook may take to send its line. A
// hook writes immediately, so a slower peer is stuck or hostile and must not
// hold a slot (issue #67).
const readTimeout = 2 * time.Second

// maxInFlight bounds concurrent connections. A hook is one line, so a burst
// beyond this waits in the kernel backlog rather than spawning goroutines.
const maxInFlight = 32

// Listener turns hook connections on a unix socket into Events.
type Listener struct {
	ln      net.Listener
	clock   func() time.Time
	sink    chan<- Event
	stop    chan struct{}
	slots   chan struct{}
	wg      sync.WaitGroup
	once    sync.Once
	mu      sync.Mutex
	conns   map[net.Conn]struct{}
	dropped atomic.Int64
}

// Listen accepts hook connections on path and sends an Event per valid payload
// to sink. A stale socket file is replaced; the socket is user-only. clock
// stamps each event so it compares like-for-like with tailer timestamps.
//
//	l, err := watcher.Listen(paths.HookSocket(home), events, time.Now)
//	defer l.Close()
func Listen(path string, sink chan<- Event, clock func() time.Time) (*Listener, error) {
	if err := refuseIfLive(path); err != nil {
		return nil, err
	}
	// A leftover socket file from a previous run makes bind fail; remove it.
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("watcher: clearing stale socket %q: %w", path, err)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, fmt.Errorf("watcher: listening on %q: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("watcher: securing socket %q: %w", path, err)
	}
	l := &Listener{ln: ln, clock: clock, sink: sink, stop: make(chan struct{}),
		slots: make(chan struct{}, maxInFlight), conns: map[net.Conn]struct{}{}}
	l.wg.Add(1)
	go l.accept()
	return l, nil
}

// refuseIfLive returns an error when another omatty already answers on path,
// so a second instance degrades to tailer-only instead of stealing the socket
// from the first (issue #68). A stale file from a dead process does not
// answer and is removed as before.
func refuseIfLive(path string) error {
	c, err := net.DialTimeout("unix", path, 200*time.Millisecond)
	if err != nil {
		return nil
	}
	_ = c.Close()
	return fmt.Errorf("watcher: another omatty is listening on %q; hook status is disabled in this instance", path)
}

// Close stops accepting, closes every in-flight connection, and waits for the
// goroutines to exit (issue #67).
func (l *Listener) Close() error {
	var err error
	l.once.Do(func() {
		close(l.stop)
		err = l.ln.Close()
		l.closeConns()
	})
	l.wg.Wait()
	return err
}

// Dropped counts hook events that found the sink full. A non-zero value
// means the UI fell behind; the tailer has since restored the truth.
func (l *Listener) Dropped() int64 { return l.dropped.Load() }

func (l *Listener) accept() {
	defer l.wg.Done()
	defer recoverServe("accept loop")
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			return // listener closed
		}
		select {
		case l.slots <- struct{}{}:
		case <-l.stop:
			_ = conn.Close()
			return
		}
		l.track(conn, true)
		l.wg.Add(1)
		go l.serve(conn)
	}
}

// serve reads one bounded line from conn within readTimeout and, if it is a
// tracked event, offers it to the sink. One connection per hook.
func (l *Listener) serve(conn net.Conn) {
	defer l.wg.Done()
	defer func() { <-l.slots }()
	defer recoverServe("connection")
	defer l.track(conn, false)
	defer func() { _ = conn.Close() }()
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
	line, ok := readLine(conn)
	if !ok {
		return
	}
	var p hooks.Payload
	if json.Unmarshal(line, &p) != nil {
		return
	}
	if kind, ok := KindOf(p); ok {
		l.offer(Event{SessionID: p.SessionID, Kind: kind, At: l.clock(), Tool: p.ToolName})
	}
}

// offer sends without blocking. A full sink means the UI is behind; the
// tailer restores the truth within a second, so dropping a hook event costs
// only latency, while blocking would stall every hook on the machine.
func (l *Listener) offer(ev Event) {
	select {
	case l.sink <- ev:
	default:
		l.dropped.Add(1)
		slog.Debug("hook event dropped, sink full", "session", ev.SessionID)
	}
}

func (l *Listener) track(conn net.Conn, add bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if add {
		l.conns[conn] = struct{}{}
		return
	}
	delete(l.conns, conn)
}

func (l *Listener) closeConns() {
	l.mu.Lock()
	defer l.mu.Unlock()
	for c := range l.conns {
		_ = c.Close()
	}
}

// recoverServe keeps a panic inside one connection (invariant 6). Branch F
// unifies this with the tailer's guard.
func recoverServe(role string) {
	if r := recover(); r != nil {
		slog.Error("hook listener panicked", "role", role, "panic", r)
	}
}

// readLine reads one payload, capped at maxLine. ok is false for an empty read
// or an oversized line, both of which are dropped.
func readLine(conn net.Conn) ([]byte, bool) {
	line, err := bufio.NewReaderSize(conn, maxLine).ReadSlice('\n')
	if (err != nil && len(line) == 0) || len(line) >= maxLine {
		if len(line) >= maxLine {
			slog.Debug("hook payload exceeded the cap, dropped")
		}
		return nil, false
	}
	return line, true
}
```

Add `"sync"` and `"sync/atomic"` to the imports.

- [ ] **Step 4: Run**

Run: `go test ./internal/watcher/ -race -count=5 -run Listen`
Expected: PASS every time, including the pre-existing listener tests.

- [ ] **Step 5: Commit**

```bash
git add internal/watcher/listener.go internal/watcher/listener_test.go
git commit -m "fix(#67): serve hook connections concurrently with a deadline and never block on the sink

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task C2: Refuse to steal a live socket (#68)

**Files:**
- Test: `internal/watcher/listener_test.go` (the code landed in C1's `refuseIfLive`; this task proves it)

- [ ] **Step 1: Write the test**

```go
// Regression, issue #68: a second omatty unlinked the first one's socket and
// bound its own; the first kept accepting on a nameless inode and never saw
// another hook, with no log line.
func TestListen_RefusesWhenAnotherInstanceIsLive_issue68(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	sink := make(chan watcher.Event, 4)
	first, err := watcher.Listen(path, sink, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = first.Close() }()

	second, err := watcher.Listen(path, make(chan watcher.Event, 1), time.Now)

	if err == nil {
		_ = second.Close()
		t.Fatal("a second Listen on a live socket succeeded, want an error so it degrades to tailer-only")
	}
	dial(t, path, `{"session_id":"still","hook_event_name":"Stop"}`)
	select {
	case ev := <-sink:
		if ev.SessionID != "still" {
			t.Errorf("first instance got %+v", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the first instance lost its socket")
	}
}
```

- [ ] **Step 2: Run**

Run: `go test ./internal/watcher/ -run issue68 -v`
Expected: PASS (C1 landed the code). To see it fail for the bug's reason, `git stash` the listener change once: the second `Listen` succeeds and the first never receives.

- [ ] **Step 3: Commit**

```bash
git add internal/watcher/listener_test.go
git commit -m "fix(#68): refuse to bind the hook socket while another omatty answers on it

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task C3: Gate, push, PR

- [ ] Full gate. `internal/watcher/listener.go` must stay under 500 lines; every function under 20.
- [ ] PR titled `fix(#67): harden the hook socket listener` with `Closes #67, #68.`
- [ ] Move to Review.

---

## Branch D — `fix/ui-notify-tick`

### Task D1: Notify off the Update goroutine, only for registered sessions, rate-limited (#69)

**Files:**
- Modify: `internal/ui/model.go` (`Model` fields, `NewModel`, `SetEvents`, `onStatus`, `maybeNotify`, `needsYou`)
- Modify: `internal/notify/notify.go`; Create: `internal/notify/notify_darwin.go`, `internal/notify/notify_other.go`
- Modify: `internal/ui/run.go:92`
- Test: `internal/ui/notify_test.go`, `internal/notify/notify_test.go`

**Interfaces:**
- Produces: `notify.New() Notifier`, `notify.Silent`; `ui.notifyCmd`; `Model.notified map[string]time.Time`, `Model.startedAt time.Time`; `needsYou(title string, status registry.Status) (string, bool)` (the ignored first parameter is gone).

- [ ] **Step 1: Rewrite the notify tests to run the returned command**

Replace `modelWithNotifier` and every test in `internal/ui/notify_test.go` with:

```go
// modelWithNotifier returns a model whose clock the test can move, with no
// event channel so the command Update returns is the notification alone.
func modelWithNotifier(t *testing.T) (*ui.Model, *notify.Fake, *time.Time) {
	t.Helper()
	m, _ := modelWithFakes(t)
	now := fixedNow
	m.SetEvents(nil, func() time.Time { return now })
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	n := &notify.Fake{}
	m.SetNotifier(n)
	return m, n, &now
}

// runCmd executes a command tree synchronously, so a notification posted as a
// tea.Cmd lands in the fake before the assertion (issue #69).
func runCmd(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	if batch, ok := cmd().(tea.BatchMsg); ok {
		for _, c := range batch {
			runCmd(c)
		}
	}
}

func status(m *ui.Model, id string, kind watcher.Kind, at time.Time) {
	_, cmd := m.Update(ui.StatusMsg{SessionID: id, Kind: kind, At: at})
	runCmd(cmd)
}

func TestModel_NotifiesWhenAWaitingSessionArrivesWhileBlurred_issue38(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "s1", watcher.PermissionRequested, fixedNow)

	if len(n.Sent) != 1 {
		t.Fatalf("sent %d notifications, want 1 for a waiting session while blurred", len(n.Sent))
	}
	if got := n.Sent[0].Body; got == "" {
		t.Error("the notification body is empty")
	}
}

func TestModel_DoesNotNotifyWhileFocused_issue38(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.FocusMsg{})

	status(m, "s1", watcher.PermissionRequested, fixedNow)

	if len(n.Sent) != 0 {
		t.Errorf("sent %d notifications while focused, want 0", len(n.Sent))
	}
}

func TestModel_DoesNotNotifyTwiceForTheSameState_issue38(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "s1", watcher.PermissionRequested, fixedNow)
	status(m, "s1", watcher.PermissionRequested, fixedNow.Add(time.Second))

	if len(n.Sent) != 1 {
		t.Errorf("sent %d notifications, want 1: a repeated waiting state must not re-notify", len(n.Sent))
	}
}

func TestModel_DoesNotNotifyForThinkingOrTool_issue38(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "s1", watcher.PromptSubmitted, fixedNow)
	status(m, "s1", watcher.ToolStarted, fixedNow.Add(time.Second))

	if len(n.Sent) != 0 {
		t.Errorf("sent %d notifications for thinking/tool, want 0 (those do not need you)", len(n.Sent))
	}
}

func TestModel_NotifiesWhenADoneSessionArrivesWhileBlurred_issue38(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "s1", watcher.TurnEnded, fixedNow)

	if len(n.Sent) != 1 {
		t.Errorf("sent %d notifications for a finished turn while blurred, want 1", len(n.Sent))
	}
}

// Regression, issue #69: the notifier ran inline in Update, freezing every
// pane for the ~40 ms osascript takes. It must come back as a command.
func TestModel_NotificationIsACommandNotAnInlineCall_issue69(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	_, cmd := m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: fixedNow})

	if len(n.Sent) != 0 {
		t.Fatal("Notify was called inside Update; it must run as a command off the event loop")
	}
	runCmd(cmd)
	if len(n.Sent) != 1 {
		t.Errorf("running the returned command sent %d notifications, want 1", len(n.Sent))
	}
}

// Regression, issue #69: any session id on the socket grew the status map and
// reached the notification body. Unregistered ids are dropped.
func TestModel_IgnoresAStatusEventForAnUnknownSession_issue69(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "not-registered", watcher.PermissionRequested, fixedNow)

	if len(n.Sent) != 0 {
		t.Errorf("an unregistered session id produced %+v, want nothing", n.Sent)
	}
}

// Regression, issue #69: a permission loop notified on every transition.
func TestModel_RateLimitsNotificationsPerSession_issue69(t *testing.T) {
	m, n, now := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "s1", watcher.PermissionRequested, fixedNow)
	status(m, "s1", watcher.TurnEnded, fixedNow.Add(time.Second))
	if len(n.Sent) != 1 {
		t.Fatalf("sent %d within the cooldown, want 1", len(n.Sent))
	}

	*now = fixedNow.Add(10 * time.Second)
	status(m, "s1", watcher.PermissionRequested, *now)
	if len(n.Sent) != 2 {
		t.Errorf("sent %d after the cooldown elapsed, want 2", len(n.Sent))
	}
}
```

- [ ] **Step 2: Run and read the failure**

Run: `go test ./internal/ui/ -run 'issue38|issue69' -v`
Expected: `NotificationIsACommand…` FAILS (`Notify was called inside Update`); `IgnoresAStatusEvent…` FAILS (1 sent); `RateLimits…` FAILS (2 within the cooldown). The `issue38` tests pass.

- [ ] **Step 3: Implement in `model.go`**

Add to the `Model` struct after `notifier`:

```go
	// notified is when each session last posted a notification (issue #69).
	notified map[string]time.Time
	// startedAt gates notifications to transitions newer than this run: the
	// first tailer poll replays old turns (issue #70).
	startedAt time.Time
```

In `NewModel`, add `notified: map[string]time.Time{}, startedAt: time.Now(),`. In `SetEvents`, after setting the clock: `m.startedAt = m.clock()`.

Replace `onStatus`, `maybeNotify`, and `needsYou`:

```go
// onStatus folds a watcher event into the session's state and re-arms the
// wait. Newer-wins lives in watcher.Apply; the model just stores the result.
// A hook can name any session id; only registered ones may grow the status
// map or reach the operator's notifications (issue #69).
func (m *Model) onStatus(ev StatusMsg) tea.Cmd {
	e := watcher.Event(ev)
	if !m.knownSession(e.SessionID) {
		return m.waitForEvent()
	}
	before := m.status[e.SessionID].Status
	after := watcher.Apply(m.status[e.SessionID], e)
	m.status[e.SessionID] = after
	m.sidebar.SetRows(SidebarRows(m.state, m.statusMap()))
	return tea.Batch(m.waitForEvent(), m.maybeNotify(e, before, after.Status))
}

func (m *Model) knownSession(id string) bool {
	for i := range m.state.Sessions {
		if m.state.Sessions[i].ID == id {
			return true
		}
	}
	return false
}

// notifyCooldown is the least time between two notifications for one
// session, so a permission loop cannot storm the desktop (issue #69).
const notifyCooldown = 5 * time.Second

// maybeNotify returns a command that posts a desktop notification when a
// session enters a state that needs the operator while omatty is
// backgrounded. It is a command, off the Update goroutine, because osascript
// takes tens of milliseconds (issue #69). Suppressed: a repeated state, a
// transition older than this run (issue #70), and a second notification for
// the same session within notifyCooldown.
func (m *Model) maybeNotify(e watcher.Event, before, after registry.Status) tea.Cmd {
	if m.notifier == nil || m.hasFocus || before == after || e.At.Before(m.startedAt) {
		return nil
	}
	body, ok := needsYou(m.sessionTitle(e.SessionID), after)
	if !ok || !m.cooldownElapsed(e.SessionID) {
		return nil
	}
	return notifyCmd(m.notifier, body)
}

func (m *Model) cooldownElapsed(id string) bool {
	now := m.clock()
	if last, ok := m.notified[id]; ok && now.Sub(last) < notifyCooldown {
		return false
	}
	m.notified[id] = now
	return true
}

func notifyCmd(n notify.Notifier, body string) tea.Cmd {
	return func() tea.Msg {
		if err := n.Notify("omatty", body); err != nil {
			slog.Warn("desktop notification failed", "body", body, "err", err)
		}
		return nil
	}
}

// needsYou returns the notification body for a status that wants attention.
func needsYou(title string, status registry.Status) (string, bool) {
	switch status {
	case registry.StatusWaiting:
		return title + " needs you", true
	case registry.StatusDone:
		return title + " finished", true
	default:
		return "", false
	}
}
```

- [ ] **Step 4: Platform notifier**

`internal/notify/notify.go`: change the `Osascript` doc to `// Osascript posts via macOS's osascript. New picks it on darwin; other platforms get Silent (issue #69).` and append:

```go
// Silent is the notifier for platforms without a delivery path yet. It
// reports success so the model's bookkeeping behaves the same everywhere.
type Silent struct{}

// Notify does nothing.
func (Silent) Notify(string, string) error { return nil }
```

Create `internal/notify/notify_darwin.go`:

```go
//go:build darwin

package notify

// New returns the notifier for this platform.
//
//	model.SetNotifier(notify.New())
func New() Notifier { return Osascript{} }
```

Create `internal/notify/notify_other.go`:

```go
//go:build !darwin

package notify

// New returns the notifier for this platform: Silent, until a Linux or
// Windows delivery path exists.
//
//	model.SetNotifier(notify.New())
func New() Notifier { return Silent{} }
```

Append to `internal/notify/notify_test.go`:

```go
func TestNew_ReturnsANotifierForThisPlatform_issue69(t *testing.T) {
	if notify.New() == nil {
		t.Fatal("New() returned nil")
	}
}

func TestSilent_ReportsSuccess_issue69(t *testing.T) {
	if err := (notify.Silent{}).Notify("omatty", "x"); err != nil {
		t.Errorf("Silent.Notify = %v, want nil", err)
	}
}
```

In `internal/ui/run.go`, change `model.SetNotifier(notify.Osascript{})` to `model.SetNotifier(notify.New())`.

- [ ] **Step 5: Run**

Run: `go test ./internal/ui/ ./internal/notify/ -race`
Expected: PASS. Check `internal/ui/status_test.go`'s `modelWithEvents` still compiles (it does; it uses `SetEvents` with a channel).

- [ ] **Step 6: Commit**

```bash
git add internal/ui/model.go internal/ui/run.go internal/ui/notify_test.go internal/notify/
git commit -m "fix(#69): post notifications as a command, only for registered sessions, rate-limited

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task D2: No notifications for turns older than this run (#70)

**Files:**
- Test: `internal/ui/notify_test.go` (the gate landed in D1's `maybeNotify`; this task proves it)

- [ ] **Step 1: Write the test**

```go
// Regression, issue #70: the first tailer poll replays the transcript, and a
// "finished" for a turn that ended days ago fired as soon as omatty was
// backgrounded.
func TestModel_DoesNotNotifyForATurnThatEndedBeforeStart_issue70(t *testing.T) {
	m, n, _ := modelWithNotifier(t)
	m.Update(tea.BlurMsg{})

	status(m, "s1", watcher.TurnEnded, fixedNow.Add(-168*time.Hour))
	if len(n.Sent) != 0 {
		t.Fatalf("notified about a turn that ended a week before this run: %+v", n.Sent)
	}

	status(m, "s1", watcher.PermissionRequested, fixedNow.Add(time.Second))
	if len(n.Sent) != 1 {
		t.Errorf("a transition after start sent %d notifications, want 1", len(n.Sent))
	}
}
```

- [ ] **Step 2: Run**

Run: `go test ./internal/ui/ -run issue70 -v`
Expected: PASS. Comment out the `e.At.Before(m.startedAt)` clause once to watch it fail with the week-old notification, then restore it.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/notify_test.go
git commit -m "fix(#70): never notify for a transition older than this run

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task D3: A once-a-second tick so the age column keeps counting (#71)

**Files:**
- Modify: `internal/ui/model.go` (`Init`, `Update`, `View`, `renderSidebar`, `renderRow`, `terminalTitle`)
- Test: `internal/ui/status_test.go`

**Interfaces:**
- Produces: `ui.TickMsg`, `scheduleTick() tea.Cmd`, `tickEvery`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/status_test.go`:

```go
// Regression, issue #71: the age was computed at render time, but nothing
// triggered a render on a quiet session, so "<1m" stayed on screen for hours.
func TestModel_TickReArmsItself_issue71(t *testing.T) {
	m, _ := modelWithFakes(t)

	_, cmd := m.Update(ui.TickMsg(fixedNow))

	if cmd == nil {
		t.Error("a tick returned no command; the age column would freeze after the first second")
	}
}

func TestModel_InitSchedulesATick_issue71(t *testing.T) {
	m := ui.NewModel(emptyState(), map[string]termwrap.Terminal{}, noCreate, noStart)

	if m.Init() == nil {
		t.Error("Init scheduled nothing with no terminals; the tick must be there regardless")
	}
}
```

- [ ] **Step 2: Run and read the failure**

Run: `go test ./internal/ui/ -run issue71 -v`
Expected: compile error `undefined: ui.TickMsg`; add `type TickMsg time.Time` temporarily and rerun: both FAIL (nil command).

- [ ] **Step 3: Implement**

In `model.go`, after `StatusMsg`:

```go
// TickMsg is the once-a-second heartbeat that re-renders the frame, so a
// quiet session's age keeps counting (issue #71). Exported so tests can send
// one.
type TickMsg time.Time

// tickEvery is the age column's resolution; finer buys nothing.
const tickEvery = time.Second

func scheduleTick() tea.Cmd {
	return tea.Tick(tickEvery, func(t time.Time) tea.Msg { return TickMsg(t) })
}
```

In `Init`, before the `return`: `cmds = append(cmds, scheduleTick())`. In `Update`, add `case TickMsg: return m, scheduleTick()` before the `FocusMsg` case.

Hoist the clock once per frame: `View` computes `now := m.clock()` and passes it to `m.renderSidebar(termH, now)` and `m.renderTerminal(termW, termH, now)`; `renderSidebar(rows int, now time.Time)` passes it to `m.renderRow(row, inner, now)`; `renderTerminal(w, h int, now time.Time)` passes it to `m.terminalTitle(w, now)`; `renderRow` and `terminalTitle` use `now` in place of `m.clock()`.

- [ ] **Step 4: Run**

Run: `go test ./internal/ui/ -race`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/model.go internal/ui/status_test.go
git commit -m "fix(#71): tick once a second so the sidebar age keeps counting on a quiet session

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task D4: Close every terminal on exit (#72)

**Files:**
- Modify: `internal/ui/run.go` (`Run`)
- Create: `internal/ui/run_internal_test.go`

- [ ] **Step 1: Write the failing test (internal package)**

```go
package ui

import (
	"testing"

	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

// Regression, issue #72: Run deferred the listener and tailer closers only,
// so the claude children were left for the OS to reap at exit.
func TestCloseTerminals_ClosesEveryOne_issue72(t *testing.T) {
	a, b := termwrap.NewFake(""), termwrap.NewFake("")

	closeTerminals(map[string]termwrap.Terminal{"a": a, "b": b})

	if !a.Closed || !b.Closed {
		t.Errorf("closed a=%v b=%v, want both", a.Closed, b.Closed)
	}
}
```

- [ ] **Step 2: Run**

Run: `go test ./internal/ui/ -run issue72`
Expected: compile error `undefined: closeTerminals`.

- [ ] **Step 3: Implement**

In `run.go`, `Run` gains `defer closeTerminals(terms)` immediately after `StartTerminals` succeeds (before the listener starts), and:

```go
// closeTerminals ends every claude process on the way out (issue #72). The
// map is the one the model adds runtime sessions to, so those close too.
// Until now the OS closed the PTY masters at exit, which is neither a
// guarantee nor omatty's decision.
func closeTerminals(terms map[string]termwrap.Terminal) {
	for id, t := range terms {
		if err := t.Close(); err != nil {
			slog.Warn("closing a terminal on exit", "session", id, "err", err)
		}
	}
}
```

- [ ] **Step 4: Run, commit**

Run: `go test ./internal/ui/ -race`

```bash
git add internal/ui/run.go internal/ui/run_internal_test.go
git commit -m "fix(#72): close every embedded terminal when omatty exits

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task D5: Gate, push, PR

- [ ] Full gate. `model.go` was already over 500 lines on develop (#76 fixes that in F); do not grow it further than these tasks require.
- [ ] PR titled `fix(#69): notify off the event loop, tick the age, close terminals on exit` with `Closes #69, #70, #71, #72.`
- [ ] Move to Review.

---

## Branch E — PR #52 follow-ups on `fix/terminal-birth-size`

### Task E1: One helper for the PTY size, one row shorter than the pane (#75)

**Files:**
- Modify: `internal/ui/layout.go`, `internal/ui/run.go:28`, `internal/ui/model.go` (`onResize`, `restartFocused`)
- Test: `internal/ui/layout_test.go`, `internal/ui/run_test.go`

**Interfaces:**
- Produces: `ui.PTYSize(width, height int) (w, h int)`; exported `ui.DefaultWidth`, `ui.DefaultHeight`.

- [ ] **Step 1: Write the failing tests**

Append to `layout_test.go`:

```go
// Regression, issue #75: the PTY was born and resized at PaneSize, but the
// pane spends its first row on the title and renders h-1 rows, so claude's
// bottom line was always clipped.
func TestPTYSize_IsOneRowShorterThanThePane_issue75(t *testing.T) {
	w, h := ui.PTYSize(120, 40)

	if w != 90 || h != 36 {
		t.Errorf("PTYSize(120, 40) = (%d, %d), want (90, 36): PaneSize 90x37 minus the title row", w, h)
	}
}
```

In `run_test.go`, change `TestStartTerminals_BirthsThePTYAtThePaneSize_issue51` to hardcode the expectation:

```go
	// PaneSize(140, 40) is 110x37; the title row takes one (issue #75).
	if gotW != 110 || gotH != 36 {
		t.Errorf("PTY started at %dx%d, want 110x36 (not the 140x40 window)", gotW, gotH)
	}
```

and delete the `wantW, wantH := ui.PaneSize(140, 40)` line. Rename `oneProject1` to `oneSessionState` (both the definition and the call; the name must say what distinguishes it from `oneProject`, which has no sessions).

In `layout_test.go`, `TestModel_ResizePassesPaneSizeToTheFocusedTerminal_issue35` now wants `90x36` with the comment `// PaneSize 90x37 minus the title row (issue #75).`

- [ ] **Step 2: Run and read the failure**

Run: `go test ./internal/ui/ -run 'issue75|issue51|issue35' -v`
Expected: `PTYSize` undefined; then after a stub `func PTYSize(w, h int) (int, int) { return PaneSize(w, h) }`, the three FAIL on the 37-vs-36 row.

- [ ] **Step 3: Implement**

In `layout.go`, export the defaults and add `PTYSize`:

```go
// DefaultWidth and DefaultHeight are the size assumed before the terminal
// reports its own, and the fallback cmd uses when it cannot query one.
const (
	DefaultWidth  = 80
	DefaultHeight = 24
)

// PTYSize is the embedded terminal's size for a window: the pane's content
// minus the title row the pane draws above it. It is the one place the PTY
// dimensions are derived, for birth and for every resize, so the two can
// never drift (issues #51, #75).
//
//	w, h := ui.PTYSize(120, 40) // 90, 36
func PTYSize(width, height int) (w, h int) {
	w, h = PaneSize(width, height)
	return w, h - 1
}
```

Replace `defaultWidth`/`defaultHeight` uses in `model.go` (`NewModel`) with the exported names. In `run.go` `StartTerminals`, use `PTYSize(w, h)`. In `model.go` `onResize`, use `term.Resize(PTYSize(msg.Width, msg.Height))`; in `restartFocused`, `PTYSize(m.width, m.height)` (E2 removes that Resize altogether).

- [ ] **Step 4: Run, commit**

Run: `go test ./internal/ui/ -race`

```bash
git add internal/ui/layout.go internal/ui/layout_test.go internal/ui/run.go internal/ui/run_test.go internal/ui/model.go
git commit -m "fix(#75): size the PTY one row shorter than the pane, from one helper

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task E2: Live size for new sessions, restarts, and focus changes (#73)

**Files:**
- Modify: `internal/ui/model.go` (`StartFunc`, `navigate`, `restartFocused`, `addSession`), `internal/ui/run.go` (`guardedStarter`, `StartTerminals`)
- Test: `internal/ui/model_test.go`, `internal/ui/refresh_test.go`, `internal/ui/restart_test.go`, `internal/ui/run_test.go`

**Interfaces:**
- Produces: `type StartFunc func(sess registry.Session, w, h int) (termwrap.Terminal, error)`; `func (m *Model) ptySize() (int, int)`; `func (m *Model) resizeFocused() tea.Cmd`; `startRecorder.W, startRecorder.H`.

- [ ] **Step 1: Change the signature everywhere, passing the frozen default at first (so the new birth-size tests fail for the bug's reason)**

`model.go`:

```go
// StartFunc launches the embedded terminal for a session at w by h. Injected
// so the model can start a session created at runtime without knowing how;
// the size is a parameter so it is never frozen at startup (issue #73).
type StartFunc func(sess registry.Session, w, h int) (termwrap.Terminal, error)
```

Temporarily call it as `m.start(sess, PTYSize(DefaultWidth, DefaultHeight))` in both `restartFocused` and `addSession`. `run.go`:

```go
// guardedStarter starts a session's terminal wrapped in a panic guard
// (invariant 6). The model passes the live pane size on every call.
func guardedStarter(l *supervisor.Launcher, f termwrap.Factory) StartFunc {
	return func(sess registry.Session, w, h int) (termwrap.Terminal, error) {
		term, err := l.Start(f, sess, w, h)
		if err != nil {
			return nil, err
		}
		return termwrap.NewGuard(term), nil
	}
}
```

Update `Run` to `start := guardedStarter(l, f)`. Tests: `noStart` becomes `func noStart(registry.Session, int, int) (termwrap.Terminal, error)`; `startRecorder` gains `W, H int` and its `fn` becomes `func (s *startRecorder) fn(sess registry.Session, w, h int) (termwrap.Terminal, error)` recording `s.W, s.H = w, h`.

- [ ] **Step 2: Write the failing tests**

Append to `model_test.go`:

```go
// Regression, issue #73: only the focused terminal tracked the window, and a
// focus change did not resize the terminal just focused, so j/k onto a
// session showed claude painted at its birth width inside a wider box - the
// #51 symptom again.
func TestModel_FocusChangeResizesTheNewlyFocusedTerminal_issue73(t *testing.T) {
	m, fakes := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	press(m, ctrl('o'))
	press(m, key('j'))

	if f := fakes["s2"]; f.Width != 90 || f.Height != 36 {
		t.Errorf("newly focused s2 is %dx%d, want PTYSize(120,40) = 90x36", f.Width, f.Height)
	}
}
```

Replace the comment above `TestModel_WindowResizeReachesOnlyTheFocusedTerminal` with `// Only the focused terminal is resized on a window change; the others catch up when focused (issue #73).`

Append to `refresh_test.go`:

```go
// Regression, issue #73: the StartFunc closure froze the pane size at Run
// time, so a session created after a window resize was born at the startup
// size and never resized.
func TestModel_SessionCreatedAfterAResizeIsBornAtTheCurrentPTYSize_issue73(t *testing.T) {
	c, s := &liveCreate{}, &startRecorder{}
	m := ui.NewModel(oneProject(), map[string]termwrap.Terminal{}, c.fn, s.fn)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})

	newSession(m, "late")

	if s.W != 170 || s.H != 56 {
		t.Errorf("born at %dx%d, want PTYSize(200,60) = 170x56, not the startup size", s.W, s.H)
	}
}
```

Append to `restart_test.go`:

```go
// Regression, issue #73: restart birthed at the frozen size and resized
// afterwards, the very race #51 removed for startup.
func TestModel_RestartBirthsAtTheCurrentPTYSize_issue73(t *testing.T) {
	s := &startRecorder{}
	m, _ := modelWithStarter(t, s)
	m.Update(tea.WindowSizeMsg{Width: 200, Height: 60})

	press(m, ctrl('o'))
	press(m, key('r'))

	if s.W != 170 || s.H != 56 {
		t.Errorf("restarted at %dx%d, want PTYSize(200,60) = 170x56", s.W, s.H)
	}
}
```

- [ ] **Step 3: Run and read the failure**

Run: `go test ./internal/ui/ -run issue73 -v`
Expected: three FAIL: s2 is `0x0`; born at `50x20`; restarted at `50x20`.

- [ ] **Step 4: Implement**

In `model.go`:

```go
// ptySize is the live embedded-terminal size for the current window.
func (m *Model) ptySize() (int, int) { return PTYSize(m.width, m.height) }

// resizeFocused sizes the newly focused terminal to the pane. Only the
// focused terminal follows the window (issue #34), so the one just focused
// may still be at the size it was born or last focused at (issue #73).
func (m *Model) resizeFocused() tea.Cmd {
	term := m.focusedTerminal()
	if term == nil {
		return nil
	}
	return term.Resize(m.ptySize())
}
```

In `navigate`, the `j` and `k` cases become `m.sidebar.MoveDown(); return m.resizeFocused()` and `m.sidebar.MoveUp(); return m.resizeFocused()`. In `restartFocused`, `term, err := m.start(sess, m.ptySize())` and the return becomes `return term.Init()` (born at the right size; the Resize race is gone). In `addSession`, `term, err := m.start(sess, m.ptySize())`. In `onResize`, keep the guard from E3 (below) and use `m.ptySize()`.

- [ ] **Step 5: Run, commit**

Run: `go test ./internal/ui/ -race`

```bash
git add internal/ui/model.go internal/ui/run.go internal/ui/model_test.go internal/ui/refresh_test.go internal/ui/restart_test.go internal/ui/run_test.go
git commit -m "fix(#73): birth and resize every terminal at the live pane size

StartFunc takes the size, so a session created or restarted after a window
resize is born right; j/k resizes the terminal it lands on. Closes the class
behind #33, #34 and #51 rather than one instance.

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task E3: Move the size query behind termwrap; ignore a 0x0 window (#74)

**Files:**
- Create: `internal/termwrap/size.go`, `internal/termwrap/size_test.go`
- Modify: `cmd/omatty/main.go` (`windowSize`, imports, constants), `internal/ui/model.go` (`onResize`)
- Test: `internal/ui/layout_test.go`

**Interfaces:**
- Produces: `termwrap.WindowSize(f *os.File) (w, h int, err error)`.

- [ ] **Step 1: Write the failing tests**

`internal/termwrap/size_test.go`:

```go
package termwrap_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/creack/pty"

	"github.com/WilsonSousajr/omatty/internal/termwrap"
)

func TestWindowSize_ReturnsColumnsThenRows_issue74(t *testing.T) {
	ptmx, tty, err := pty.Open()
	if err != nil {
		t.Skip("no pty available:", err)
	}
	defer func() { _ = ptmx.Close(); _ = tty.Close() }()
	if err := pty.Setsize(ptmx, &pty.Winsize{Rows: 40, Cols: 140}); err != nil {
		t.Fatal(err)
	}

	w, h, err := termwrap.WindowSize(tty)

	if err != nil || w != 140 || h != 40 {
		t.Errorf("WindowSize = (%d, %d, %v), want (140, 40, nil): columns first", w, h, err)
	}
}

// Off a tty the caller must get an error to log, not a silent default.
func TestWindowSize_ErrorsOffATTY_issue74(t *testing.T) {
	f, err := os.Create(filepath.Join(t.TempDir(), "not-a-tty"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	if _, _, err := termwrap.WindowSize(f); err == nil {
		t.Error("WindowSize on a regular file returned nil error")
	}
}
```

Append to `internal/ui/layout_test.go`:

```go
// Regression, issue #74: off a tty bubbletea reports a 0x0 window, which
// clobbered the 80x24 default and floored every pane to 20x4.
func TestModel_IgnoresAZeroWindowSize_issue74(t *testing.T) {
	m, fakes := modelWithFakes(t)

	m.Update(tea.WindowSizeMsg{Width: 0, Height: 0})

	if fakes["s1"].Width != 0 {
		t.Errorf("a 0x0 window resized the terminal to %dx%d; it must be ignored", fakes["s1"].Width, fakes["s1"].Height)
	}
}
```

- [ ] **Step 2: Run and read the failure**

Run: `go test ./internal/termwrap/ ./internal/ui/ -run issue74 -v`
Expected: `termwrap.WindowSize` undefined; the ui test FAILS with `resized the terminal to 20x3`.

- [ ] **Step 3: Implement**

`internal/termwrap/size.go`:

```go
package termwrap

import (
	"fmt"
	"os"

	"github.com/creack/pty"
)

// WindowSize returns the terminal attached to f in columns and rows, so the
// first PTY is born at the real pane size (issue #51). It lives here rather
// than in cmd so the query sits behind omatty's own terminal seam and inside
// the coverage gate (issue #74). Off a tty it returns an error the caller
// logs before falling back to a default.
//
//	w, h, err := termwrap.WindowSize(os.Stdout)
func WindowSize(f *os.File) (w, h int, err error) {
	rows, cols, err := pty.Getsize(f)
	if err != nil {
		return 0, 0, fmt.Errorf("termwrap: querying the size of %q: %w", f.Name(), err)
	}
	if cols == 0 || rows == 0 {
		return 0, 0, fmt.Errorf("termwrap: %q reports a %dx%d terminal, want both non-zero", f.Name(), cols, rows)
	}
	return cols, rows, nil
}
```

`cmd/omatty/main.go`: delete the `defaultWidth`/`defaultHeight` constants and the `pty` import; replace `windowSize`:

```go
// windowSize is the real terminal size, so sessions are born at the right
// width (issue #51). Off a tty there is nothing to measure; the default is
// logged and used, and onResize ignores the 0x0 bubbletea then reports
// (issue #74).
func windowSize() (int, int) {
	w, h, err := termwrap.WindowSize(os.Stdout)
	if err != nil {
		slog.Warn("terminal size unavailable; sessions start at the default",
			"err", err, "width", ui.DefaultWidth, "height", ui.DefaultHeight)
		return ui.DefaultWidth, ui.DefaultHeight
	}
	return w, h
}
```

`internal/ui/model.go` `onResize`:

```go
// onResize gives the terminal pane whatever the sidebar and diff pane leave.
// Off a tty bubbletea reports 0x0, which would floor every pane; the default
// stands instead (issue #74).
func (m *Model) onResize(msg tea.WindowSizeMsg) tea.Cmd {
	if msg.Width == 0 || msg.Height == 0 {
		return nil
	}
	m.width, m.height = msg.Width, msg.Height
	return m.resizeFocused()
}
```

Run `go mod tidy` and confirm `go.mod` is unchanged (`creack/pty` stays a direct dependency because termwrap imports it).

- [ ] **Step 4: Run, commit**

Run: `go build ./... && go test ./... -race`

```bash
git add internal/termwrap/size.go internal/termwrap/size_test.go cmd/omatty/main.go internal/ui/model.go internal/ui/layout_test.go
git commit -m "refactor(#74): query the window size behind termwrap and ignore a 0x0 window

Co-Authored-By: Claude Fable 5.1 <noreply@anthropic.com>
Claude-Session: https://claude.ai/code/session_01XnFMVti99GDmBvmjBegaA5"
```

### Task E4: Gate, push, correct the PR body

- [ ] Full gate.
- [ ] `git push` (the branch already tracks origin).
- [ ] Rewrite the PR body with `gh pr edit 52 --body "$(cat <<'EOF' … EOF)"`: keep the symptom; replace the root cause with: *bubbleterm does forward the resize (`pty.Setsize`, and the child owns the tty via `Setsid`/`Setctty`, so SIGWINCH is delivered). The race is narrower: claude reads its column count at boot and commits its banner before it installs a resize handler, so a resize inside that window is lost. Birthing at the right size removes the window at startup; #73 removes it on restart, on new sessions, and on focus change; #74 moves the size query behind termwrap; #75 reconciles the PTY height with the rendered rows.* Add `Closes #51, #73, #74, #75.` and the verification list.
- [ ] Move #73–#75 to Review.

---

## Branch F — `refactor/m2-structure` (blocked until A–E merge)

Branch from `origin/develop` once it contains A–E. Re-read the files first; the code below assumes A–E landed as written above.

### Task F1: Split `model.go`; construct the model from a `Deps` struct (#76)

**Files:**
- Create: `internal/ui/status.go`, `internal/ui/render.go`
- Modify: `internal/ui/model.go`, `internal/ui/style.go`, `internal/ui/run.go`, every `ui_test` file that calls `NewModel` or the setters

**Interfaces:**
- Produces:

```go
// Deps is everything a Model needs. Constructor injection, so no field is
// optional and no method needs a nil guard (issue #76).
//
//	m := ui.NewModel(ui.Deps{State: st, Terms: terms, Create: create, Start: start,
//	        Events: events, Clock: time.Now, Notifier: notify.New(), TailStart: tail})
type Deps struct {
	State     registry.State
	Terms     map[string]termwrap.Terminal
	Create    CreateFunc
	Start     StartFunc
	Events    <-chan watcher.Event // nil in tests that never send status
	Clock     func() time.Time    // nil means time.Now
	Notifier  notify.Notifier     // nil means notify.Silent{}
	TailStart func(registry.Session) // nil means no tailer for runtime sessions
}

func NewModel(d Deps) *Model
```

- [ ] Move to `status.go`: `StatusMsg`, `TickMsg`, `tickEvery`, `scheduleTick`, `waitForEvent`, `onStatus`, `knownSession`, `notifyCooldown`, `maybeNotify`, `cooldownElapsed`, `notifyCmd`, `sessionTitle`, `needsYou`, `statusMap`.
- [ ] Move to `render.go`: `footer`, `emptyStateHint`, `promptLine`, `View`, `renderSidebar`, `renderTerminal`, `terminalTitle`, `renderFooter`, `renderRow`, `fitBlock`, `fitLine`, `padRight`.
- [ ] Move `statusGlyph` to `style.go` beside `statusColors`, as a map:

```go
// statusGlyphs pairs each status with its one-column marker; a status not
// listed renders "-".
var statusGlyphs = map[registry.Status]string{
	registry.StatusThinking: "*", registry.StatusTool: "@", registry.StatusWaiting: "!",
	registry.StatusDone: "+", registry.StatusError: "x", registry.StatusExited: "∅",
}

func statusGlyph(s registry.Status) string {
	if g, ok := statusGlyphs[s]; ok {
		return g
	}
	return "-"
}
```

- [ ] Replace `NewModel`'s four parameters with `Deps`; fill defaults (`Clock` → `time.Now`, `Notifier` → `notify.Silent{}`); set `startedAt: clock()`. Delete `SetEvents`, `SetNotifier`, `SetTailStarter`, and `WireStatusForTest`. `wireStatus` builds `Deps` directly. `TestWireStatus_StartsATailerPerSessionAndClosesThemAll_issue19` moves into `run_internal_test.go` (package `ui`) and asserts `len(tailers) == 3` by calling `wireStatus` and reading the slice the closer captures; give `wireStatus` a `(*Model, []*watcher.Tailer, func())` return for that.
- [ ] Update every test constructor: `ui.NewModel(ui.Deps{State: twoProjectState(), Terms: terms, Create: noCreate, Start: noStart})` and so on; `modelWithEvents` passes `Events: events, Clock: fixedClock`; `modelWithNotifier` passes `Notifier: n` and a movable clock.
- [ ] Gate; `wc -l internal/ui/*.go` must show every file under 500. Commit `refactor(#76): split model.go and construct the model from Deps`.

### Task F2: A `watcher.Watch` façade; `Status` moves to `watcher` (#77)

**Files:**
- Create: `internal/watcher/watch.go`, `internal/watcher/watch_test.go`, `internal/watcher/status.go`
- Modify: `internal/registry/state.go` (delete `Status` and its constants and `String`), `internal/ui/run.go`, every file using `registry.Status`

**Interfaces:**

```go
// Watch owns the status subsystem's goroutines: one hook listener and one
// tailer per session, feeding one channel. ui.Run holds a Watch; it no
// longer knows the socket path, the transcript path, the poll interval, or
// the buffer size (issue #77).
//
//	w := watcher.Start(home, st.Sessions, time.Now)
//	defer w.Close()
//	model := ui.NewModel(ui.Deps{Events: w.Events(), TailStart: w.Add, …})
type Watch struct {
	home    string
	clock   func() time.Time
	events  chan Event
	closeLn func()
	mu      sync.Mutex
	tailers []*Tailer
}

func Start(home string, sessions []registry.Session, clock func() time.Time) *Watch
func (w *Watch) Events() <-chan Event
func (w *Watch) Add(sess registry.Session)
func (w *Watch) Close()
```

`Start` moves `eventBuffer`, `startListener` (with the #49 degradation log), `tailStarter`, `StartTailers`, and `closeAll` out of `ui/run.go`. `pollEvery = time.Second` becomes a `watcher` constant with the WHY. `Status` and its constants move to `internal/watcher/status.go` unchanged in value; `sed -i '' 's/registry\.Status/watcher.Status/g'` over `internal/ui internal/watcher`, then fix imports. `registry` no longer declares it (its doc already says it is never persisted).

- [ ] Tests: `TestStart_OneTailerPerSessionAndAddGrowsIt_issue77`, `TestStart_CloseStopsEveryTailer_issue77` (assert each `Done()` closes), `TestStart_DegradesWhenTheSocketCannotBind_issue49` (over-long path; `Events()` still works from a tailer).
- [ ] `ui.Run` becomes: start terminals, `w := watcher.Start(...)`, `defer w.Close()`, build `Deps`, run.
- [ ] Gate. Commit `refactor(#77): give watcher a Watch façade and move Status out of registry`.

### Task F3: One owner for the hook event vocabulary; drop the dead field (#78)

**Files:**
- Modify: `internal/watcher/listener.go`, `internal/watcher/event.go`, `internal/hooks/settings.go`, `internal/hooks/settings_test.go`, `cmd/omatty/main.go`

**Interfaces:**

```go
// HookEventNames lists every hook event the listener maps to a Kind, plus
// Notification, whose kind depends on notification_type. hooks.Render takes
// this list so the two can never drift (issue #78).
func HookEventNames() []string

func Render(binPath string, events []string) ([]byte, error)
```

- [ ] `HookEventNames` returns the sorted keys of `kindByEvent` with `"Notification"` appended, then sorted for stable output. `hooks.statusEvents` is deleted; `settings_test.go` passes an explicit list and asserts every name it passes is registered. Add `TestHookEventNames_MatchesTheListenerMap_issue78` in `watcher`.
- [ ] Delete `Event.Tool` (grep confirms no reader) and the `Tool:` assignment in `serve`. Unify `recoverServe` into `recoverLoop` from `guard.go`.
- [ ] Gate. Commit `refactor(#78): source the hook event names from the listener's map`.

### Task F4: `supervisor.InstallHooks` (#79)

**Files:**
- Modify: `internal/supervisor/hooks.go`, `internal/supervisor/hooks_test.go`, `cmd/omatty/main.go`

**Interfaces:**

```go
// InstallHooks regenerates ~/.omatty/hooks.json for the running binary and
// returns its path. It runs before any session starts: claude refuses
// --settings on a missing file (issue #31) and the binary path moves with
// `go install`. It was four steps of logic in cmd (invariant 10, issue #79).
//
//	hooksFile, err := supervisor.InstallHooks(home, watcher.HookEventNames())
func InstallHooks(home string, events []string) (string, error) {
	bin, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("supervisor: locating the omatty binary: %w", err)
	}
	content, err := hooks.Render(bin, events)
	if err != nil {
		return "", fmt.Errorf("supervisor: rendering hooks for %q: %w", bin, err)
	}
	path := paths.HooksFile(home)
	if err := WriteHooksFile(path, content); err != nil {
		return "", err
	}
	return path, nil
}
```

- [ ] Test `TestInstallHooks_WritesTheRunningBinaryPath_issue79`: `home := t.TempDir()`; the file exists at `paths.HooksFile(home)` and contains `os.Executable()` of the test binary, shell-quoted. `runTUI` shrinks to `hooksFile, err := supervisor.InstallHooks(home, watcher.HookEventNames())`.
- [ ] Gate. Commit `refactor(#79): move the hooks bootstrap out of cmd`.

### Task F5: Tighten the two vacuous tests and the glyph mirror (#80)

- [ ] `TestWireStatus…` already counts tailers after F1/F2 (fold into F2 if easier).
- [ ] `TestReport_OversizedStdinIsBounded_issue18` was tightened in A2.
- [ ] In `status_test.go`, delete `statusGlyphFor` and assert on the literal glyph next to the session title on its row (e.g. the line containing `main` contains `!`), so the test stops mirroring production.
- [ ] Replace the two 300 ms negative waits (`listener_test.go` oversized case, `integration_test.go` drive loop) with `Dropped()`/`Close()`-based assertions where the listener is involved; leave `drive` as is and note it in the PR.
- [ ] Gate. Commit `test(#80): assert behaviour, not the fake, in the M2 tests`. Open the PR titled `refactor(#76): M2 structure` with `Closes #76, #77, #78, #79, #80.`

---

## Self-review

- Spec coverage: #54–#75 each have a task with a failing test; #76–#80 are specified in F with target signatures and blocked on the merge. #57 has no separate failing test because after #55 the forwarded line is a few hundred bytes and cannot block; the deadline is landed and documented in A2's commit.
- Type consistency: `StartFunc(sess, w, h)` is used identically in E2's model, run.go, and every test fake; `PTYSize` is the only size helper the PTY sees after E1; `ParsePayload` is the exported name used by A2's tests and `Report`; `Done()` and `recoverLoop` are defined in B5 and reused by F3; `Deps` fields in F1 match `wireStatus`'s needs in F2.
- Placeholders: none. Every code step is complete.
