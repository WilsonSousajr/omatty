# Review scoped to the last turn — implementation plan (#311)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `t` in the review column switches the diff between everything the session changed and only what changed since the current turn began.

**Architecture:** When the `UserPromptSubmit` hook fires, omatty writes the session's working tree as a git tree object through a temporary index and points `refs/omatty/turn/<session-id>` at it. The turn scope diffs a freshly built tree against that ref. `internal/vcs` does the git work, `internal/review` composes it into `SnapTurn`/`LoadTurn`/`DropTurn`, and `internal/ui` reaches those through an injected `TurnFuncs`.

**Tech Stack:** Go 1.26.8, the git CLI, bubbletea v2.

**Spec:** `docs/superpowers/specs/2026-09-23-review-since-last-turn-design.md`

## Global Constraints

- Invariant 4: git is called only from `internal/vcs`; `internal/ui` gets it through injected functions.
- Invariant 9: no `state.json` field is added; the ref name is `refs/omatty/turn/<row id>`.
- Invariant 11: nothing on the hook path waits for the snapshot; it runs as a `tea.Cmd`.
- Only a hook event may start a snapshot: `Event.Hook` is set by the listener and never by the tailer.
- `PruneSent`, `Compose` and the file tree's markers keep the **full** diff; everything that draws or indexes diff rows uses `shownDiff()`.
- Every test that exists for this feature carries `_issue311` in its name (AGENTS.md).
- Commits are `type(#311): message` and end with the two attribution lines.
- The gate after every task: `gofmt`, `go vet ./...`, `go mod tidy -diff`, `golangci-lint run`, `./scripts/check-deps.sh`, `govulncheck ./...`, `go test ./... -race`, `./scripts/check-coverage.sh 90`, `./scripts/check-crap.sh 12`, `go build ./...`.

**Spec correction.** The spec's §3 says `omatty rm` deletes refs. It does not need to: `registry.RemoveProject` refuses a project that still holds sessions (#159), so archive is the only path by which a session with a ref goes away. Nothing is built for `rm`.

## Review Focus

1. **A snapshot finishes after its session was archived.** Expect: nothing is stored for it; the per-session maps do not grow. Pinned in Task 4.
2. **A turn diff arrives after the operator moved the column to another session, or toggled back to the whole session.** Expect: it is dropped, the view is unchanged. Pinned in Task 5.
3. **A comment written in the turn scope, viewed in the whole-session scope.** Expect: it sits on the same line, not `(moved)`. Pinned in Task 5.
4. **`r` in the turn scope.** Expect: it reloads the turn diff, not only the full one. Pinned in Task 5.
5. **Switching to the turn scope before the turn diff has loaded.** Expect: `reading this turn...`, never a false `no changes`. Pinned in Task 5.

---

### Task 0: Branch

- [ ] **Step 1:** The spec and this plan are on `docs/311-turn-scope-spec`. Rename it for the build and push:

```bash
git switch docs/311-turn-scope-spec
git branch -m feat/311-turn-scope
git push -q -u origin feat/311-turn-scope
git push -q origin --delete docs/311-turn-scope-spec
```

Move #311 to In Progress on project 13 (`gh project item-add` then `item-edit` with option `986dbe85`).

---

### Task 1: `vcs` — snapshot trees, turn refs, tree diffs

**Files:**
- Modify: `internal/vcs/git.go` (the `Git` interface; `capture` gains an env-taking sibling)
- Create: `internal/vcs/turn.go`
- Create: `internal/vcs/turn_test.go`
- Modify: `internal/review/fakes_test.go` (FakeGit must keep satisfying `vcs.Git`)

**Interfaces:**
- Produces, on `vcs.Git` and `*vcs.CLI`:
  - `SnapshotTree(dir string) (string, error)`
  - `SetTurnRef(dir, id, tree string) error`
  - `TurnRef(dir, id string) (tree string, ok bool, err error)`
  - `DeleteTurnRef(dir, id string) error`
  - `DiffTrees(dir, from, to string) (string, error)`

- [ ] **Step 1: Write the failing tests** — `internal/vcs/turn_test.go`:

```go
package vcs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func treeFiles(t *testing.T, dir, tree string) string {
	t.Helper()
	return gitOut(t, dir, "ls-tree", "-r", "--name-only", tree)
}

// A turn baseline is the working tree as the agent is about to change it:
// the files it created count, the ones .gitignore excludes do not (#311).
func TestCLI_SnapshotTreeTakesUntrackedAndSkipsIgnored_issue311(t *testing.T) {
	dir := newRepo(t)
	write(t, filepath.Join(dir, ".gitignore"), "*.log\n")
	write(t, filepath.Join(dir, "new.txt"), "fresh\n")
	write(t, filepath.Join(dir, "noise.log"), "ignored\n")

	tree, err := vcs.NewCLI().SnapshotTree(dir)
	if err != nil {
		t.Fatalf("SnapshotTree() error = %v", err)
	}

	files := treeFiles(t, dir, tree)
	if !strings.Contains(files, "new.txt") || !strings.Contains(files, ".gitignore") {
		t.Errorf("the snapshot misses an untracked file:\n%s", files)
	}
	if strings.Contains(files, "noise.log") {
		t.Errorf("the snapshot took an ignored file:\n%s", files)
	}
}

// The snapshot goes through a copy of the index. The operator's staging,
// HEAD and stash are theirs, and a baseline that moved any of them would be
// omatty editing the repository behind their back.
func TestCLI_SnapshotTreeLeavesIndexHeadAndStashAlone_issue311(t *testing.T) {
	dir := newRepo(t)
	write(t, filepath.Join(dir, "staged.txt"), "one\n")
	gitOut(t, dir, "add", "staged.txt")
	write(t, filepath.Join(dir, "loose.txt"), "two\n")
	index := filepath.Join(dir, ".git", "index")
	before, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	head := gitOut(t, dir, "rev-parse", "HEAD")
	status := gitOut(t, dir, "status", "--porcelain")

	if _, err := vcs.NewCLI().SnapshotTree(dir); err != nil {
		t.Fatalf("SnapshotTree() error = %v", err)
	}

	after, err := os.ReadFile(index)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Error("the real index changed")
	}
	if got := gitOut(t, dir, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD moved from %s to %s", head, got)
	}
	if got := gitOut(t, dir, "status", "--porcelain"); got != status {
		t.Errorf("status changed:\nbefore %q\nafter  %q", status, got)
	}
	if got := gitOut(t, dir, "stash", "list"); strings.TrimSpace(got) != "" {
		t.Errorf("the stash is not empty: %q", got)
	}
}

// A linked worktree has its own index, and refs live in the common
// repository: a worktree session's baseline must read back from anywhere.
func TestCLI_SnapshotTreeInALinkedWorktree_issue311(t *testing.T) {
	repo := newRepo(t)
	wt := filepath.Join(t.TempDir(), "wt")
	git := vcs.NewCLI()
	if err := git.AddWorktree(repo, wt, "feature", "main"); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(wt, "only-here.txt"), "wt\n")

	tree, err := git.SnapshotTree(wt)
	if err != nil {
		t.Fatalf("SnapshotTree() error = %v", err)
	}
	if err := git.SetTurnRef(wt, "s1", tree); err != nil {
		t.Fatalf("SetTurnRef() error = %v", err)
	}

	got, ok, err := git.TurnRef(repo, "s1")
	if err != nil || !ok || got != tree {
		t.Errorf("TurnRef from the main checkout = %q, %v, %v; want %q, true, nil", got, ok, err, tree)
	}
	if !strings.Contains(treeFiles(t, repo, tree), "only-here.txt") {
		t.Error("the worktree's snapshot does not hold the worktree's file")
	}
}

// A repository nobody has committed to has no index file at all.
func TestCLI_SnapshotTreeInARepoWithNoCommits_issue311(t *testing.T) {
	dir := t.TempDir()
	gitOut(t, dir, "init", "-b", "main")
	write(t, filepath.Join(dir, "a.txt"), "a\n")

	tree, err := vcs.NewCLI().SnapshotTree(dir)
	if err != nil {
		t.Fatalf("SnapshotTree() error = %v", err)
	}
	if !strings.Contains(treeFiles(t, dir, tree), "a.txt") {
		t.Error("the snapshot of an empty repository misses its file")
	}
}

func TestCLI_DiffTreesReportsAddedModifiedAndDeleted_issue311(t *testing.T) {
	dir := newRepo(t)
	write(t, filepath.Join(dir, "keep.txt"), "old\n")
	write(t, filepath.Join(dir, "gone.txt"), "bye\n")
	git := vcs.NewCLI()
	base, err := git.SnapshotTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "keep.txt"), "changed\n")
	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "new.txt"), "hello\n")
	now, err := git.SnapshotTree(dir)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := git.DiffTrees(dir, base, now)
	if err != nil {
		t.Fatalf("DiffTrees() error = %v", err)
	}
	for _, want := range []string{"+changed", "deleted file mode", "diff --git a/new.txt b/new.txt", "+hello"} {
		if !strings.Contains(raw, want) {
			t.Errorf("the diff lacks %q:\n%s", want, raw)
		}
	}
}

func TestCLI_TurnRefRoundTrips_issue311(t *testing.T) {
	dir := newRepo(t)
	git := vcs.NewCLI()
	if _, ok, err := git.TurnRef(dir, "s1"); ok || err != nil {
		t.Fatalf("TurnRef before any = ok %v, err %v; want false, nil", ok, err)
	}
	tree, err := git.SnapshotTree(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := git.SetTurnRef(dir, "s1", tree); err != nil {
		t.Fatalf("SetTurnRef() error = %v", err)
	}
	if got, ok, err := git.TurnRef(dir, "s1"); got != tree || !ok || err != nil {
		t.Errorf("TurnRef = %q, %v, %v; want %q, true, nil", got, ok, err, tree)
	}
	if err := git.DeleteTurnRef(dir, "s1"); err != nil {
		t.Fatalf("DeleteTurnRef() error = %v", err)
	}
	if _, ok, _ := git.TurnRef(dir, "s1"); ok {
		t.Error("the ref survived its delete")
	}
	if err := git.DeleteTurnRef(dir, "s1"); err != nil {
		t.Errorf("deleting a missing ref = %v, want nil: archive must not fail on it", err)
	}
}
```

- [ ] **Step 2: Run them to see them fail**

Run: `go test ./internal/vcs -run issue311`
Expected: build failure, `git.SnapshotTree undefined` (and the other four).

- [ ] **Step 3: Implement.** In `internal/vcs/git.go`, add the five methods to the `Git` interface after `ListFiles`:

```go
	// SnapshotTree writes dir's working tree - tracked and untracked files,
	// honouring .gitignore - as a tree object, without touching HEAD, the
	// index or the stash (#311).
	SnapshotTree(dir string) (string, error)
	// SetTurnRef, TurnRef and DeleteTurnRef keep one session's turn baseline
	// under refs/omatty/turn/<id> (#311).
	SetTurnRef(dir, id, tree string) error
	TurnRef(dir, id string) (string, bool, error)
	DeleteTurnRef(dir, id string) error
	// DiffTrees is the unified diff between two trees (#311).
	DiffTrees(dir, from, to string) (string, error)
```

Split `capture` so an env can be passed; `capture` keeps its signature and every caller:

```go
func (c *CLI) capture(dir string, okExit int, args ...string) (string, error) {
	return c.captureEnv(dir, okExit, nil, args...)
}

// captureEnv is capture with extra environment entries, for SnapshotTree,
// which points git at a temporary index (#311).
func (c *CLI) captureEnv(dir string, okExit int, env []string, args ...string) (string, error) {
	if err := checkDir(dir); err != nil {
		return "", err
	}
	cmd := exec.Command(c.bin, args...)
	cmd.Dir = dir
	if env != nil {
		cmd.Env = append(os.Environ(), env...)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil && !exitedWith(err, okExit) {
		return "", fmt.Errorf("vcs: `git %s` in %q failed: %s: %w",
			strings.Join(args, " "), dir, strings.TrimSpace(stderr.String()), err)
	}
	return string(out), nil
}
```

Create `internal/vcs/turn.go`:

```go
package vcs

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// turnRef names a session's turn baseline. Refs live in the common
// repository, so a worktree session's baseline is visible from the main
// checkout and outlives omatty (#311). The id is the session's row id, which
// never changes, so state.json needs no field (invariant 9).
func turnRef(id string) string { return "refs/omatty/turn/" + id }

// SnapshotTree writes dir's working tree as a tree object and returns its id.
// It runs `add -A` and `write-tree` against a temporary copy of the index, so
// the operator's staging, HEAD and stash are never touched and no git
// identity is needed, since no commit is made (#311).
//
//	tree, err := vcs.NewCLI().SnapshotTree("/wt/parser-fix")
func (c *CLI) SnapshotTree(dir string) (string, error) {
	tmp, err := os.MkdirTemp("", "omatty-index-")
	if err != nil {
		return "", fmt.Errorf("vcs: temporary index for %q: %w", dir, err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	index := filepath.Join(tmp, "index")
	if err := c.seedIndex(dir, index); err != nil {
		return "", err
	}
	env := []string{"GIT_INDEX_FILE=" + index}
	if _, err := c.captureEnv(dir, 0, env, "add", "-A"); err != nil {
		return "", err
	}
	out, err := c.captureEnv(dir, 0, env, "write-tree")
	return strings.TrimSpace(out), err
}

// seedIndex copies dir's own index to path, so `add -A` reuses git's stat
// cache instead of hashing every file. `--git-path` answers relative to dir
// in a main checkout and absolutely in a linked worktree. A repository with
// no index yet leaves path absent, and git starts from empty.
func (c *CLI) seedIndex(dir, path string) error {
	src, err := c.run(dir, "rev-parse", "--git-path", "index")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(src) {
		src = filepath.Join(dir, src)
	}
	body, err := os.ReadFile(src)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("vcs: reading the index of %q: %w", dir, err)
	}
	return os.WriteFile(path, body, 0o600)
}

// SetTurnRef points id's turn baseline at tree.
func (c *CLI) SetTurnRef(dir, id, tree string) error {
	_, err := c.run(dir, "update-ref", turnRef(id), tree)
	return err
}

// TurnRef is the tree id's baseline names; false when there is none, which
// rev-parse --verify --quiet reports as exit 1 with no output.
func (c *CLI) TurnRef(dir, id string) (string, bool, error) {
	out, err := c.capture(dir, 1, "rev-parse", "--verify", "--quiet", turnRef(id))
	tree := strings.TrimSpace(out)
	return tree, err == nil && tree != "", err
}

// DeleteTurnRef removes id's baseline. git succeeds on a ref that is not
// there, which archive relies on.
func (c *CLI) DeleteTurnRef(dir, id string) error {
	_, err := c.run(dir, "update-ref", "-d", turnRef(id))
	return err
}

// DiffTrees is the unified diff from one tree to another, with the flags Diff
// uses. Both sides being trees built with `add -A`, a new file arrives as an
// addition and there is no untracked pass to make.
func (c *CLI) DiffTrees(dir, from, to string) (string, error) {
	return c.capture(dir, 0, diffArgs(from, to, "--")...)
}
```

In `internal/review/fakes_test.go`, add fields to `FakeGit` after `Files`:

```go
	SnapshotOut  string // SnapshotTree result (#311)
	TurnTree     string // TurnRef's tree; empty means no baseline
	DiffTreesOut string // DiffTrees result
```

and the methods:

```go
func (f *FakeGit) SnapshotTree(dir string) (string, error) {
	return f.SnapshotOut, f.record("SnapshotTree", dir)
}

func (f *FakeGit) SetTurnRef(dir, id, tree string) error {
	return f.record("SetTurnRef", dir, id, tree)
}

func (f *FakeGit) TurnRef(dir, id string) (string, bool, error) {
	return f.TurnTree, f.TurnTree != "", f.record("TurnRef", dir, id)
}

func (f *FakeGit) DeleteTurnRef(dir, id string) error {
	return f.record("DeleteTurnRef", dir, id)
}

func (f *FakeGit) DiffTrees(dir, from, to string) (string, error) {
	return f.DiffTreesOut, f.record("DiffTrees", dir, from, to)
}
```

- [ ] **Step 4: Run them to see them pass**

Run: `go test ./internal/vcs ./internal/review`
Expected: `ok` for both.

- [ ] **Step 5: Commit**

```bash
git add internal/vcs/git.go internal/vcs/turn.go internal/vcs/turn_test.go internal/review/fakes_test.go
git commit -m "feat(#311): vcs snapshots a working tree and keeps a turn ref"
```

---

### Task 2: `watcher` — mark hook events

**Files:**
- Modify: `internal/watcher/event.go` (`Event.Hook`)
- Modify: `internal/watcher/listener.go:218`
- Test: `internal/watcher/listener_test.go`, `internal/watcher/tailer_test.go`

**Interfaces:**
- Produces: `watcher.Event.Hook bool` — true only on events from the socket listener.

- [ ] **Step 1: Write the failing tests.** Append to `internal/watcher/listener_test.go`:

```go
// Only a hook knows a prompt was just submitted; the tailer reports
// PromptSubmitted for tool results too. The UI snapshots a turn baseline on
// the first and must never on the second (#311).
func TestListen_marksItsEventsAsFromAHook_issue311(t *testing.T) {
	path := filepath.Join(shortDir(t), "s")
	sink := make(chan watcher.Event, 1)
	l, err := watcher.Listen(path, sink, time.Now, watcher.ClaudeAdapter())
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer func() { _ = l.Close() }()

	dial(t, path, `{"session_id":"abc","hook_event_name":"UserPromptSubmit"}`)

	select {
	case ev := <-sink:
		if !ev.Hook || ev.Kind != watcher.PromptSubmitted {
			t.Errorf("event = %+v, want a PromptSubmitted marked Hook", ev)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no event emitted")
	}
}
```

Append to `internal/watcher/tailer_test.go`:

```go
func TestTailer_neverMarksAnEventAsFromAHook_issue311(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	sink := make(chan watcher.Event, 8)
	tl := watcher.Tail("s1", path, sink, time.Now, time.Hour, watcher.ClaudeAdapter())
	defer tl.Close()
	if err := os.WriteFile(path, []byte(promptLine), 0o600); err != nil {
		t.Fatal(err)
	}
	tl.Poll()

	for _, ev := range drain(sink) {
		if ev.Hook {
			t.Errorf("the tailer marked %+v as a hook event", ev)
		}
	}
}
```

- [ ] **Step 2: Run to see the failure**

Run: `go test ./internal/watcher -run issue311`
Expected: build failure, `ev.Hook undefined`.

- [ ] **Step 3: Implement.** In `internal/watcher/event.go`, add to `Event` after `Owner`:

```go
	// Hook is true on an event the socket listener produced, false on the
	// tailer's. The tailer's PromptSubmitted also fires on tool results, so a
	// consumer that needs "a prompt was just submitted" must ask this (#311).
	Hook bool
```

In `internal/watcher/listener.go:218`:

```go
	return Event{SessionID: p.SessionID, Kind: kind, At: l.clock(), Owner: p.OmattySession, Hook: true}, true
```

- [ ] **Step 4: Run to see them pass**

Run: `go test ./internal/watcher`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/watcher
git commit -m "feat(#311): a hook event says it came from a hook"
```

---

### Task 3: `review` — SnapTurn, LoadTurn, DropTurn

**Files:**
- Create: `internal/review/turn.go`
- Create: `internal/review/turn_test.go`

**Interfaces:**
- Consumes: Task 1's `vcs.Git` methods.
- Produces:
  - `var review.ErrNoTurn error`
  - `func (s *Source) SnapTurn(sess registry.Session) error`
  - `func (s *Source) LoadTurn(sess registry.Session, projectRoot string) (Diff, error)` — a `ui.DiffFunc`
  - `func (s *Source) DropTurn(sess registry.Session, projectRoot string) error`

- [ ] **Step 1: Write the failing tests** — `internal/review/turn_test.go`:

```go
package review_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
)

var turnSess = registry.Session{ID: "s1", Dir: "/wt/s1"}

func TestSource_SnapTurnPointsTheSessionsRefAtItsTree_issue311(t *testing.T) {
	git := &FakeGit{SnapshotOut: "tree1"}

	if err := review.NewSource(git).SnapTurn(turnSess); err != nil {
		t.Fatalf("SnapTurn() error = %v", err)
	}

	want := "SnapshotTree(/wt/s1) SetTurnRef(/wt/s1,s1,tree1)"
	if got := strings.Join(git.Calls, " "); got != want {
		t.Errorf("calls = %s, want %s", got, want)
	}
}

func TestSource_SnapTurnFailureNamesTheSession_issue311(t *testing.T) {
	git := &FakeGit{Errs: map[string]error{"SnapshotTree": errors.New("disk full")}}

	err := review.NewSource(git).SnapTurn(turnSess)

	if err == nil || !strings.Contains(err.Error(), "s1") || !strings.Contains(err.Error(), "disk full") {
		t.Errorf("error = %v, want one naming s1 and the cause", err)
	}
}

// With no baseline there is no turn to show, and saying "no changes" would
// be a lie: LoadTurn says so with its own error, and builds nothing.
func TestSource_LoadTurnWithNoBaselineIsErrNoTurn_issue311(t *testing.T) {
	git := &FakeGit{}

	_, err := review.NewSource(git).LoadTurn(turnSess, "/p")

	if !errors.Is(err, review.ErrNoTurn) {
		t.Errorf("error = %v, want ErrNoTurn", err)
	}
	if strings.Contains(strings.Join(git.Calls, " "), "SnapshotTree") {
		t.Errorf("snapshotted with no baseline to diff against: %v", git.Calls)
	}
}

func TestSource_LoadTurnDiffsTheBaselineAgainstTheTreeNow_issue311(t *testing.T) {
	git := &FakeGit{TurnTree: "base", SnapshotOut: "now", DiffTreesOut: twoFileDiff}

	d, err := review.NewSource(git).LoadTurn(turnSess, "/p")

	if err != nil {
		t.Fatalf("LoadTurn() error = %v", err)
	}
	if !strings.Contains(strings.Join(git.Calls, " "), "DiffTrees(/wt/s1,base,now)") {
		t.Errorf("calls = %v, want DiffTrees(/wt/s1,base,now)", git.Calls)
	}
	if len(d.Files) != 2 {
		t.Errorf("parsed %d files, want 2", len(d.Files))
	}
}

// The worktree may already be gone when a session is archived; the ref is
// in the common repository, so it is deleted from the project root.
func TestSource_DropTurnDeletesFromTheProjectRoot_issue311(t *testing.T) {
	git := &FakeGit{}

	if err := review.NewSource(git).DropTurn(turnSess, "/p/omatty"); err != nil {
		t.Fatalf("DropTurn() error = %v", err)
	}
	if got := strings.Join(git.Calls, " "); got != "DeleteTurnRef(/p/omatty,s1)" {
		t.Errorf("calls = %s, want DeleteTurnRef(/p/omatty,s1)", got)
	}
}
```

- [ ] **Step 2: Run to see the failure**

Run: `go test ./internal/review -run issue311`
Expected: build failure, `review.NewSource(git).SnapTurn undefined`.

- [ ] **Step 3: Implement** — `internal/review/turn.go`:

```go
package review

import (
	"errors"
	"fmt"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/registry"
)

// ErrNoTurn is LoadTurn's answer for a session with no turn baseline: no
// prompt has been submitted since #311 shipped, or its hooks are not
// arriving (#49). The review column shows it as a notice, not a failure.
var ErrNoTurn = errors.New("review: no turn recorded yet")

// SnapTurn records sess's working tree as the baseline of the turn that is
// starting - what LoadTurn diffs against until the next prompt (#311).
//
//	err := src.SnapTurn(sess)
func (s *Source) SnapTurn(sess registry.Session) error {
	tree, err := s.git.SnapshotTree(sess.Dir)
	if err != nil {
		return fmt.Errorf("review: snapshotting session %s in %q: %w", sess.ID, sess.Dir, err)
	}
	if err := s.git.SetTurnRef(sess.Dir, sess.ID, tree); err != nil {
		return fmt.Errorf("review: recording the turn baseline of session %s: %w", sess.ID, err)
	}
	return nil
}

// LoadTurn is what sess changed since its turn began: the working tree now
// against the baseline SnapTurn recorded. The unused projectRoot makes it a
// DiffFunc like Load, so the UI loads both the same way.
//
//	d, err := src.LoadTurn(sess, projectRoot)
func (s *Source) LoadTurn(sess registry.Session, _ string) (Diff, error) {
	base, ok, err := s.git.TurnRef(sess.Dir, sess.ID)
	if err != nil {
		return Diff{}, fmt.Errorf("review: reading the turn baseline of session %s: %w", sess.ID, err)
	}
	if !ok {
		return Diff{}, ErrNoTurn
	}
	now, err := s.git.SnapshotTree(sess.Dir)
	if err != nil {
		return Diff{}, fmt.Errorf("review: snapshotting session %s in %q: %w", sess.ID, sess.Dir, err)
	}
	raw, err := s.git.DiffTrees(sess.Dir, base, now)
	if err != nil {
		return Diff{}, fmt.Errorf("review: diffing session %s's turn: %w", sess.ID, err)
	}
	return ParseDiff(strings.NewReader(raw))
}

// DropTurn deletes sess's baseline when the session is archived. From
// projectRoot, because the worktree may be removed in the same breath and
// the ref lives in the common repository anyway.
func (s *Source) DropTurn(sess registry.Session, projectRoot string) error {
	if err := s.git.DeleteTurnRef(projectRoot, sess.ID); err != nil {
		return fmt.Errorf("review: deleting the turn baseline of session %s: %w", sess.ID, err)
	}
	return nil
}
```

- [ ] **Step 4: Run to see them pass**

Run: `go test ./internal/review`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/review/turn.go internal/review/turn_test.go
git commit -m "feat(#311): review records, loads and drops a session's turn"
```

---

### Task 4: `ui` — take the baseline on a hook prompt, drop it on archive

**Files:**
- Create: `internal/ui/turn.go`
- Create: `internal/ui/turn_test.go`
- Create: `internal/ui/turn_internal_test.go`
- Modify: `internal/ui/deps.go` (`TurnFuncs`, `Deps.Turn`, defaults)
- Modify: `internal/ui/run.go` (`RunDeps.Turn`, passed into `Deps`)
- Modify: `internal/ui/model.go` (fields, map init, `onDataMsg` case)
- Modify: `internal/ui/status.go` (`afterStatus` batches `maybeSnapTurn`)
- Modify: `internal/ui/archive.go` (`dropSession` batches `dropTurnCmd`; `forgetCardMaps` deletes the two maps)
- Modify: `internal/ui/forget_internal_test.go` (`filledModel` fills the two maps)
- Modify: `cmd/omatty/wiring.go` (`tuiDeps` wires the Source's three methods)

**Interfaces:**
- Consumes: `watcher.Event.Hook` (Task 2); `review.Source.SnapTurn/LoadTurn/DropTurn` (Task 3).
- Produces:
  - `type ui.TurnFuncs struct { Snap func(registry.Session) error; Diff DiffFunc; Drop func(registry.Session, string) error }`
  - `Deps.Turn`, `RunDeps.Turn`
  - `type ui.TurnSnappedMsg struct { SessionID string; Err error }`
  - model fields `turn TurnFuncs`, `turnPending map[string]bool`, `turnErr map[string]string`
  - `func (m *Model) onTurnSnapped(msg TurnSnappedMsg) tea.Cmd` — returns `nil` here; Task 5 makes it reload an open turn view.

- [ ] **Step 1: Write the failing tests** — `internal/ui/turn_test.go`:

```go
package ui_test

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// turnRecorder is a named fake for ui.TurnFuncs (#311).
type turnRecorder struct {
	Snapped   []string
	SnapErr   error
	Diff      review.Diff
	DiffErr   error
	DiffCalls int
	Dropped   [][2]string // id, projectRoot
}

func (r *turnRecorder) funcs() ui.TurnFuncs {
	return ui.TurnFuncs{
		Snap: func(s registry.Session) error {
			r.Snapped = append(r.Snapped, s.ID)
			return r.SnapErr
		},
		Diff: func(registry.Session, string) (review.Diff, error) {
			r.DiffCalls++
			return r.Diff, r.DiffErr
		},
		Drop: func(s registry.Session, root string) error {
			r.Dropped = append(r.Dropped, [2]string{s.ID, root})
			return nil
		},
	}
}

func hookPrompt(id string) ui.StatusMsg {
	return ui.StatusMsg{SessionID: id, Kind: watcher.PromptSubmitted, At: time.Now(), Hook: true}
}

func modelWithTurn(t *testing.T, tr *turnRecorder) *ui.Model {
	t.Helper()
	terms, _ := fakeTerms(t)
	d := baseDeps(twoProjectState(), terms)
	d.Turn = tr.funcs()
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return m
}

func TestModel_aHookPromptSnapsTheTurnBaseline_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m := modelWithTurn(t, tr)

	_, cmd := m.Update(hookPrompt("s1"))
	settle(m, cmd)

	if len(tr.Snapped) != 1 || tr.Snapped[0] != "s1" {
		t.Errorf("snapped = %v, want [s1]", tr.Snapped)
	}
}

// The tailer reports PromptSubmitted for every tool result; a snapshot on
// one would move the baseline mid-turn and drop the turn's earlier edits.
func TestModel_aTailerPromptDoesNotSnap_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m := modelWithTurn(t, tr)
	msg := hookPrompt("s1")
	msg.Hook = false

	_, cmd := m.Update(msg)
	settle(m, cmd)

	if len(tr.Snapped) != 0 {
		t.Errorf("snapped %v on a tailer event", tr.Snapped)
	}
}

// Two prompts inside one snapshot: the second is dropped, so the diff shows
// more than one turn rather than less.
func TestModel_aPromptWhileASnapIsInFlightStartsNothing_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m := modelWithTurn(t, tr)

	_, first := m.Update(hookPrompt("s1"))
	_, second := m.Update(hookPrompt("s1"))
	settle(m, first)
	settle(m, second)

	if len(tr.Snapped) != 1 {
		t.Errorf("snapped %d times, want 1", len(tr.Snapped))
	}
}

func TestModel_archiveDropsTheTurnBaseline_issue311(t *testing.T) {
	tr := &turnRecorder{}
	r := &recordArchive{}
	terms, _ := fakeTerms(t)
	st := worktreeState()
	r.State = st
	d := baseDeps(st, terms)
	d.Archive, d.TailStop, d.RemoveWorktree = r.archive, r.stopTail, r.removeWorktree
	d.Turn = tr.funcs()
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	openArchive(t, m, "s1")

	pressAndSettle(m, key('y'))

	if len(tr.Dropped) != 1 || tr.Dropped[0] != [2]string{"s1", "/p/omatty"} {
		t.Errorf("dropped = %v, want [[s1 /p/omatty]]", tr.Dropped)
	}
}

var errDiskFull = errors.New("disk full")
```

`internal/ui/turn_internal_test.go` (package `ui`, for Review Focus 1):

```go
package ui

import (
	"errors"
	"testing"
)

// A snapshot that finishes after its session was archived must not bring
// the session's maps back: forgetSessionMaps has already run (#311).
func TestOnTurnSnapped_forAForgottenSessionStoresNothing_issue311(t *testing.T) {
	m := filledModel()
	m.forgetSession(forgottenID)

	m.onTurnSnapped(TurnSnappedMsg{SessionID: forgottenID, Err: errors.New("dir gone")})

	if _, ok := m.turnErr[forgottenID]; ok {
		t.Error("turnErr holds an archived session")
	}
	if _, ok := m.turnPending[forgottenID]; ok {
		t.Error("turnPending holds an archived session")
	}
}
```

In `internal/ui/forget_internal_test.go`'s `filledModel`, after the `gateSent` line:

```go
	m.turnPending[forgottenID] = true
	m.turnErr[forgottenID] = "disk full"
```

- [ ] **Step 2: Run to see the failure**

Run: `go test ./internal/ui -run 'issue311|PerSessionMap'`
Expected: build failure, `d.Turn undefined`, `ui.StatusMsg ... Hook` unknown field.

- [ ] **Step 3: Implement.** `internal/ui/deps.go`, after `RepoStatFunc`:

```go
// TurnFuncs are the three calls #311 makes on a session's turn baseline,
// injected because ui may not touch git (invariant 4). Snap records the
// baseline as a prompt is submitted, Diff loads what changed since it, and
// Drop deletes it when the session is archived.
type TurnFuncs struct {
	Snap func(sess registry.Session) error
	Diff DiffFunc
	Drop func(sess registry.Session, projectRoot string) error
}
```

`Deps`, after `Stat`:

```go
	// Turn reaches a session's turn baseline (#311). Unwired, Snap and Drop
	// do nothing and Diff says there is no turn - what every test sees.
	Turn TurnFuncs
```

`withReviewDefaults`, before `return d`:

```go
	if d.Turn.Snap == nil {
		d.Turn.Snap = func(registry.Session) error { return nil }
	}
	if d.Turn.Diff == nil {
		d.Turn.Diff = func(registry.Session, string) (review.Diff, error) { return review.Diff{}, review.ErrNoTurn }
	}
	if d.Turn.Drop == nil {
		d.Turn.Drop = func(registry.Session, string) error { return nil }
	}
```

`internal/ui/run.go`: add `Turn TurnFuncs` to `RunDeps` beside `Diff DiffFunc`, and `Turn: d.Turn,` to the `Deps` literal at line 190.

`internal/ui/model.go`: beside `gateSent`,

```go
	// turn reaches the turn baselines (#311). turnPending holds a session
	// whose snapshot is in flight, so a second prompt inside it starts none;
	// turnErr is the last snapshot's failure, shown instead of a turn diff
	// that would silently span two turns. Neither is persisted.
	turn        TurnFuncs
	turnPending map[string]bool
	turnErr     map[string]string
```

where `NewModel` copies deps: `turn: d.Turn,`; in the map init beside `m.gateSent`:

```go
	m.turnPending = map[string]bool{}
	m.turnErr = map[string]string{}
```

and in `onDataMsg`'s switch:

```go
	case TurnSnappedMsg:
		return m.onTurnSnapped(typed)
```

Create `internal/ui/turn.go`:

```go
// The turn baseline (#311): taken when a prompt is submitted, so the review
// column can show only what the current turn changed.

package ui

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/registry"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// TurnSnappedMsg carries a baseline snapshot's outcome into Update. Exported
// so tests can send one.
type TurnSnappedMsg struct {
	SessionID string
	Err       error
}

// maybeSnapTurn records the session's working tree as a turn begins. Only
// the hook counts: the tailer reports PromptSubmitted for every tool result,
// and a snapshot taken mid-turn would drop the turn's earlier edits from its
// own diff. A snapshot already in flight absorbs the prompt.
func (m *Model) maybeSnapTurn(e watcher.Event) tea.Cmd {
	if e.Kind != watcher.PromptSubmitted || !e.Hook || m.turnPending[e.SessionID] {
		return nil
	}
	sess, ok := m.session(e.SessionID)
	if !ok {
		return nil
	}
	m.turnPending[sess.ID] = true
	snap := m.turn.Snap
	return func() tea.Msg { return TurnSnappedMsg{SessionID: sess.ID, Err: snap(sess)} }
}

// onTurnSnapped settles a snapshot. A failure is kept until the next success:
// the previous turn's ref is still standing, and diffing against it would
// show two turns as one.
func (m *Model) onTurnSnapped(msg TurnSnappedMsg) tea.Cmd {
	delete(m.turnPending, msg.SessionID)
	if !m.knownSession(msg.SessionID) {
		return nil
	}
	if msg.Err != nil {
		slog.Warn("taking a turn baseline", "session", msg.SessionID, "err", msg.Err)
		m.turnErr[msg.SessionID] = msg.Err.Error()
		return nil
	}
	delete(m.turnErr, msg.SessionID)
	return nil
}

// dropTurnCmd deletes an archived session's baseline off the Update
// goroutine. A failure is logged and nothing more: the archive has happened,
// and a stray ref costs a few objects, not correctness.
func (m *Model) dropTurnCmd(sess registry.Session) tea.Cmd {
	root, drop := m.projectRoot(sess.Project), m.turn.Drop
	return func() tea.Msg {
		if err := drop(sess, root); err != nil {
			slog.Warn("deleting a turn baseline", "session", sess.ID, "err", err)
		}
		return nil
	}
}
```

`internal/ui/status.go`, `afterStatus`'s batch gains `m.maybeSnapTurn(e)`:

```go
	return tea.Batch(m.waitForEvent(), m.maybeNotify(e, before, after),
		m.refreshReview(e.SessionID, before, after), m.maybeName(e.SessionID),
		m.refreshStat(e.SessionID, before, after), m.maybeSnapTurn(e))
```

`internal/ui/archive.go`: in `dropSession`,

```go
	cmds := []tea.Cmd{m.stopSessionCmd(sess, nil), m.resizeSelected(), m.followSession(), m.dropTurnCmd(sess)}
```

and in `forgetCardMaps`:

```go
	delete(m.turnPending, id) // the turn baseline's bookkeeping (#311)
	delete(m.turnErr, id)
```

`cmd/omatty/wiring.go`, in `tuiDeps`: build the source once and use it for all four readers.

```go
	src := review.NewSource(git)
	...
		Diff:    src.Load,
		Stat:    src.Stat,
		Turn:    ui.TurnFuncs{Snap: src.SnapTurn, Diff: src.LoadTurn, Drop: src.DropTurn},
```

- [ ] **Step 4: Run to see them pass**

Run: `go test ./internal/ui ./cmd/omatty`
Expected: `ok` for both. If `TestForgetSession_ClearsEveryPerSessionMap` or `TestStopSession_KeepsEveryPerSessionMapButTerms_issue318` fails, a map is missing from `filledModel` or `forgetCardMaps`.

- [ ] **Step 5: Run the full gate** (Global Constraints). If `funlen` flags `tuiDeps`, move the `Turn` line into `withStoreDeps`' neighbour, a new `withReviewSource(deps, git)` beside `withTableDeps`.

- [ ] **Step 6: Commit**

```bash
git add internal/ui cmd/omatty
git commit -m "feat(#311): take a turn baseline on a hook prompt, drop it on archive"
```

---

### Task 5: `ui` — the turn scope in the review column

**Files:**
- Modify: `internal/ui/review.go` (`ReviewPane` fields, `TurnLoadedMsg`, `loadDiff` split, `onTurnLoaded`, `rebuildEntries`, `shownDiff`)
- Modify: `internal/ui/turn.go` (`onTurnSnapped` reloads an open turn view; `toggleScope`; `turnNotice`)
- Modify: `internal/ui/reviewkeys.go` (`t`; comment anchor from the shown diff)
- Modify: `internal/ui/reviewview.go` (title part, body notices, every row lookup through `shownDiff()`)
- Modify: `internal/ui/crosslink.go` (row lookups through `shownDiff()`)
- Modify: `internal/ui/model.go` (`onDataMsg` case for `TurnLoadedMsg`)
- Modify: `internal/ui/modalview.go` (help row), `README.md` (Review section)
- Test: `internal/ui/turn_test.go`

**Interfaces:**
- Consumes: `TurnFuncs.Diff`, `turnErr`, `onTurnSnapped` (Task 4); `review.ErrNoTurn` (Task 3).
- Produces: `type ui.TurnLoadedMsg struct { SessionID string; Diff review.Diff; Err error }`; `ReviewPane.Scope reviewScope`, `ReviewPane.TurnDiff review.Diff`, `ReviewPane.TurnErr error`, `ReviewPane.TurnReady bool`; `func (m *Model) shownDiff() review.Diff`.

- [ ] **Step 1: Write the failing tests.** Append to `internal/ui/turn_test.go`, and add `"strings"` to its imports:

```go
// turnDiffText is what one turn did on top of sampleDiff's first file: it
// added c := 4. The full diff also has b's change and new.txt; this does not.
const turnDiffText = `diff --git a/internal/ui/model.go b/internal/ui/model.go
index 3333333..2222222 100644
--- a/internal/ui/model.go
+++ b/internal/ui/model.go
@@ -10,4 +10,5 @@ func (m *Model) onKey() {
 	a := 1
 	b := 3
+	c := 4
 	return
 }
`

func turnDiffParsed(t *testing.T) review.Diff {
	t.Helper()
	d, err := review.ParseDiff(strings.NewReader(turnDiffText))
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// modelWithTurnDiff opens the review column on s1 with sampleDiff as the
// whole session and turnDiffText as the turn.
func modelWithTurnDiff(t *testing.T, tr *turnRecorder) (*ui.Model, map[string]*termwrap.Fake) {
	t.Helper()
	terms, fakes := fakeTerms(t)
	rec := &diffRecorder{Diff: sampleDiffParsed(t)}
	tr.Diff = turnDiffParsed(t)
	d := baseDeps(twoProjectState(), terms)
	d.Diff, d.Turn = rec.fn, tr.funcs()
	m := ui.NewModel(d)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('d'))
	return m, fakes
}

func TestModel_tShowsOnlyThisTurn_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})

	pressAndSettle(m, key('t'))
	body := m.View().Content
	if !strings.Contains(body, "this turn") || strings.Contains(body, "fresh") || strings.Contains(body, "b := 2") {
		t.Errorf("the turn scope does not show only this turn:\n%s", body)
	}

	pressAndSettle(m, key('t'))
	body = m.View().Content
	if strings.Contains(body, "this turn") || !strings.Contains(body, "fresh") {
		t.Errorf("t again does not return to the whole session:\n%s", body)
	}
}

// A narrow column gives up title parts; "this turn" is never one of them,
// because a turn view that reads as the whole diff is this feature's worst
// failure.
func TestModel_thisTurnStaysInTheTitleAtANarrowWidth_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 30})

	pressAndSettle(m, key('t'))

	if !strings.Contains(m.View().Content, "this turn") {
		t.Errorf("the title lost \"this turn\" at 80 columns:\n%s", m.View().Content)
	}
}

func TestModel_noBaselineSaysSo_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m, _ := modelWithTurnDiff(t, tr)
	tr.Diff, tr.DiffErr = review.Diff{}, review.ErrNoTurn

	pressAndSettle(m, key('t'))

	body := m.View().Content
	if !strings.Contains(body, "no turn recorded yet") || strings.Contains(body, "no changes") {
		t.Errorf("a missing baseline is not explained:\n%s", body)
	}
}

// After a failed snapshot the ref still names the previous turn; showing
// that diff would present two turns as one.
func TestModel_aFailedSnapshotShowsInsteadOfTheLastTurn_issue311(t *testing.T) {
	tr := &turnRecorder{SnapErr: errDiskFull}
	m, _ := modelWithTurnDiff(t, tr)
	_, cmd := m.Update(hookPrompt("s1"))
	settle(m, cmd)

	pressAndSettle(m, key('t'))

	body := m.View().Content
	if !strings.Contains(body, "could not be taken") || strings.Contains(body, "c := 4") {
		t.Errorf("a failed baseline did not replace the turn diff:\n%s", body)
	}
}

// Outside this turn is not moved - it is elsewhere in the session - so the
// comment is hidden here, and still counted and sent.
func TestModel_aCommentOutsideThisTurnIsHiddenNotMoved_issue311(t *testing.T) {
	m, fakes := modelWithTurnDiff(t, &turnRecorder{})
	down(m, 3)
	typeNote(m, "about b")

	pressAndSettle(m, key('t'))
	body := m.View().Content
	if strings.Contains(body, "(moved)") || strings.Contains(body, "about b") {
		t.Errorf("a comment outside this turn is drawn:\n%s", body)
	}

	press(m, shiftS)
	if len(fakes["s1"].Sent) != 1 || !strings.Contains(fakes["s1"].Sent[0], "about b") {
		t.Errorf("S in the turn scope did not send the comment outside it: %q", fakes["s1"].Sent)
	}
}

// Review Focus 3: the anchor is content, so a comment written on this
// turn's line sits on the same line in the whole-session view.
func TestModel_aCommentWrittenInTheTurnScopeStaysOnItsLine_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})
	pressAndSettle(m, key('t'))
	down(m, 4) // file, hunk, a, b, then +c := 4
	typeNote(m, "why 4")

	pressAndSettle(m, key('t'))

	body := m.View().Content
	if strings.Contains(body, "(moved) why 4") || !strings.Contains(body, ">> why 4") {
		t.Errorf("the comment did not follow its line into the whole-session view:\n%s", body)
	}
}

// PruneSent drops a sent comment whose line is gone. Outside this turn is
// not gone, so a turn load must never prune.
func TestModel_aTurnLoadNeverPrunesASentComment_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})
	down(m, 3)
	typeNote(m, "why?")
	press(m, shiftS)
	refocusColumn(m)

	pressAndSettle(m, key('t'))
	pressAndSettle(m, key('t'))

	if !strings.Contains(m.View().Content, "(sent") {
		t.Errorf("the sent comment was pruned by the turn load:\n%s", m.View().Content)
	}
}

// Review Focus 2: a turn diff that arrives after the column moved on, or
// after the scope went back to the whole session, is dropped.
func TestModel_aLateTurnDiffIsDropped_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})

	m.Update(ui.TurnLoadedMsg{SessionID: "s1", Diff: turnDiffParsed(t)})

	body := m.View().Content
	if strings.Contains(body, "this turn") || !strings.Contains(body, "fresh") {
		t.Errorf("a turn diff replaced the whole-session view:\n%s", body)
	}
}

// Review Focus 4.
func TestModel_rInTheTurnScopeReloadsTheTurn_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m, _ := modelWithTurnDiff(t, tr)
	pressAndSettle(m, key('t'))
	before := tr.DiffCalls

	pressAndSettle(m, key('r'))

	if tr.DiffCalls != before+1 {
		t.Errorf("r loaded the turn %d times, want 1", tr.DiffCalls-before)
	}
}

// Review Focus 5: before the turn diff arrives the column says it is
// reading, never "no changes", which would be a claim.
func TestModel_theTurnScopeSaysItIsReadingUntilTheDiffArrives_issue311(t *testing.T) {
	m, _ := modelWithTurnDiff(t, &turnRecorder{})

	press(m, key('t')) // the load is scheduled, not yet run

	body := m.View().Content
	if !strings.Contains(body, "reading this turn") || strings.Contains(body, "no changes") {
		t.Errorf("the turn scope claims something before its diff arrived:\n%s", body)
	}
}

// A snapshot that lands while the turn view is open reloads it.
func TestModel_aSnapshotReloadsAnOpenTurnView_issue311(t *testing.T) {
	tr := &turnRecorder{}
	m, _ := modelWithTurnDiff(t, tr)
	pressAndSettle(m, key('t'))
	before := tr.DiffCalls

	_, cmd := m.Update(hookPrompt("s1"))
	settle(m, cmd)

	if tr.DiffCalls != before+1 {
		t.Errorf("the turn view loaded %d times after a snapshot, want 1", tr.DiffCalls-before)
	}
}
```

Add `"github.com/WilsonSousajr/omatty/internal/termwrap"` to the imports.

- [ ] **Step 2: Run to see the failures**

Run: `go test ./internal/ui -run issue311`
Expected: build failure, `ui.TurnLoadedMsg undefined`; once that compiles, the `t` tests fail with the whole-session view still showing.

- [ ] **Step 3: Implement the pane state.** `internal/ui/review.go`, `ReviewPane`, after `Entries`:

```go
	// Scope is the whole session or this turn (#311). TurnDiff is what the
	// turn scope draws, TurnErr its last load's error (ErrNoTurn is a notice,
	// not a failure), and TurnReady whether a load has answered since the
	// scope was entered - until it has, the column says it is reading.
	Scope     reviewScope
	TurnDiff  review.Diff
	TurnErr   error
	TurnReady bool
```

After `ReviewPane`:

```go
// reviewScope is how much of the session the diff view shows (#311).
type reviewScope int

const (
	scopeSession reviewScope = iota // everything the session changed
	scopeTurn                       // what changed since this turn began
)

// TurnLoadedMsg carries a loaded turn diff into Update. Exported so tests
// can send one.
type TurnLoadedMsg struct {
	SessionID string
	Diff      review.Diff
	Err       error
}

// shownDiff is the diff the rows are drawn from and indexed into. PruneSent,
// Compose and the tree's markers keep m.review.Diff, the whole session, on
// purpose (#311).
func (m *Model) shownDiff() review.Diff {
	if m.review.Scope == scopeTurn {
		return m.review.TurnDiff
	}
	return m.review.Diff
}
```

Rename the existing `loadDiff` to `loadFullDiff` (body unchanged) and add:

```go
// loadDiff reloads what the column shows: always the whole session, which
// the tree's markers and PruneSent read, and the turn as well while the turn
// scope is on, so it refreshes on every trigger the full diff has.
func (m *Model) loadDiff(id string) tea.Cmd {
	return tea.Batch(m.loadFullDiff(id), m.loadTurn(id))
}

// loadTurn fetches the turn diff when the column is showing that session's
// turn, and nothing otherwise.
func (m *Model) loadTurn(id string) tea.Cmd {
	if !m.review.Open || m.review.Scope != scopeTurn || id != m.review.SessionID {
		return nil
	}
	sess, ok := m.session(id)
	if !ok {
		return nil
	}
	root, load := m.projectRoot(sess.Project), m.turn.Diff
	return func() tea.Msg {
		d, err := load(sess, root)
		return TurnLoadedMsg{SessionID: id, Diff: d, Err: err}
	}
}

// onTurnLoaded paints a turn diff, unless the column moved on or went back
// to the whole session while git ran.
func (m *Model) onTurnLoaded(msg TurnLoadedMsg) tea.Cmd {
	if !m.review.Open || msg.SessionID != m.review.SessionID || m.review.Scope != scopeTurn {
		return nil
	}
	if msg.Err != nil && !errors.Is(msg.Err, review.ErrNoTurn) {
		slog.Warn("loading a turn diff", "session", msg.SessionID, "err", msg.Err)
	}
	m.review.TurnDiff, m.review.TurnErr, m.review.TurnReady = msg.Diff, msg.Err, true
	m.rebuildEntries()
	return nil
}
```

`rebuildEntries` draws from the shown diff and, in the turn scope, drops orphans:

```go
func (m *Model) rebuildEntries() {
	d := m.shownDiff()
	placed := review.Place(d, m.commentsFor(m.review.SessionID).All())
	if m.review.Scope == scopeTurn {
		// A comment that does not place on this turn is not moved: it is
		// elsewhere in the session, still counted and still sent (#311).
		placed.Orphans, placed.Lost = nil, nil
	}
	m.review.Entries = review.Flatten(d, placed)
	m.contentChanged()
	if m.review.Cursor >= len(m.review.Entries) {
		m.review.Cursor = max(len(m.review.Entries)-1, 0)
	}
}
```

`internal/ui/model.go`, `onDataMsg`:

```go
	case TurnLoadedMsg:
		return m.onTurnLoaded(typed)
```

- [ ] **Step 4: The toggle and the notices.** `internal/ui/turn.go`: `onTurnSnapped`'s two `return nil` after the known-session guard become `return m.loadTurn(msg.SessionID)`. Add:

```go
// toggleScope switches the diff between the whole session and this turn,
// starting from the top: the two are different lists of rows.
func (m *Model) toggleScope() tea.Cmd {
	if m.review.Scope == scopeTurn {
		m.review.Scope = scopeSession
	} else {
		m.review.Scope = scopeTurn
	}
	m.review.Cursor, m.review.Offset, m.review.ColOffset = 0, 0, 0
	m.review.TurnDiff, m.review.TurnErr, m.review.TurnReady = review.Diff{}, nil, false
	m.rebuildEntries()
	return m.loadTurn(m.review.SessionID)
}

// turnNotice is what the turn scope says instead of rows, split into lines
// short enough for the narrowest column; nil when there are rows to show.
// isErr says a load failed, as opposed to there being nothing to show yet.
func (m *Model) turnNotice() (lines []string, isErr bool) {
	id := m.review.SessionID
	switch {
	case m.turnErr[id] != "":
		return []string{"this turn's baseline", "could not be taken:", m.turnErr[id]}, true
	case errors.Is(m.review.TurnErr, review.ErrNoTurn):
		return []string{"no turn recorded yet:", "a baseline is taken", "when you send a prompt"}, false
	case m.review.TurnErr != nil:
		return []string{"reading this turn failed:", m.review.TurnErr.Error()}, true
	case !m.review.TurnReady:
		return []string{"reading this turn..."}, false
	}
	return nil, false
}
```

(add `"errors"` and `"github.com/WilsonSousajr/omatty/internal/review"` to `turn.go`'s imports; `review.go` also needs `"errors"`.)

`internal/ui/reviewkeys.go`, `reviewAction`:

```go
	case "t":
		return m.toggleScope()
```

and in the comment constructor at lines 109-110 use `m.shownDiff()` for both `AnchorAt` and `LineAt`: the cursor indexes the shown diff.

`internal/ui/reviewview.go`:
- `reviewBody`, after the `m.review.Err` check:

```go
	if m.review.Scope == scopeTurn {
		if lines, isErr := m.turnNotice(); lines != nil {
			return noticeLines(lines, isErr, w)
		}
	}
	if len(m.review.Entries) == 0 {
		return []string{mutedStyle.Render(m.noChanges())}
	}
```

with

```go
// noChanges is the empty diff's line, which says which diff was empty.
func (m *Model) noChanges() string {
	if m.review.Scope == scopeTurn {
		return "no changes this turn"
	}
	return "no changes"
}

func noticeLines(lines []string, isErr bool, w int) []string {
	style := mutedStyle
	if isErr {
		style = errorStyle
	}
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = style.Render(fitLine(l, w))
	}
	return out
}
```

- `diffTitleParts`: replace the `parts := []titlePart{...}` literal and the `pairingNote` line with

```go
	d := m.shownDiff()
	parts := []titlePart{{text: "diff", priority: keepAlways}}
	if m.review.Scope == scopeTurn {
		// keepFlag, as ⚠ no tests: a turn view that reads as the whole diff
		// is this feature's worst failure (#311).
		parts = append(parts, titlePart{text: "this turn", priority: keepFlag})
	}
	parts = append(parts,
		titlePart{text: fmt.Sprintf("%d files", len(d.Files)), priority: dropFiles},
		titlePart{text: fmt.Sprintf("%d comments", comments), priority: priority})
	if note := pairingNote(d); note != "" {
```

leaving the rest of the function as it is.
- Every other `m.review.Diff` in `reviewview.go` (lines 283, 296, 333, 351, 355, 384, 389) and in `crosslink.go` (21, 41, 70, 76) becomes `m.shownDiff()`. `tree.go:113`, `review.go:241` and `reviewkeys.go:169/171` stay on `m.review.Diff`.

`internal/ui/modalview.go`, after the `S` row:

```go
	{"t", "diff: the whole session, or only this turn"},
```

`README.md`, the Review key table after `r`:

```
| `t` | switch between the whole session and only this turn |
```

and after the paragraph on anchoring:

```
`t` narrows the diff to what changed since you last sent the session a
prompt, and back. omatty takes the baseline when the prompt hook fires, as a
git tree under `refs/omatty/turn/<session>` built through a temporary index,
so your staging, HEAD and stash are never touched; archiving the session
deletes it. Comments work the same in both views: one outside this turn is
hidden rather than shown as moved, and `S` still sends it.
```

- [ ] **Step 5: Run the tests**

Run: `go test ./internal/ui`
Expected: `ok`. A pre-existing test that asserted on `loadDiff`'s message type still passes, because `tea.Batch` with one non-nil command returns that command itself.

- [ ] **Step 6: Run the full gate.** If `funlen` or `gocognit` flags `diffTitleParts` or `reviewBody`, move the new branch into its own function (`turnTitlePart`, `turnBody`), as `submitGate` was split in #335.

- [ ] **Step 7: Commit**

```bash
git add internal/ui README.md
git commit -m "feat(#311): t shows only what changed since the turn began"
```

---

### Task 6: Smoke test, changelog, PR

**Files:**
- Modify: `CHANGELOG.md` (Unreleased)

- [ ] **Step 1: Real-PTY smoke test.** Scratch `HOME` at the short path `/tmp/omv2` (the socket path limit), `fake-claude` on PATH, built binaries under the scratchpad (`$S`), as in the v0.2.0 and #335 runs:

```bash
S=<scratchpad>/smoke; T=/tmp/omv2
go build -o $S/omatty ./cmd/omatty && go build -o $S/ptyrun ./testdata/ptyrun && go build -o $S/screen ./testdata/screen
rm -rf $T && mkdir -p $T/bin $T/repo && ln -s "$PWD/testdata/fake-claude" $T/bin/claude
( cd $T/repo && git init -q -b main && printf 'a\n' > a.txt && git add . && git -c user.name=s -c user.email=s@s commit -qm init && printf 'before the prompt\n' > early.txt )
printf 'exec env -i HOME=%s PATH=%s/bin:%s:/opt/homebrew/bin:/usr/bin:/bin TERM=xterm-256color "$@"\n' $T $T "$(go env GOROOT)/bin" > $T/env.sh
sh $T/env.sh $S/omatty add $T/repo && sh $T/env.sh $S/omatty new repo main
ID=$(python3 -c 'import json;print(json.load(open("/tmp/omv2/.omatty/state.json"))["sessions"][0]["id"])')
mkdir -p $T/.omatty && printf '[sessions]\nlazy_start = false\n' > $T/.omatty/config.toml
# in the background: at 3s the prompt hook fires, at 4s the turn writes a file
( sleep 3; printf '{"session_id":"%s","hook_event_name":"UserPromptSubmit"}' $ID | sh $T/env.sh OMATTY_SESSION=$ID $S/omatty hook; sleep 1; printf 'during the turn\n' > $T/repo/late.txt ) &
sh $T/env.sh PTY_COLS=140 PTY_ROWS=20 PTY_WAIT=6s PTY_KEYS=$'\x0fd' PTY_WAIT2=2s PTY_KEYS2=t $S/ptyrun $S/omatty > $S/t311.raw
python3 $S/show.py $S/t311.raw && $S/screen $S/t311.raw.trim 140 20
git -C $T/repo for-each-ref refs/omatty/
```

Read the frame: the title says `this turn`, `late.txt` is listed, `early.txt` is not. Run again with `PTY_KEYS2=''` and confirm the whole-session view lists both files. `for-each-ref` shows `refs/omatty/turn/<ID>`. Then archive the session in a third run (`PTY_KEYS=$'\x0fx'`, `PTY_KEYS2=y`) and confirm the ref is gone. Kill any `$T/bin/claude`, `chmod -R u+w $T`, remove it.

Record in the PR that a real-claude run, which would confirm the hook's timing against the first edit, is owed.

- [ ] **Step 2: CHANGELOG.** Under `## [Unreleased]`, add `### Added` and `### Fixed` if absent:

```markdown
### Added

- **`t` in the review column shows only this turn.** When a prompt is
  submitted, omatty snapshots the session's working tree as a git tree under
  `refs/omatty/turn/<session>`, built through a temporary index, so staging,
  HEAD and the stash are never touched; `t` diffs the tree now against it.
  Only the prompt hook takes the baseline, never the transcript tailer, which
  also reports tool results. A failed snapshot is shown instead of a diff that
  would span two turns; archiving deletes the ref. (#311)
- **A sent review comment stays on its line**, muted and marked
  `(sent 14:36)`, is never sent again, and goes once its line does; the gate
  pane's `S` asks before resending the same failures. (#335)

### Fixed

- The gate pane's rows keep one command column, pending or with a verdict,
  whatever the length of the step names. (#342)
```

- [ ] **Step 3: Full gate, commit, push, PR**

```bash
git add CHANGELOG.md
git commit -m "docs(#311): changelog for #311, #335 and #342"
git push -q
```

Open the PR into `develop` titled `feat(#311): review scoped to the last turn`, body with **Closes #311**, what changed per task, the verification (tests first, negative control, gate, smoke frames), and the attribution footer. Put the PR in Review and #311 in Review on project 13; merge once CI is green on both runners (standing approval for the verification-core batch), then move both to Done.
