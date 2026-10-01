# Phase 2 Safety Net Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pin omatty's current behaviour at every seam ADR 0001 will cut, and
report the layer rule's violations, **without changing one line of production
code**. The migration then has a net that fails the moment a move changes
anything a person can see.

**Architecture:** The net has three kinds of test, plus a report.

1. **Golden files.** They pin what is persisted (`state.json`) and what is
   drawn (`View()` for the main screens at fixed sizes).
2. **Characterization tables, also stored as golden files.** They pin
   behaviour:
   - every key in every focus context → an observable outcome;
   - every routed message type → the resulting state and the message the
     returned Cmd produces.
3. **A report-only layer check** (`tools/layercheck`) that applies ADR 0001's
   layer table to today's import graph and prints what the migration has to
   fix.

**Tech Stack:** Go 1.26.8 stdlib `testing` with a per-package `-update` flag,
the existing `internal/golist` and `internal/depgraph`, and bash for the
script. No new dependency.

**Spec:** `docs/adr/0001-architecture.md` (accepted, #618), with the evidence
in `docs/ARCHITECTURE_AUDIT.md` (#615).

## Global Constraints

- **Zero production changes.** Only these may change:
  - `*_test.go` files
  - `testdata/` golden files
  - `tools/layercheck/`
  - `scripts/`
  - `.github/workflows/ci.yml`
  - AGENTS.md's gate list
  - this plan

  `internal/**/*.go` non-test files are untouched: `git diff --stat
  origin/develop -- 'internal/**/*.go' ':!*_test.go'` must be empty.
- Go 1.26.8. No new module dependency; `go mod tidy -diff` must stay empty.
- Every tracked `.go` file stays ≤ 500 lines (`scripts/check-file-length.sh`).
  That includes `internal/ui/export_test.go`, which is 318 lines today, so new
  accessors go in a new `export_characterize_test.go`.
- Test names follow AGENTS.md. Each characterization test is named after what
  it pins, and every test references #620, the Phase 2 issue.
- Goldens are rewritten only by `go test ./<pkg> -run <Test> -update`. A
  golden diff in a later PR is either the intended change, stated in the
  commit, or a regression.
- Commit style is `test(#620): …` / `ci(#620): …`, with `Co-Authored-By` and
  **no session URL**.
- Tests are F.I.R.S.T.: no `time.Sleep`, no wall clock, no dependence on
  environment variables or map iteration order.
- The full local gate runs before the PR: gofmt, vet, `go mod tidy -diff`,
  golangci-lint, govulncheck, check-deps, check-file-length, shellcheck,
  `go test ./... -race`, coverage 90, C.R.A.P. 12.

## Review Focus

1. **A golden that changes from run to run.** Every View golden is rendered
   twice in the same test and under `NO_COLOR=1` / `TERM=dumb` /
   `COLORTERM=truecolor`, and must be byte-identical each time. That test is
   in Task 2.
2. **A golden that pins the wall clock.** Every model built for a golden sets
   `Deps.Clock` to `fixedNow`, because `baseDeps` leaves the wall clock in. A
   test that rebuilds the goldens one minute later must produce no diff; Task
   2 runs `-count=2`.
3. **A key table that is quietly incomplete.**
   - Uppercase and shifted letters arrive in three spellings, so the key set
     sends all three: `{Code:'n',Mod:shift,Text:"N"}`, `{Code:'N',Text:"N"}`
     and `{Code:'n',Mod:shift}`. Task 3 asserts the table has a row for each.
   - Task 3 also asserts row count = contexts × keys, so a context that
     silently fails to set up cannot pass as "no effect".
4. **A layer report that exits non-zero and so blocks CI in report mode.**
   Task 5 tests that `layercheck` exits 0 with violations present unless
   `-enforce` is passed.
5. **An unclassified package reported as clean.** A package the transitional
   table does not know is printed as `unlayered`, never skipped. Task 5 tests
   it.

---

### Task 0: Branch

Issue #620 is open, labelled `test area:ui area:registry M17`, and In Progress on the board.

- [ ] `git switch -c test/620-safety-net origin/develop`

---

### Task 1: `state.json` golden (invariant 9)

**Files:**
- Create: `internal/registry/schema_test.go`
- Create: `internal/registry/golden_test.go` (the `-update` flag and `assertGolden`)
- Create: `internal/registry/testdata/state.golden.json`

**Interfaces:**
- Produces: `assertGolden(t *testing.T, name string, got []byte)` in package
  `registry_test`. It compares `got` against `testdata/<name>` and rewrites
  the file under `-update`. Task 2 carries its own copy for `ui_test`: two
  packages, and a shared test helper would need a production package.

- [ ] **Step 1: Write the golden helper**

```go
// golden_test.go
package registry_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

// update rewrites golden files instead of comparing them. Goldens change only
// on purpose: go test ./internal/registry -run <Test> -update.
var update = flag.Bool("update", false, "rewrite golden files")

// assertGolden fails when got differs from testdata/name, so a schema change
// is always a visible diff in review (#620).
func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s (run with -update to create it): %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("%s differs from the golden file; if the change is intended, rerun with -update\n--- got\n%s\n--- want\n%s", path, got, want)
	}
}
```

- [ ] **Step 2: Write the failing schema test**

Build a `registry.State` in which **every** field of `State`, `Project` and
`Session` is non-zero: Gate, Carry, GateRuns, GateFirstPass, Collapsed, Base,
Worktree, Agent, Conversation and a fixed `Started`. Read `state.go:15-115` and
set each tagged field. Save it through the real `Store` into `t.TempDir()`,
read the bytes back, and golden them. Then load the golden back and require
`reflect.DeepEqual` with the input.

```go
// schema_test.go
package registry_test

// Pins every state.json key before ADR 0001 moves these types into
// domain/session (#620). Invariant 9: state.json alone must relaunch every
// session, so a renamed tag is a lost session, not a refactor.
func TestState_schemaIsPinned_issue620(t *testing.T) {
	st := everyFieldSet()
	store := registry.NewStore(filepath.Join(t.TempDir(), "state.json"))
	if err := store.Save(st); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatal(err)
	}
	assertGolden(t, "state.golden.json", raw)
}

func TestState_goldenLoadsBackToTheSameState_issue620(t *testing.T) {
	dir := t.TempDir()
	golden, err := os.ReadFile(filepath.Join("testdata", "state.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state.json")
	if err := os.WriteFile(path, golden, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := registry.NewStore(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := everyFieldSet(); !reflect.DeepEqual(got, want) {
		t.Errorf("golden loads as\n%+v\nwant\n%+v", got, want)
	}
}
```

`everyFieldSet()` is a helper in `schema_test.go`. Check `registry.NewStore`'s
real signature and how the store exposes its path; if there is no `Path()`,
keep the path in a local variable. Add a third test that walks `State`,
`Project` and `Session` with `reflect` and fails if any field is zero in
`everyFieldSet()`. That way a field added later cannot silently escape the pin.

- [ ] **Step 3: Run it and watch it fail.** Run `go test ./internal/registry
  -run schema -count=1`. Expected: FAIL, "reading golden … (run with -update
  to create it)".
- [ ] **Step 4: Create the golden.** Run `go test ./internal/registry -run
  TestState_schemaIsPinned -update`. Open `testdata/state.golden.json` and
  read it: every key named in `state.go`'s tags appears. The load-back test
  now passes.
- [ ] **Step 5: Negative control.** Temporarily change one JSON tag in
  `state.go` (`"base"` → `"base2"`), confirm both tests FAIL, then revert
  with `git checkout internal/registry/state.go`.
- [ ] **Step 6: Commit** `test(#620): pin every state.json key with a golden file`.

---

### Task 2: View goldens for the main screens

**Files:**
- Create: `internal/ui/golden_test.go` (the `-update` flag, `assertGolden`, `scene` table)
- Create: `internal/ui/viewgolden_test.go` (the tests)
- Create: `internal/ui/testdata/view/<scene>_<w>x<h>.golden` and `.plain.golden`

**Interfaces:**
- Consumes the existing helpers `baseDeps`, `fakeTermsFor`, `fixedNow`,
  `settle`, `leader`, `modelWithDiff`, `modelWithTree`, `trackerModel`,
  `itemModel`, `modelWithGate` and the `stripSGR` regexp (`meter_test.go:14`).
- Produces: `scenes []scene` with `type scene struct{ name string; build
  func(t *testing.T) *ui.Model }`. Task 4 reuses `scene.build` as its starting
  states.

**Scenes.** Each is built from existing helpers, sized with
`tea.WindowSizeMsg`, and has `Deps.Clock = func() time.Time { return fixedNow }`:

| Scene | How it is reached |
|---|---|
| `terminal` | `modelWithFakes`, terminal focused, s1 selected |
| `sidebar_many` | `sevenProjectState()`, cursor mid-list |
| `diff` | `modelWithDiff` with its fixture diff, `ctrl+o d` |
| `diff_turn` | `modelWithTurnDiff`, turn scope (`t`) |
| `tree` | `modelWithTree`, tree face |
| `preview` | `modelWithTree`, enter on a file |
| `gate` | `modelWithGate`, with a delivered report of one pass, one fail, one missing |
| `tracker` | `trackerModel` with PRs and issues loaded |
| `tracker_item` | `itemModel`, one item open |
| `help` | `ctrl+o ?` |
| `new_session` | `ctrl+o n` modal |
| `switcher` | the switcher modal (`modelWithManySessions`) |
| `zoomed_diff` | `diff` plus zoom |

Sizes are `120x30` and `80x24`: the smoke-test size and the narrowest
common one.

- [ ] **Step 1: Write the helper and the table, then the failing test**

```go
// viewgolden_test.go
package ui_test

// Pins every main screen's frame before ADR 0001 splits internal/ui into
// tui/app + screens (#620). The .golden file keeps the SGR, so a theme move
// that changes one colour fails; the .plain.golden strips it so a reviewer
// can read the diff.
func TestView_mainScreensArePinned_issue620(t *testing.T) {
	for _, sc := range scenes {
		for _, size := range [][2]int{{120, 30}, {80, 24}} {
			name := fmt.Sprintf("%s_%dx%d", sc.name, size[0], size[1])
			t.Run(name, func(t *testing.T) {
				frame := renderScene(t, sc, size[0], size[1])
				assertGolden(t, filepath.Join("view", name+".golden"), []byte(frame))
				assertGolden(t, filepath.Join("view", name+".plain.golden"), []byte(stripSGR(frame)))
			})
		}
	}
}

// A golden that differs between two renders, or with the terminal's colour
// environment, would fail at random and teach everyone to rerun -update.
func TestView_goldensAreDeterministic_issue620(t *testing.T) {
	for _, env := range [][2]string{{"NO_COLOR", "1"}, {"TERM", "dumb"}, {"COLORTERM", "truecolor"}} {
		t.Setenv(env[0], env[1])
		for _, sc := range scenes {
			a, b := renderScene(t, sc, 120, 30), renderScene(t, sc, 120, 30)
			if a != b {
				t.Errorf("%s renders differently twice under %s=%s", sc.name, env[0], env[1])
			}
		}
	}
}
```

`renderScene` builds the model, delivers the `WindowSizeMsg` through
`settle`, and returns `m.View().Content`. The determinism test must also
compare against the committed golden under each env. If lipgloss reads the
environment, record that finding in the commit and pin
`lipgloss.SetColorProfile` in the test only; production is untouched either
way.

- [ ] **Step 2: Run it and watch it fail.** Expected: every subtest FAILs
  with "reading golden …".
- [ ] **Step 3: Generate with `-update`, then read every `.plain.golden`
  yourself.** Each must show the screen its name claims: title row, panes,
  footer. A scene that renders the wrong view is a broken `build`, not a
  golden to accept. Fix the builder.
- [ ] **Step 4: Run twice to check stability.** `go test ./internal/ui -run
  'TestView_' -count=2` PASSes. Then run with `-race`.
- [ ] **Step 5: Negative control.** Temporarily change one colour constant in
  `style.go`; the `.golden` subtests FAIL and the `.plain.golden` ones do
  not. Revert.
- [ ] **Step 6: Commit** `test(#620): golden frames for the main screens at 120x30 and 80x24`.

---

### Task 3: Key characterization table (invariant 1, pain point 3)

**Files:**
- Create: `internal/ui/export_characterize_test.go` (package `ui`)
- Create: `internal/ui/keytable_test.go`
- Create: `internal/ui/testdata/keys.golden`

**Interfaces:**
- Produces, in `export_characterize_test.go` (test-only, package `ui`):

```go
// Fingerprint is the part of the model a keypress can change, as one line, so
// a key table can say what every key does without knowing how (#620).
func (m *Model) Fingerprint() string {
	return fmt.Sprintf("focus=%v view=%v modal=%v sel=%s open=%v zoom=%v diffcur=%d filecur=%d gatecur=%d trackcur=%d filter=%q",
		m.focus(), m.review.View, m.modal.Kind, m.Selected(), m.review.Open, m.review.Zoomed,
		m.review.DiffList.Cursor(), m.review.Files.Cursor(), m.review.GateCursor, m.TrackerCursor(), m.activeFilter().Query)
}
```

  Read the real field and method names before writing this: `listWindow`'s
  cursor accessor, `focus()` and `activeFilter()`. The fields listed are the
  ones ADR 0001 dissolves into screens.

**Contexts** (13), each a fresh model:
- terminal focused
- leader armed (`ctrl+o` pressed)
- review focused on each of the 6 views: diff, tree, preview, gate, tracker, tracker item
- diff with the comment editor open
- tree filter active
- the new-session modal
- the help modal
- the switcher modal

**Keys** (built once):
- `a`–`z` as `key(r)`
- for each letter, all three shift spellings (Review Focus 3)
- `0`–`9` and ASCII punctuation
- `ctrl+a`–`ctrl+z` via `ctrl(r)`
- `special(...)` for enter, esc, tab, shift+tab, backspace, space, up, down, left, right, pgup, pgdown, home, end

**One row per (context, key):**

```
<context> <key.String()> <Code>/<Mod>/<Text> -> pty=<n msgs> <Fingerprint after>
```

`pty=` is the growth in `termwrap.Fake.Msgs` for the selected session, which
is how "went to the PTY" is observed (invariant 1).

- [ ] **Step 1: Write the failing test**

```go
// keytable_test.go
package ui_test

// Pins what every key does in every focus context before ADR 0001 moves the
// 24 switch-key sites onto key.Binding (#620). A binding that drops a spelling
// ("shift+N" from the legacy terminal, #87) changes a row here.
func TestKeys_everyKeyInEveryContextIsPinned_issue620(t *testing.T) {
	var rows []string
	for _, c := range keyContexts {
		for _, k := range allKeys() {
			m, terms := c.build(t)
			before := len(terms[m.Selected()].Msgs)
			pressAndSettle(m, k)
			rows = append(rows, fmt.Sprintf("%-14s %-12s %d/%d/%q -> pty=%d %s",
				c.name, k.String(), k.Code, k.Mod, k.Text, len(terms[m.Selected()].Msgs)-before, m.Fingerprint()))
		}
	}
	if got, want := len(rows), len(keyContexts)*len(allKeys()); got != want {
		t.Fatalf("%d rows, want %d: a context failed to build", got, want)
	}
	assertGolden(t, "keys.golden", []byte(strings.Join(rows, "\n")+"\n"))
}
```

  `keyContexts` entries return `(*ui.Model, map[string]*termwrap.Fake)`, built
  from `fakeTerms` / `baseDeps` with `Clock` fixed. Each context also asserts
  its own precondition through `Fingerprint()`, for example that the gate
  context really shows `view=gate`. A context that silently lands elsewhere
  is Review Focus 3.

  If `pressAndSettle` blocks on a Cmd that waits on a channel (`waitForEvent`
  with a nil `Events` is guarded; check the gate wait), use a bounded `settle`
  that drops those message types. The survey found `settle` already drops
  `SpinTickMsg`.

- [ ] **Step 2: Run it and watch it fail** on the missing golden.
- [ ] **Step 3: Generate, then audit the golden.**
  - In the `terminal` context, every non-leader key must read `pty=1` with an
    unchanged fingerprint. Any other result is an invariant 1 bug. **File it;
    do not accept it.**
  - `grep -c` the golden for each context: every context should have the
    same row count.
- [ ] **Step 4: Negative control.** Temporarily delete `"shift+N"` from one
  `case` in `routing.go`. The table FAILs on exactly that context's rows.
  Revert.
- [ ] **Step 5: Commit** `test(#620): pin every key in every focus context in one table`.

---

### Task 4: Message-routing characterization (the `Update` chain)

**Files:**
- Create: `internal/ui/msgtable_test.go`
- Modify: `internal/ui/export_characterize_test.go`: add constructors for the
  four unexported routed types (`generatedMsg`, `coverageMsg`,
  `previewRestMsg`, `sessionRelaunchMsg`), modelled on the existing
  `GeneratedMsgFor` (`export_test.go:316`).
- Create: `internal/ui/testdata/msgs.golden`

For each of the **38** routed message types (listed in `msgroute.go`: onInput,
onHeartbeat, onStreamMsg, onColumnMsg, onDataMsg, onNamingMsg, onTurnMsg,
onPaneMsg, onForgeMsg, onSessionMsg, onWindowFocus, WindowSizeMsg), plus one
unrouted type to pin the `broadcast` fallthrough:

1. Build a representative value.
2. Deliver it to the `diff` scene from Task 2.
3. Record `<type> -> cmd=<nil|type of each msg the Cmd yields, one level>
   frameChanged=<bool> <Fingerprint>`.

- [ ] **Step 1: Write the failing test.** `TestUpdate_everyRoutedMessageIsPinned_issue620`
  iterates a `[]struct{ name string; msg tea.Msg }` table. It asserts the table
  has one entry per case in the `on*` switches, counting them by `go/parser`
  over `msgroute.go`, the way `help_test.go:67` already parses `routing.go`.
  So a new message type added without a row fails.
- [ ] **Step 2: Run it and watch it fail** on the missing golden.
- [ ] **Step 3: Generate** the golden, then read it. Every Cmd type named
  must be one the handler plausibly returns.
- [ ] **Step 4: Negative control.** Temporarily swap two cases between `on*`
  tables; a row changes. Revert.
- [ ] **Step 5: Commit** `test(#620): pin what every routed message does to the model`.

---

### Task 5: `tools/layercheck`, a report-only layer rule

**Files:**
- Create: `tools/layercheck/main.go`: flags, `run()`, exit codes; ≤ 20-line functions
- Create: `tools/layercheck/layers.go`: `layerOf`, the rules, `check`
- Create: `tools/layercheck/layers_test.go`
- Create: `scripts/check-layers.sh`
- Create: `scripts/layers_test.go`
- Modify: `.github/workflows/ci.yml` (step after `file length`); `AGENTS.md` gate list

**Interfaces:**

```go
type Layer string // "domain" "service" "infra" "pubsub" "tui" "cli" "cmd" "tools" "unlayered"

// layerOf places a package by ADR 0001's path prefix. Today's packages come
// from the transitional table (the ADR's tree), which the migration empties.
func layerOf(importPath, module string) Layer

type Finding struct{ From, To string; FromLayer, ToLayer Layer; Rule string }

// check applies ADR 0001's layer table to every production edge and every
// third-party or stdlib import a layer must not have.
func check(module string, pkgs []golist.Package) []Finding
```

- **Allowed layer edges.** These are ADR 0001's table, verbatim:
  - domain → domain
  - service → domain, pubsub
  - infra → domain
  - tui → service, domain, pubsub
  - cli → service, domain
  - cmd → *
  - tools → *
- **Forbidden imports:**
  - `domain`: anything non-stdlib; `os`, `os/exec`, `net`, `net/http`
  - `service`: `os/exec`, `net/http`, `charm.land/*`
  - `infra`: `charm.land/*`
- **The transitional table maps today's 28 packages** to their ADR target
  layer: e.g. `internal/registry` → `service` (mixed, split later),
  `internal/vcs` → `infra`, `internal/ui` → `tui`, `internal/termwrap` → `tui`,
  `internal/paths` → `infra`. It is copied from the ADR tree, and each entry
  carries a one-word comment naming its target package.
- **Output:** one line per finding, then `layer findings: N (report only)`.
  The exit status is always 0 unless `-enforce` is set; then it is 1 when
  N > 0. It exits 2 if the graph cannot be read.

- [ ] **Step 1: Write the failing tests** in `tools/layercheck/layers_test.go`:
  - `TestLayerOf_placesTargetPathsByPrefix_issue620`, for all seven prefixes
  - `TestLayerOf_unknownPackageIsUnlayeredNotClean_issue620` (Review Focus 5)
  - `TestCheck_serviceImportingInfraIsAFinding_issue620`
  - `TestCheck_domainImportingOsIsAFinding_issue620`
  - `TestCheck_serviceImportingBubbleteaIsAFinding_issue620`
  - `TestCheck_tuiImportingServiceIsClean_issue620`
  - `TestCheck_everyTodayPackageIsInTheTransitionalTable_issue620`. It lists
    `./...` through `golist.List` and requires `layerOf` ≠ `unlayered` for
    each, so a package added during the migration must be placed.

  In `scripts/layers_test.go`:
  - `TestLayerCheck_reportsButPassesByDefault_issue620`, which runs
    `./scripts/check-layers.sh` on the repo and expects exit 0 plus `layer
    findings:` in the output (Review Focus 4)
  - `TestLayerCheck_enforceFailsOnFindings_issue620`, with `-enforce` on the
    repo today expecting a non-zero exit
  - `TestCI_runsTheLayerReport_issue620`
- [ ] **Step 2: Run them and watch them fail** (the package does not exist).
- [ ] **Step 3: Implement** `layers.go`, then `main.go`, modelled on
  `tools/depcheck/main.go`: `golist.Module(".")`, `golist.List(".", "./...")`,
  and the production `Imports` of each package. The script:

```bash
#!/usr/bin/env bash
# Reports ADR 0001's layer rule against the import graph (#620). Report-only
# until the migration's last PR adds -enforce to CI; the count is the backlog.
set -euo pipefail
go run ./tools/layercheck "$@"
```

- [ ] **Step 4: Run the tests and watch them pass.** Run `./scripts/check-layers.sh`
  and read the findings. They should match the audit's leak list: `ui` →
  infra, `registry` → `vcs`, `agent` → `paths`/`hooks`/`watcher`, and so on.
  Copy the count into the PR body; it is the migration's starting backlog.
- [ ] **Step 5: CI and docs.**
  - ci.yml gains `- name: layer report (ADR 0001, report-only)` / `run:
    ./scripts/check-layers.sh` after `file length`.
  - AGENTS.md's gate block gains `./scripts/check-layers.sh  # ADR 0001
    layers, report-only until the migration ends`.
- [ ] **Step 6: Commit** `ci(#620): report ADR 0001's layer rule, report-only`.

---

### Task 6: Gate, PR, stop

- [ ] Confirm zero production changes. `git diff --stat origin/develop --
  'internal/**/*.go' ':!*_test.go'` prints nothing.
- [ ] Run the full local gate (Global Constraints).
- [ ] Push and open a PR against develop that closes #620. The body should
  cover:
  - what each golden pins, and how to update one (`-update`, stated in the
    commit)
  - the layer report's starting count
  - the negative controls that were run
  - "no production file changed"
- [ ] Board: PR → Review. **Stop.** Phase 3 (`MIGRATION_PLAN.md`) waits on the user.
