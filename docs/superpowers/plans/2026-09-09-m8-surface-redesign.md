# M8 Surface Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task (this repo's standing preference is inline execution; any subagent must run on Opus). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace omatty's three rounded boxes with a header row, a rule and hairline-separated columns; two-line session cards with a rail cursor; the one-hue-one-meaning status palette; a facts footer; and a per-session branch + diffstat poll.

**Architecture:** Every geometry change is a constant or function in `internal/ui/layout.go`, which the caret, wheel and click code already derive from. The sidebar's row height becomes one function shared by the scroll window and the click hit-test. New data (branch, diffstat) follows the diff's route: `vcs.Git` → `review.Source` → a typed func in `ui.Deps` wired by `cmd/omatty`, polled by a `tea.Cmd` off the render path.

**Tech Stack:** Go 1.26, `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, stdlib `testing`, `git` CLI via `internal/vcs`.

**Spec:** `docs/superpowers/specs/2026-09-09-tui-redesign-design.md` (PR #173). Parent issue #172; slices #174 frame, #175 colour, #176 cards, #177 header, #178 footer, #180 diffstat, #179 close-out.

## Global Constraints

- Gate before any "done": `gofmt -l .` prints nothing; `go vet ./...`; `golangci-lint run`; `go test ./... -race`; `./scripts/check-coverage.sh 90`.
- Functions 4–20 lines, files under 500 lines, max 2 levels of indentation, `gocyclo` 10, `gocognit` 15, `dupl` on. Early returns.
- TDD: failing test first, run it, watch it fail for the right reason, then code.
- Every regression test is named after its issue: `TestX_whatWasWrong_issueN`.
- Commit messages `type(#issue): message`, each ending with the Co-Authored-By and Claude-Session trailers this session uses.
- One PR per slice, branch `feat/<issue>-<slug>` off `develop`, PR title = commit pattern. Every `gh pr create --body "..."` below uses this body, filled in:

  ```
  Closes #<issue>. Spec: docs/superpowers/specs/2026-09-09-tui-redesign-design.md, section "<name>".

  ## What
  <two to four sentences: the visible change and the seam it went through>

  ## Why
  <one or two sentences from the spec's context>

  ## Verified
  Gate green locally (gofmt, vet, golangci-lint, go test -race, coverage ≥ 90%). Smoke: <sizes run and what was read>.

  🤖 Generated with [Claude Code](https://claude.com/claude-code)

  https://claude.ai/code/session_01Bkb9okzVr3Tb36jQHpk575
  ```
- Board: move the issue to In Progress when starting a slice and the PR to Review when opened: `gh project item-add 13 --owner WilsonSousajr --url <url> --format json --jq .id`, then `gh project item-edit --project-id PVT_kwHOBTZlyM4BiIw3 --id <item> --field-id PVTSSF_lAHOBTZlyM4BiIw3zhhB38g --single-select-option-id <In Progress 986dbe85 | Review 585b7724>`.
- This plan file is copied to `docs/superpowers/plans/2026-09-09-m8-surface-redesign.md` and committed on the first slice's branch, as earlier milestone plans were, so the plan travels with the repo.
- Palette stays ANSI-256 indices; only the lane fade and the meter ramp are truecolor blends (#154).
- Keep existing comments; write WHY comments referencing issue numbers.
- Never touch the real `~/.omatty` or `~/.claude` in tests or smokes; the smoke uses a scratch short-path HOME with `testdata/fake-claude` on PATH.
- The footer's exit key (`<leader> q quit`) is first at every width (#28, #30); no task may move it.
- `ui` is the only package importing bubbletea; only `vcs` runs git.

## Slice order and PR plan

| Slice | Issue | Branch | Blocked by |
|---|---|---|---|
| A. Frame | #174 | `feat/174-frame` | — |
| B. Colour rule | #175 | `feat/175-colour-rule` | — |
| C. Cards | #176 | `feat/176-cards` | A, B merged |
| D. Header | #177 | `feat/177-header` | A, B, PR #171 merged |
| E. Footer | #178 | `feat/178-footer` | B merged |
| F. Diffstat | #180 | `feat/180-diffstat` | C, D merged |
| G. Close-out | #179 | `docs/179-m8-closeout` | all above merged |

Build strictly in this order when working alone: A, B, C, D, E, F, G. Each slice rebases on `develop` after its blockers merge. F also needs D merged (it fills the header's branch), which is a blocker #180 should list.

**Test conventions used throughout** (all exist today): tests are `package ui_test`; a model comes from `modelWithFakes(t)` (three sessions s1/s2/s3 over projects omatty/api-svc, `twoProjectState()`), `baseDeps(st, terms)`, `fakeTermsFor(st)`; a frame is `m.View().Content` split on `\n`; `stripSGR(s)` (meter_test.go) removes colour; `status(m, id, kind, at)` sends a StatusMsg and runs the commands; `statusDeliver` feeds results back; `deliver(m, cmd)` runs a command tree (never on a tick); `leader(m, key('d'))` presses ctrl+o then a key; `fixedNow` is the frozen clock. `export_test.go` (package `ui`) re-exports internals for assertions; add to it rather than duplicating tables in tests.

**Slice summaries for a reader in a hurry.** A: `layout.go` constants + a new `hairline.go` replacing `panebox.go`; `View` composes header, rule, columns, footer. B: `style.go` palette and status tables; `ramp.go` meter endpoints. C: `sidebar.go` line-based window + `sidebarclick.go` inverse + `render.go` cards. D: header row content and collapse steps + modal names. E: footer facts. F: `vcs.Shortstat` → `review.Source.Stat` → `ui.RepoStatFunc` → poll + card line two + header branch. G: smoke, ROADMAP, AGENTS.md.

---

## Slice A — Frame (#174), branch `feat/174-frame`

What changes: `layout.go` constants; `panebox.go` is deleted and `hairline.go` created; `View` composes header row, rule, hairline-joined columns and footer; `renderSidebar`/`renderTerminal`/`renderReview` return bare `fitBlock` bodies; two chrome literals (`-2`) become one helper. Colours keep today's values (39/240) under the new names `colorAccent`/`colorHairline` so slice B only changes values.

### Task A1: Geometry constants

**Files:**
- Modify: `internal/ui/layout.go`
- Modify: `internal/ui/style.go` (rename `colorFocused`→`colorAccent`, `colorBlurred`→`colorHairline`; values unchanged; delete `paneBox`, `borderColor`, `borderStyle`)
- Modify: `internal/ui/pan.go:41`, `internal/ui/wheel.go` (`overSidebar`)
- Test: `internal/ui/layout_test.go`, `internal/ui/caret_test.go`, `internal/ui/sidebarclick_test.go`, `internal/ui/quit_test.go`, `internal/ui/wheel_test.go`, `internal/ui/wheelpan_test.go`, `internal/ui/model_test.go`

**Interfaces:**
- Produces: `headerRows = 1`, `ruleRows = 1`; `PaneSize(w,h,open) = (w - SidebarWidth - ReviewWidth, h - headerRows - ruleRows - footerRows)`; `PaneOrigin() = (SidebarWidth, headerRows+ruleRows)`; `sidebarTop() = headerRows + ruleRows + sidebarHeaderRows` (= 2); `reviewContentWidth(width int) int = ReviewWidth(width, true) - 1`; `sidebarContentCols = SidebarWidth - 1`.

- [ ] **Step 1: Change the layout tests to the new numbers**

In `layout_test.go` find the literal expectations for `PaneSize`, `PaneOrigin` and `PTYSize` (the tests near the top named for #35/#75) and change them: `PaneSize(120, 40, false)` → `(92, 37)`; `PaneOrigin()` → `(28, 2)`; `PTYSize(120, 40, false)` → `(92, 37)`; at 100x30 → `(72, 27)`; with the review open at 100 wide → `100 - 28 - 28 = 44`. Add:

```go
// The two border rows became the header row and the rule, so the pane keeps
// its rows; the two border columns are gone, so it gains two (#174).
func TestPaneSize_TheFrameSpendsTwoRowsAndNoColumns_issue174(t *testing.T) {
	w, h := ui.PaneSize(80, 24, false)
	if w != 80-ui.SidebarWidth || h != 24-3 {
		t.Errorf("PaneSize(80,24) = %dx%d, want %dx21", w, h, 80-ui.SidebarWidth)
	}
	if x, y := ui.PaneOrigin(); x != ui.SidebarWidth || y != 2 {
		t.Errorf("PaneOrigin() = (%d,%d), want (%d,2)", x, y, ui.SidebarWidth)
	}
}
```

`caret_test.go`: the pinned literals `ui.SidebarWidth+1+7, 1+3` → `ui.SidebarWidth+7, 2+3` and `PaneOrigin() == (SidebarWidth+1, 1)` → `(SidebarWidth, 2)`; update the comment to say the pane has no left border and sits under the header row and the rule (#174).
`sidebarclick_test.go`: `ui.SidebarTop() != 1` → `!= 2`, message `want 2 (the header row and the rule, #174)`.
`quit_test.go` (three sites around lines 138–181): `ui.SidebarWidth + fakes["s1"].Width + 2 == 100` → `ui.SidebarWidth + fakes["s1"].Width == 100`; the review variant → `ui.SidebarWidth + fakes["s1"].Width + ui.ReviewWidth(100, true) == 100`.
`wheel_test.go` `overReview` and `wheelpan_test.go` `overReviewAt`: `+ 2` → `+ 1` (the review's first content column is past its own hairline).
`model_test.go` `TestModel_MovingTheCursorResizesTheNewlySelectedTerminal_issue73`: `90x37` → `92x37` in both the assertion and the comment. Then `grep -n '90\b\|29\b' internal/ui/*_test.go` for any other literal that encoded the border columns and fix the same way.

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/ui -run 'PaneSize|PaneOrigin|PTYSize|Caret|SidebarTop|issue73' -v`
Expected: FAIL on the new numbers.

- [ ] **Step 3: Rewrite layout.go**

Replace the `borderRows/borderCols/titleRows` const block with (keeping the `titleRows` comment verbatim):

```go
// headerRows is the breadcrumb row across the top of the window and ruleRows
// the rule beneath it (#174). Together they replaced the pane box's two
// border rows, so the pane keeps exactly the rows it had; the two border
// columns went to the pane. Every derivation of the pane's geometry goes
// through them, so the caret, the wheel target, the click hit-test and the
// emulator's size move together (#106, #107, #45).
const (
	headerRows = 1
	ruleRows   = 1
	// titleRows is 0: ... (existing comment, unchanged)
	titleRows = 0
)

// sidebarContentCols is what the sidebar draws in: its outer width minus the
// hairline on its right, which belongs to no column (#174).
const sidebarContentCols = SidebarWidth - 1
```

`sidebarHeaderRows` stays 0 with its comment. `sidebarTop`:

```go
// sidebarTop is the window row the first sidebar row is drawn at: under the
// header row and the rule, the exact inverse of what View prepends (#45).
func sidebarTop() int { return headerRows + ruleRows + sidebarHeaderRows }
```

`PaneSize`: `termW = width - SidebarWidth - ReviewWidth(width, reviewOpen)`; `termH = height - headerRows - ruleRows - footerRows`; doc example `// 92, 37`; rewrite its comment: no box spends columns now; the window minus the sidebar, minus the review column, is the pane; rows are the window minus the header row, the rule and the footer. `PaneOrigin`: `return SidebarWidth, headerRows + ruleRows + titleRows`, example `// 28, 2`. `PTYSize` example `// 92, 36`→`// 92, 37` (titleRows is 0). Add:

```go
// reviewContentWidth is the review column's content: its outer width minus
// its own left hairline. Named once because the renderer and the pan clamp
// both need it and a literal in each drifted before (#94, #174).
func reviewContentWidth(width int) int { return ReviewWidth(width, true) - 1 }
```

Update `ReviewWidth`'s comment: "outer width, its left hairline included".

`pan.go:41`: `w := reviewContentWidth(m.width)`. `wheel.go`:

```go
// overSidebar reports whether a window column falls inside the sidebar's
// content. The hairline on its right belongs to no one: a click there does
// nothing (#45, #174).
func (m *Model) overSidebar(winX int) bool { return winX >= 0 && winX < sidebarContentCols }
```

`style.go`: rename the two colours (values unchanged) and update the comments they carry: `colorAccent = lipgloss.Color("39") // focus: the keyboard owner's hairline, the rail` and `colorHairline = lipgloss.Color("240") // every other divider`; delete `paneBox`, `borderColor`, `borderStyle`; `headerStyle` reads `colorAccent`. Fix compile errors by name only — the callers are rewritten in A2.

- [ ] **Step 4: Do not run yet** — the package will not compile until A2 replaces `titledBox`. Commit A1 and A2 together.

### Task A2: `hairline.go` and the composed frame

**Files:**
- Create: `internal/ui/hairline.go`
- Delete: `internal/ui/panebox.go` (move `clip` to `render.go` first; it is still used by A3's age column and D's header)
- Modify: `internal/ui/render.go` (`View`, `renderSidebar`, `renderTerminal`, delete `terminalTitle`'s call site from the box, keep the function for the header), `internal/ui/reviewview.go` (`renderReview`)
- Modify: `internal/ui/export_test.go` (delete `TopRule`; add `HairlineCell`, `HeaderRow`, `RuleRow`; `RowOf` uses `sidebarContentCols`)
- Test: create `internal/ui/frame_test.go`; edit `layout_test.go` (delete `TestTopRule_IsExactlyTheBoxWidth_issue128`), `render_test.go` (`SidebarWidth-2` → `SidebarWidth-1`)

**Interfaces:**
- Produces: `type keyboardEdge int` (`edgeNone`, `edgePane`, `edgeReview`); `(m *Model) keyboardEdge() keyboardEdge`; `type segment struct{ title string; width int; owns bool }`; `headerRow(segs []segment) string`; `ruleRow(segs []segment) string`; `hairlineColumn(accent bool, h int) string`; `(m *Model) paneTitle(now time.Time) string` (slice D replaces its body); `renderTerminal(w, h int) string` (no `now`); `renderSidebar(rows int, now time.Time) string`.

- [ ] **Step 1: Write the failing tests** (`frame_test.go`)

```go
package ui_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

func frameLines(m *ui.Model) []string { return strings.Split(m.View().Content, "\n") }

// footerOf is the frame's last line; shared with the header and footer tests
// of later slices.
func footerOf(m *ui.Model) string {
	lines := strings.Split(strings.TrimRight(m.View().Content, "\n"), "\n")
	return lines[len(lines)-1]
}

func TestFrame_DrawsNoBoxAnywhere_issue174(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	leader(m, key('d'))
	if view := m.View().Content; strings.ContainsAny(view, "╭╮╰╯") {
		t.Errorf("the frame still draws a rounded box:\n%s", view)
	}
}

// Line 0 is the header row, line 1 the rule with a joint under each
// hairline, and every body line has the hairline at the sidebar's edge.
func TestFrame_HeaderRuleAndHairlinesLineUp_issue174(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	lines := frameLines(m)

	head, rule := []rune(stripSGR(lines[0])), stripSGR(lines[1])
	if !strings.HasPrefix(string(head), " projects") || head[ui.SidebarWidth-1] != '│' {
		t.Errorf("header row = %q, want the sidebar title and a hairline at column %d", string(head), ui.SidebarWidth-1)
	}
	if want := strings.Repeat("─", ui.SidebarWidth-1) + "┼" + strings.Repeat("─", 100-ui.SidebarWidth); rule != want {
		t.Errorf("rule = %q\nwant   %q", rule, want)
	}
	for y := 2; y < len(lines)-1; y++ {
		if r := []rune(stripSGR(lines[y])); r[ui.SidebarWidth-1] != '│' {
			t.Errorf("line %d has %q at the sidebar's edge, want the hairline", y, string(r[ui.SidebarWidth-1]))
		}
	}
}

// The accent hairline stands on the left edge of whatever owns the keyboard.
func TestFrame_TheAccentHairlineFollowsTheKeyboardOwner_issue174(t *testing.T) {
	m, _, _ := modelWithDiff(t)
	body := func() string { return frameLines(m)[3] }
	accent, plain := ui.HairlineCell(true), ui.HairlineCell(false)

	if l := body(); strings.Count(l, accent) != 1 || strings.Contains(l, plain) {
		t.Errorf("terminal focused: %q, want one accent hairline and no plain one", stripSGR(l))
	}
	leader(m, key('d')) // the review column opens focused
	if l := body(); strings.Count(l, accent) != 1 || strings.Index(l, plain) > strings.Index(l, accent) {
		t.Errorf("review focused: %q, want the accent on the pane/review hairline, the plain one on the left", stripSGR(l))
	}
	leader(m, key('?')) // a modal owns the keys
	if l := body(); strings.Index(l, accent) > strings.Index(l, plain) {
		t.Errorf("modal open: %q, want the accent on the pane's hairline", stripSGR(l))
	}
}

func TestFrame_NothingOwnsTheKeysOnAnEmptyRegistry_issue174(t *testing.T) {
	m := ui.NewModel(baseDeps(emptyState(), fakeTermsFor(emptyState())))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if strings.Contains(m.View().Content, ui.HairlineCell(true)) {
		t.Error("an empty registry draws an accent hairline, but nothing owns the keyboard")
	}
}

// The frame promise of #35, at the three smoke sizes, with the column open
// and closed: every line is exactly the window.
func TestFrame_EveryLineIsExactlyTheWindow_issue174(t *testing.T) {
	for _, size := range [][2]int{{80, 24}, {100, 30}, {120, 32}} {
		for _, open := range []bool{false, true} {
			m, _, _ := modelWithDiff(t)
			m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
			if open {
				leader(m, key('d'))
			}
			lines := frameLines(m)
			if len(lines) != size[1] {
				t.Errorf("%v open=%v: %d lines, want %d", size, open, len(lines), size[1])
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w != size[0] {
					t.Errorf("%v open=%v line %d is %d cells: %q", size, open, i, w, stripSGR(l))
				}
			}
		}
	}
}

func TestHeaderRow_AndRuleRow_AreExactlyTheirWidths_issue174(t *testing.T) {
	for _, tt := range []struct {
		titles []string
		widths []int
	}{
		{[]string{"projects", "parser-fix"}, []int{27, 72}},
		{[]string{"projects", strings.Repeat("x", 90), "diff"}, []int{27, 44, 27}},
		{[]string{"", "日本語のタイトル", ""}, []int{27, 12, 5}},
	} {
		sum := len(tt.widths) - 1
		for _, w := range tt.widths {
			sum += w
		}
		if got := ui.HeaderRow(tt.titles, tt.widths, 1); lipgloss.Width(got) != sum {
			t.Errorf("HeaderRow(%v) is %d cells, want %d: %q", tt.widths, lipgloss.Width(got), sum, stripSGR(got))
		}
		if got := ui.RuleRow(tt.widths); lipgloss.Width(got) != sum || strings.Count(stripSGR(got), "┼") != len(tt.widths)-1 {
			t.Errorf("RuleRow(%v) = %q, want %d cells with a joint per hairline", tt.widths, stripSGR(got), sum)
		}
	}
}
```

Exports:

```go
// HairlineCell is one rendered hairline cell, accent or plain, so a test can
// find which edge the accent stands on (#174). HeaderRow and RuleRow build
// the two chrome lines from titles and widths; owner is the index of the
// segment that owns the keys, -1 for none.
func HairlineCell(accent bool) string { return hairlineStyle(accent).Render(hairline) }
func HeaderRow(titles []string, widths []int, owner int) string {
	segs := make([]segment, len(titles))
	for i := range titles {
		segs[i] = segment{title: titles[i], width: widths[i], owns: i == owner}
	}
	return headerRow(segs)
}
func RuleRow(widths []int) string {
	segs := make([]segment, len(widths))
	for i, w := range widths {
		segs[i] = segment{width: w}
	}
	return ruleRow(segs)
}
```

Delete `TopRule` from export_test.go and its test from layout_test.go (the rule it measured no longer exists; say so in the commit message). `RowOf`: `m.renderRow(row, sidebarContentCols)`; render_test's `ui.SidebarWidth-2` → `ui.SidebarWidth-1` (both occurrences in `TestRenderRow_TheAgeMovedToTheFocusedPanesRule_issue128`).

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/ui -run 'Frame_|HeaderRow' -v`
Expected: compile errors (`undefined: hairlineStyle`, `titledBox`).

- [ ] **Step 3: Create hairline.go**

```go
// The frame's chrome (#174): a header row, a rule and hairline columns where
// three rounded boxes used to be. The box around the session pane framed a
// program that draws its own rounded prompt box, and its four border columns
// were 5% of an 80-column window. One hairline per seam gives those columns
// to claude; the header row carries what the boxes' top rules carried.
//
// One rule for the whole screen: an accent vertical line stands on the left
// edge of whatever owns the keyboard. Nothing else on the screen is accent.

package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// hairline, ruleDash and ruleJoint are the three glyphs the chrome is drawn
// with. Box Drawing, one cell wide in every width table.
const hairline, ruleDash, ruleJoint = "│", "─", "┼"

// keyboardEdge names the hairline the accent stands on.
type keyboardEdge int

const (
	edgeNone   keyboardEdge = iota // nothing to type into
	edgePane                       // a terminal, or a modal drawn in its pane
	edgeReview                     // the review column
)

// keyboardEdge is the seam whose right-hand column owns the keys now. The
// order is the routing order: a modal takes the keys from everything, the
// review column takes them from the terminal (#95, #21).
func (m *Model) keyboardEdge() keyboardEdge {
	if m.modalOpen() {
		return edgePane
	}
	if m.reviewOwnsKeys() {
		return edgeReview
	}
	if m.focusedTerminal() != nil {
		return edgePane
	}
	return edgeNone
}

// hairlineStyle is the accent on the keyboard owner's edge, the hairline
// grey on every other seam.
func hairlineStyle(accent bool) lipgloss.Style {
	if accent {
		return lipgloss.NewStyle().Foreground(colorAccent)
	}
	return lipgloss.NewStyle().Foreground(colorHairline)
}

// hairlineColumn is one hairline h rows tall, a column for JoinHorizontal.
// Each cell is rendered on its own so a line of the frame carries the cell's
// SGR whole, which is also what lets a test find it.
func hairlineColumn(accent bool, h int) string {
	cell := hairlineStyle(accent).Render(hairline)
	return strings.TrimSuffix(strings.Repeat(cell+"\n", h), "\n")
}

// segment is one column's share of the header row.
type segment struct {
	title string
	width int
	owns  bool // the column owns the keyboard: its title is ink, not muted
}

// headerRow lays the segments across the window with a hairline cell between
// each pair. A title is padded and cut to its column, so the row is exactly
// the frame's width whatever a title says (#35). The hairline cells here are
// never accent: the accent runs from the rule down, not through the chrome.
func headerRow(segs []segment) string {
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		parts = append(parts, segmentStyle(s.owns).Render(fitLine(" "+s.title, s.width)))
	}
	return strings.Join(parts, hairlineStyle(false).Render(hairline))
}

// segmentStyle is ink and bold for the keyboard owner's title, muted for the
// rest, so the header row says where a keystroke lands as the hairline does.
func segmentStyle(owns bool) lipgloss.Style {
	if owns {
		return headerStyle
	}
	return mutedStyle
}

// ruleRow is the line under the header row: dashes across every column and a
// joint under each hairline.
func ruleRow(segs []segment) string {
	parts := make([]string, 0, len(segs))
	for _, s := range segs {
		parts = append(parts, strings.Repeat(ruleDash, s.width))
	}
	return hairlineStyle(false).Render(strings.Join(parts, ruleJoint))
}
```

- [ ] **Step 4: Rewrite View, the three renderers, and delete panebox.go**

In `render.go`, `View` becomes:

```go
// View lays the header row and the rule over the body, and the keymap under
// it (#35, #174). Every column is sized exactly before it is joined, so the
// frame never exceeds the window.
func (m *Model) View() tea.View {
	v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, m.frame(), m.renderFooter()))
	v.AltScreen = true
	v.ReportFocus = true // so FocusMsg/BlurMsg drive notifications
	// (keep the existing mouse-mode comment verbatim)
	v.MouseMode = tea.MouseModeCellMotion
	v.Cursor = m.paneCursor()
	return v
}

// frame is the header row, the rule and the body: the sidebar, the pane and,
// when open, the review column, a hairline between each pair (#174).
func (m *Model) frame() string {
	termW, termH := PaneSize(m.width, m.height, m.review.Open)
	now := m.clock() // once per frame, so every age is measured against the same instant
	cols, segs := m.bodyColumns(termW, termH, now)
	body := lipgloss.JoinHorizontal(lipgloss.Top, cols...)
	return lipgloss.JoinVertical(lipgloss.Left, headerRow(segs), ruleRow(segs), body)
}

// bodyColumns is every column with the hairline before each one but the
// first, and one header segment per column. The accent hairline and the ink
// title go to the same column: the one keyboardEdge names.
func (m *Model) bodyColumns(termW, termH int, now time.Time) ([]string, []segment) {
	edge := m.keyboardEdge()
	cols := []string{m.renderSidebar(termH, now), hairlineColumn(edge == edgePane, termH), m.renderTerminal(termW, termH)}
	segs := []segment{
		{title: "projects", width: sidebarContentCols},
		{title: m.paneTitle(now), width: termW, owns: edge == edgePane},
	}
	if !m.review.Open {
		return cols, segs
	}
	rw := reviewContentWidth(m.width)
	cols = append(cols, hairlineColumn(edge == edgeReview, termH), m.renderReview(rw, termH))
	return cols, append(segs, segment{title: m.reviewTitle(), width: rw, owns: edge == edgeReview})
}

// paneTitle is the pane's header segment: the focused session's title line,
// nothing for a modal or an empty pane. Slice #177 makes it the breadcrumb.
func (m *Model) paneTitle(now time.Time) string {
	if m.modalOpen() || m.focusedTerminal() == nil {
		return ""
	}
	return m.terminalTitle(now)
}
```

`renderSidebar(rows int, now time.Time)`: body `lines` as today with `inner := sidebarContentCols`; return `fitBlock(lines, inner, rows)`; rewrite its comment (no box; the "projects" title is the header row's). `now` is unused until slice C; name the parameter `_ time.Time` for now with a comment `// the cards' ages, #176`. `renderTerminal(w, h int)`: each of the three arms returns the `fitBlock(...)` alone; keep the arm comments; delete the `now` parameter. `renderReview(w, h int)` in reviewview.go: `return fitBlock(lines, w, h)`; keep the comment about the modal check but move its point into `keyboardEdge`'s comment (it is now decided there). Delete `panebox.go` after moving `clip` (with its comment) to `render.go` beside `fitLine`.

- [ ] **Step 5: Run everything and the gate**

Run: `go test ./internal/ui -race` → fix any remaining test that pinned the old chrome (the grep in A1 step 1; `render_test.go`'s frame checks; `layout_test.go`'s `│` test survives since the hairline is `│`). Then the full gate. `View` is now 8 lines, `frame` 6, `bodyColumns` 13.

- [ ] **Step 6: Commit and open the PR**

```bash
git add -A internal/ui
git commit -m "feat(#174): a header row, a rule and hairline columns replace the three boxes"
git push -u origin feat/174-frame && gh pr create --base develop --title "feat(#174): frame" --body "..."
```
Board: PR to Review, #174 to Review. Smoke at 120x32 and 80x24 (recipe in Task F4 step 4, no repo needed) and read: no box, `┼` under the hairline, the accent on the pane's edge, the frame exactly the window.

---

## Slice B — Colour rule (#175), branch `feat/175-colour-rule`

Assumes slice A is merged (no `paneBox`/`borderColor` left; `hairline.go` reads `colorAccent`/`colorHairline` by the names below, so A must already use those two names — see Task A1).

### Task B1: Palette constants and the status tables

**Files:**
- Modify: `internal/ui/style.go` (the `var (...)` palette block at the top, `statusColors`, `headerStyle`/`footerStyle`/`errorStyle`, `statusGlyphs`, the diff colour block)
- Modify: `internal/ui/export_test.go` (add `AccentColor`, `AmberColor`, `TextColor`, `AllStatuses`)
- Test: `internal/ui/style_test.go`

**Interfaces:**
- Produces: `colorInk, colorText, colorMuted, colorHairline, colorAccent, colorAmber, colorGreen, colorRed lipgloss.Color`; `amberStyle lipgloss.Style`; `statusColors` and `statusGlyphs` covering all seven statuses; exports `ui.AccentColor()`, `ui.AmberColor()`, `ui.TextColor()`, `ui.AllStatuses() []watcher.Status`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/ui/style_test.go`:

```go
// One hue, one meaning (#175): amber is bound to waiting and to nothing
// else, and the accent means focus alone, so it appears in no status.
func TestStatusColors_AmberMeansWaitingAloneAndAccentMeansFocusAlone_issue175(t *testing.T) {
	for _, s := range ui.AllStatuses() {
		isAmber := sameRGB(ui.StatusColor(s), ui.AmberColor())
		if isAmber != (s == watcher.StatusWaiting) {
			t.Errorf("status %s amber=%v; amber must mean waiting and only waiting", s, isAmber)
		}
		if sameRGB(ui.StatusColor(s), ui.AccentColor()) {
			t.Errorf("status %s is drawn in the accent, which means focus alone", s)
		}
	}
}

// Working states earn no colour: the lane's height already says how busy.
func TestStatusColors_WorkingStatesAreTextColoured_issue175(t *testing.T) {
	for _, s := range []watcher.Status{watcher.StatusThinking, watcher.StatusTool} {
		if !sameRGB(ui.StatusColor(s), ui.TextColor()) {
			t.Errorf("status %s = %v, want the text colour", s, ui.StatusColor(s))
		}
	}
}

// Every status has a glyph and a colour, so a new status cannot render "-".
func TestStatusTables_CoverEveryStatus_issue175(t *testing.T) {
	for _, s := range ui.AllStatuses() {
		if ui.StatusColor(s) == nil {
			t.Errorf("status %s has no colour", s)
		}
	}
	if len(ui.StatusGlyphs()) != len(ui.AllStatuses()) {
		t.Errorf("%d glyphs for %d statuses", len(ui.StatusGlyphs()), len(ui.AllStatuses()))
	}
}
```

Add the import `"github.com/WilsonSousajr/omatty/internal/watcher"` to style_test.go. `sameRGB` already exists in ramp_test.go (same package).

Change the existing `TestStatusGlyphs_AreOneCellWide_issue128`: replace the `slices.Contains(ui.StatusGlyphs(), "●")` check with `slices.Contains(ui.StatusGlyphs(), "◐")` and message `"the working glyph ◐ is missing"`; the width loop stays and now also covers the new glyphs.

Add to `export_test.go`:

```go
// AccentColor, AmberColor and TextColor are the palette entries the colour
// rule binds (#175); AllStatuses is every status the tables must cover.
func AccentColor() color.Color { return colorAccent }
func AmberColor() color.Color  { return colorAmber }
func TextColor() color.Color   { return colorText }
func AllStatuses() []watcher.Status {
	return []watcher.Status{watcher.StatusIdle, watcher.StatusThinking, watcher.StatusTool,
		watcher.StatusWaiting, watcher.StatusDone, watcher.StatusError, watcher.StatusExited}
}
```

- [ ] **Step 2: Run and watch them fail**

Run: `go test ./internal/ui -run 'Status' -v`
Expected: compile error, `undefined: ui.AllStatuses` (and the rest).

- [ ] **Step 3: Rewrite the palette in style.go**

Replace the palette `var (...)` block (the four `colorFocused/colorBlurred/colorMuted/colorFooter` lines) with, keeping the comment above it and appending one paragraph to it:

```go
// M8 (#175) made the palette eight indices and the rule one hue, one
// meaning: working states are text, amber is waiting and nothing else,
// green is done, red is error, muted is true-but-not-actionable, and the
// accent means focus alone. 75 replaced 39 as the accent because it is the
// nearest index to monocode's blue that is not neon.
var (
	colorInk      = lipgloss.Color("253") // the focused header segment, the selected title
	colorText     = lipgloss.Color("250") // titles, branches, working glyphs
	colorMuted    = lipgloss.Color("245") // headers, ages, counts, keymap
	colorHairline = lipgloss.Color("238") // every divider that is not focused
	colorAccent   = lipgloss.Color("75")  // the focused hairline, the rail
	colorAmber    = lipgloss.Color("214") // waiting, and nothing else
	colorGreen    = lipgloss.Color("78")  // done, +added
	colorRed      = lipgloss.Color("203") // error, −removed
)
```

Replace `statusColors` with:

```go
// statusColors is the only place a status becomes a colour. Working states
// are text: working is the default, and the lane's height already says how
// busy. Amber goes to waiting alone, so the newest amber cell in any lane
// still answers "which of these needs me" (#175).
var statusColors = map[watcher.Status]color.Color{
	watcher.StatusIdle: colorMuted, watcher.StatusThinking: colorText, watcher.StatusTool: colorText,
	watcher.StatusWaiting: colorAmber, watcher.StatusDone: colorGreen, watcher.StatusError: colorRed,
	watcher.StatusExited: colorMuted,
}
```

Replace the `headerStyle/footerStyle/errorStyle/mutedStyle` block with:

```go
var (
	headerStyle = lipgloss.NewStyle().Foreground(colorInk).Bold(true)
	footerStyle = lipgloss.NewStyle().Foreground(colorMuted)
	errorStyle  = lipgloss.NewStyle().Foreground(colorRed).Bold(true)
	mutedStyle  = lipgloss.NewStyle().Foreground(colorMuted)
	textStyle   = lipgloss.NewStyle().Foreground(colorText)
	amberStyle  = lipgloss.NewStyle().Foreground(colorAmber)
	accentStyle = lipgloss.NewStyle().Foreground(colorAccent)
)
```

Replace `statusGlyphs` and its comment with:

```go
// statusGlyphs pairs each status with its one-column marker (#175). The
// filled dot is waiting, the loudest glyph for the loudest state; the shapes
// differ enough to read with colour off. All are Geometric Shapes or
// Mathematical Operators, so no font draws them two cells wide - the gear and
// the pause sign #128 warned about are gone. ○ ◐ ◆ ● are East Asian
// Ambiguous like the lane cells and the rail: RUNEWIDTH_EASTASIAN=1 doubles
// all of them or none.
var statusGlyphs = map[watcher.Status]string{
	watcher.StatusIdle: "○", watcher.StatusThinking: "◐", watcher.StatusTool: "◆",
	watcher.StatusWaiting: "●", watcher.StatusDone: "✓", watcher.StatusError: "✕",
	watcher.StatusExited: "∅",
}
```

In the diff colour block use the names: `addedStyle` → `Foreground(colorGreen)`, `removedStyle` → `Foreground(colorRed)`, `commentStyle` → `Foreground(colorAmber)`. Keep the comment; add one sentence: `A queued comment is amber because it is the operator's own pending move, the one meaning the rule gives amber (#175).`

Then fix every reference to the removed names: `grep -n 'colorFocused\|colorBlurred\|colorFooter' internal/ui/*.go`. After slice A only `hairline.go` should reference `colorAccent`/`colorHairline` already; anything else found is renamed here.

- [ ] **Step 4: Run the ui tests**

Run: `go test ./internal/ui -race`
Expected: the new style tests pass; `status_test.go` fails on `⏸`/`●` (fixed in B2); `ramp_test.go` fails on the meter endpoints (fixed in B3).

- [ ] **Step 5: Commit**

```bash
git add internal/ui/style.go internal/ui/style_test.go internal/ui/export_test.go
git commit -m "feat(#175): eight-index palette and one hue per status"
```

### Task B2: Move the glyph assertions to the new glyphs

**Files:**
- Modify: `internal/ui/status_test.go` (three assertions), any other test found by `grep -n '⏸\|⚙\|✗\|"●"' internal/ui/*_test.go`

- [ ] **Step 1: Update the tests to the new glyph table**

In `status_test.go`:
- `TestModel_StatusMsgUpdatesTheGlyph_issue20`: `"⏸"` → `"●"`, comment `the waiting glyph ● must sit on its row (#175)`.
- `TestModel_OlderStatusMsgIsIgnored_issue20`: `!strings.Contains(got, "⏸") || strings.Contains(got, "●")` → `!strings.Contains(got, "●") || strings.Contains(got, "◐")`.
- `TestNewModel_DefaultsTheOptionalDeps_issue76`: `"⏸"` → `"●"`.

Run the grep; apply the same mapping anywhere else (thinking `●`→`◐`, tool `⚙`→`◆`, waiting `⏸`→`●`, error `✗`→`✕`).

- [ ] **Step 2: Run**

Run: `go test ./internal/ui -race -run 'Status|Default'`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/ui/*_test.go
git commit -m "test(#175): the status tests read the new glyphs"
```

### Task B3: The meter's endpoints are named constants

**Files:**
- Modify: `internal/ui/ramp.go` (`meterCellColor`)
- Modify: `internal/ui/export_test.go` (`MeterRamp`)
- Test: `internal/ui/ramp_test.go` (`TestMeterCellColor_RampsFromAmberToGreen_issue154`)

- [ ] **Step 1: Change the test to read the endpoints from the constants**

In ramp_test.go replace the first assertion of `TestMeterCellColor_RampsFromAmberToGreen_issue154` with:

```go
	warm, cool := ui.MeterRamp()
	first, last := ui.MeterCellColor(0), ui.MeterCellColor(ui.MeterCells()-1)
	if !sameRGB(first, warm) || !sameRGB(last, cool) {
		t.Errorf("meter ends = %v / %v, want the ramp's amber and green", first, last)
	}
	if !sameRGB(warm, ui.AmberColor()) {
		t.Error("the meter no longer starts at amber")
	}
```

Add to export_test.go: `func MeterRamp() (warm, cool color.Color) { return rampWarm, rampCool }`.

- [ ] **Step 2: Run to see it fail**

Run: `go test ./internal/ui -run MeterCellColor`
Expected: `undefined: ui.MeterRamp`.

- [ ] **Step 3: Implement**

In ramp.go, above `meterCellColor`:

```go
// rampWarm and rampCool are the meter's two ends. Named rather than looked up
// through the status map, so changing what a status means cannot recolour the
// meter (#175).
var rampWarm, rampCool = colorAmber, colorGreen
```

and the body becomes `return blend(rampWarm, rampCool, t)`; update its comment to `warming from amber to green across the bar`.

- [ ] **Step 4: Run the whole gate**

```bash
gofmt -l . && go vet ./... && golangci-lint run && go test ./... -race && ./scripts/check-coverage.sh 90
```
Expected: all clean.

- [ ] **Step 5: Commit and open the PR**

```bash
git add internal/ui/ramp.go internal/ui/ramp_test.go internal/ui/export_test.go
git commit -m "feat(#175): the meter ramp names its own endpoints"
git push -u origin feat/175-colour-rule
gh pr create --base develop --title "feat(#175): one hue, one meaning" --body "..."   # body: what/why/verified, the trailer
```
Add the PR to the board in Review; move #175 to Review.

---

## Slice C — Cards (#176), branch `feat/176-cards`

Assumes A and B merged (`sidebarContentCols`, `accentStyle`, `textStyle`, `headerStyle` = ink bold).

### Task C1: The sidebar counts lines

**Files:**
- Modify: `internal/ui/sidebar.go` (`Window`, `revealHeader` unchanged, add `rowHeight`, `scrollTo`, `linesBetween`, `rowAtLine`)
- Modify: `internal/ui/export_test.go` (`RowHeight`, `(*Sidebar).RowAtLine`)
- Test: `internal/ui/sidebar_test.go`

**Interfaces:**
- Produces: `const cardLines = 2`; `rowHeight(r Row) int`; `(s *Sidebar) Window(lines int) []Row` (same signature, the argument is now lines); `(s *Sidebar) rowAtLine(line int) (int, bool)`; `Offset()` unchanged (a row index).

- [ ] **Step 1: Change the window tests to line numbers and add the card tests**

Row indices in `sevenProjectState` are unchanged (header of project p at row 3p, its sessions at 3p+1 and 3p+2); what changes is how many fit. Rewrite the five tests:

```go
// Thirty-five lines, ten fit: rows 12..17 are the ten lines that end on the
// cursor's card (header 1 + 2 + 2 + header 1 + 2 + 2).
func TestSidebar_WindowFollowsTheCursor_issue129(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(sevenProjectState(), nil))
	s.SelectByID("p5-s1") // row 17

	win := s.Window(10)

	if len(win) != 6 || s.Offset() != 12 {
		t.Fatalf("Window(10) = %d rows from offset %d, want 6 from 12", len(win), s.Offset())
	}
	found := false
	for _, r := range win {
		found = found || (r.Session != nil && r.Session.ID == "p5-s1")
	}
	if !found {
		t.Errorf("the selected row is not in the window: %+v", win)
	}
}
```

`WindowScrollsBackToZeroAtTheTop`: unchanged in shape (asserts `Offset() == 0`). `WindowIsTheWholeListWhenItFits`: unchanged (`Window(20)` on 8 lines → 5 rows, offset 0). `WindowShrinkingKeepsTheCursorVisible`: cursor p3-s0 (row 10), `Window(20)` fits rows 0..11 (20 lines), then `Window(5)` → offset 8, 3 rows:

```go
	win := s.Window(5)
	if s.Offset() != 8 || len(win) != 3 {
		t.Errorf("Window(5) after shrinking: offset %d len %d, want 8 and 3 (rows 8, 9, 10 are five lines)", s.Offset(), len(win))
	}
```

Add:

```go
func TestRowHeight_ACardIsTwoLinesAndAHeaderOne_issue176(t *testing.T) {
	rows := ui.SidebarRows(twoProjectState(), nil)
	if ui.RowHeight(rows[0]) != 1 || ui.RowHeight(rows[1]) != 2 {
		t.Errorf("heights = %d, %d; want 1 for a header and 2 for a session", ui.RowHeight(rows[0]), ui.RowHeight(rows[1]))
	}
}

// Whatever the cursor and the budget, the selected card is drawn whole and
// the drawn rows never take more lines than fit.
func TestSidebar_WindowNeverSplitsTheSelectedCard_issue176(t *testing.T) {
	rows := ui.SidebarRows(sevenProjectState(), nil)
	for lines := 3; lines <= 12; lines++ {
		for i := range rows {
			s := ui.NewSidebar(rows)
			if rows[i].Session == nil {
				continue
			}
			s.SelectByID(rows[i].Session.ID)
			win, used, seen := s.Window(lines), 0, false
			for _, r := range win {
				used += ui.RowHeight(r)
				seen = seen || (r.Session != nil && r.Session.ID == rows[i].Session.ID)
			}
			if !seen || used > lines {
				t.Errorf("lines=%d cursor=%d: drawn %d lines, selected drawn=%v", lines, i, used, seen)
			}
		}
	}
}

// The click inverse walks the same heights the window did: line 0 is the
// header, lines 1 and 2 are s1's card, 3 and 4 are s2's, 5 is api-svc, 6 and
// 7 are s3's.
func TestSidebar_RowAtLineWalksTheDrawnHeights_issue176(t *testing.T) {
	s := ui.NewSidebar(ui.SidebarRows(twoProjectState(), nil))
	s.Window(20)
	for line, want := range map[int]int{0: 0, 1: 1, 2: 1, 3: 2, 4: 2, 5: 3, 6: 4, 7: 4} {
		if got, ok := s.RowAtLine(line); !ok || got != want {
			t.Errorf("RowAtLine(%d) = %d, %v; want %d", line, got, ok, want)
		}
	}
	for _, line := range []int{-1, 8, 40} {
		if _, ok := s.RowAtLine(line); ok {
			t.Errorf("RowAtLine(%d) = ok, want false past the list", line)
		}
	}
}
```

Exports: `func RowHeight(r Row) int { return rowHeight(r) }` and `func (s *Sidebar) RowAtLine(line int) (int, bool) { return s.rowAtLine(line) }`.

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/ui -run 'Sidebar_Window|RowHeight|RowAtLine' -v`
Expected: `undefined: ui.RowHeight`, then the offsets 8/12 fail.

- [ ] **Step 3: Implement in sidebar.go**

Replace `Window`'s body and add the helpers (keep `Window`'s comment, replacing "when only rows lines fit" with "when only lines fit, in lines: a card is two, a header one (#176)"):

```go
// cardLines is how many lines a session draws: the glyph, title and age, then
// the branch, diffstat and lane (#176). A header draws one.
const cardLines = 2

// rowHeight is the lines a row draws. The window math and the click inverse
// both read it, so the two cannot drift (#45, #129, #176).
func rowHeight(r Row) int {
	if r.Session == nil {
		return 1
	}
	return cardLines
}

func (s *Sidebar) Window(lines int) []Row {
	s.offset = s.revealHeader(s.scrollTo(lines))
	end := s.offset
	for budget := lines; end < len(s.rows) && rowHeight(s.rows[end]) <= budget; end++ {
		budget -= rowHeight(s.rows[end])
	}
	if s.offset >= end {
		return nil
	}
	return s.rows[s.offset:end]
}

// scrollTo is the first row to draw so the cursor's whole card fits in lines,
// moving the offset as little as possible: back to the cursor when it is
// above the window, forward one row at a time until the rows from the offset
// to the cursor fit when it is below. ScrollOffset in lines rather than
// rows, which is why it is not ScrollOffset.
func (s *Sidebar) scrollTo(lines int) int {
	c := max(s.cursor, 0)
	if c >= len(s.rows) {
		return 0
	}
	off := min(s.offset, c)
	for off < c && s.linesBetween(off, c+1) > lines {
		off++
	}
	return off
}

// linesBetween is how many lines rows [from, to) draw.
func (s *Sidebar) linesBetween(from, to int) int {
	n := 0
	for _, r := range s.rows[from:to] {
		n += rowHeight(r)
	}
	return n
}

// rowAtLine is the row drawn line lines below the first drawn one, walking
// the heights Window drew with, so a click on either line of a card lands on
// it (#45, #176). ok is false above the list and past its end.
func (s *Sidebar) rowAtLine(line int) (int, bool) {
	if line < 0 {
		return 0, false
	}
	for i := s.offset; i < len(s.rows); i++ {
		if line < rowHeight(s.rows[i]) {
			return i, true
		}
		line -= rowHeight(s.rows[i])
	}
	return 0, false
}
```

`revealHeader` is unchanged: it fires only when the cursor is the first drawn row, and the header it reveals is one line, which a pane of at least four always has room for.

- [ ] **Step 4: Run**

Run: `go test ./internal/ui -run 'Sidebar' -race` → PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/ui/sidebar.go internal/ui/sidebar_test.go internal/ui/export_test.go
git commit -m "feat(#176): the sidebar window and its inverse count lines, a card being two"
```

### Task C2: Draw the cards, click on either line

**Files:**
- Create: `internal/ui/card.go` (`renderRow` moves here from render.go with `headerMarker`, `rowChrome` deleted)
- Modify: `internal/ui/render.go` (`renderSidebar` appends the slice; `padLeft`), `internal/ui/sidebarclick.go` (`sidebarRowAt`), `internal/ui/lane.go` (comment on `laneCells`)
- Modify: `internal/ui/export_test.go` (`CardOf` replaces `RowOf`)
- Test: create `internal/ui/card_test.go`; edit `sidebarclick_test.go`, `render_test.go`, `layout_test.go`, `reach_test.go`, `archive_test.go`, `meter_test.go`

**Interfaces:**
- Produces: `(m *Model) renderRow(row Row, now time.Time) []string`; `(m *Model) cardMeta(id string) string` (a 16-blank stub; F fills it); `(m *Model) rail(selected bool) string`; constants `cardCols = 27`, `titleCols = 18`, `ageCols = 4`, `metaCols = 16`; export `(m *Model) CardOf(id string) []string`.

- [ ] **Step 1: Write the failing tests** (`card_test.go`)

```go
package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// Line one: rail, glyph, space, 18 columns of title, space, the age in four,
// a blank. Line two: rail, two spaces, 16 columns for the branch and the
// diffstat, a space, the six-cell lane, a blank. Both 27 cells.
func TestCard_HasTheSpecsColumns_issue176(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	status(m, "s1", watcher.TurnEnded, time.Now().Add(-4*time.Minute))

	card := m.CardOf("s1")

	if len(card) != 2 {
		t.Fatalf("a card is %d lines, want 2", len(card))
	}
	one, two := stripSGR(card[0]), stripSGR(card[1])
	if want := "▎✓ main" + strings.Repeat(" ", 14) + "   4m "; one != want {
		t.Errorf("line one = %q\nwant       %q", one, want)
	}
	if want := "▎  " + strings.Repeat(" ", 16) + " " + "     ▂" + " "; two != want {
		t.Errorf("line two = %q\nwant       %q", two, want)
	}
	for i, l := range card {
		if lipgloss.Width(l) != ui.SidebarWidth-1 {
			t.Errorf("line %d is %d cells, want %d", i, lipgloss.Width(l), ui.SidebarWidth-1)
		}
	}
}

func TestCard_TheRailIsAccentOnTheSelectedCardOnly_issue176(t *testing.T) {
	m, _ := modelWithFakes(t)
	rail := ui.Rail()
	if sel := m.CardOf("s1"); !strings.HasPrefix(sel[0], rail) || !strings.HasPrefix(sel[1], rail) {
		t.Errorf("the selected card does not open both lines with the accent rail: %q", sel)
	}
	if other := m.CardOf("s2"); strings.Contains(other[0], rail) || !strings.HasPrefix(stripSGR(other[0]), " ") {
		t.Errorf("an unselected card carries the rail: %q", other)
	}
}

func TestCard_TheTitleIsClippedToEighteenColumns_issue176(t *testing.T) {
	st := twoProjectState()
	st.Sessions[0].Title = "a-title-that-is-far-longer-than-eighteen"
	m := ui.NewModel(baseDeps(st, fakeTermsFor(st)))
	one := stripSGR(m.CardOf("s1")[0])
	if !strings.Contains(one, "a-title-that-is-fa") || strings.Contains(one, "far-l") {
		t.Errorf("line one = %q, want the title cut at 18 columns", one)
	}
}

// A project header is one line: the rail column, then the name, muted; the
// rail only on an empty project the cursor rests on (#158).
func TestCard_AProjectHeaderIsOneLine_issue176(t *testing.T) {
	m := modelWithEmptyProject(t, &recordCreate{})
	lines := frameLines(m)
	if got := stripSGR(lines[2]); !strings.HasPrefix(got, " omatty") {
		t.Errorf("the first body line = %q, want the omatty header with no rail", got)
	}
	leader(m, key(']'))
	if got := stripSGR(m.View().Content); !strings.Contains(got, "▎wstech") {
		t.Errorf("the selected empty project's header carries no rail:\n%s", got)
	}
}
```

Assert the rail's *presence* on SGR-stripped text with the literal `▎`, because the muted name's SGR sits between the rail and the name; assert the rail's *colour* only through `ui.Rail()` as a prefix of a card line, where nothing intervenes.

Exports: `func (m *Model) CardOf(id string) []string` (find the row, `return m.renderRow(row, m.clock())`), `func Rail() string { return accentStyle.Render(rail) }`. Delete `RowOf`.

Existing tests to change:
- `sidebarclick_test.go`: `sidebarRowY(i)` now takes a line: rename to `sidebarLineY(line int) int { return ui.SidebarTop() + line }`. s3 is lines 6 and 7 → test both; s2 is line 3 (and 4); headers are lines 0 and 5; below the list line 12. The scrolled test at 80x20 (17 body lines) after 13 `j` presses (cursor row 20): the offset is row 11, so line 3 is `p4-s0`; change the expected id and the comment to "rows 11..20 are seventeen lines, so the first drawn row is p3-s1 and line 3 is p4's first session".
- `layout_test.go` `TestModel_FocusedSessionRowIsMarked_issue35`: work on `stripSGR` lines; `"»"` → `"▎"`; assert the first body line carrying `▎` contains `main` and the line after it also starts with `▎`.
- `render_test.go` #129 test: `strings.Contains(view, "» ")` → `strings.Contains(stripSGR(view), "▎")`; `"> p0"` → the first body line starts with `" p0"` after the 13 `k` presses (`strings.HasPrefix(stripSGR(frameLines(m)[2]), " p0")`). `TestRenderRow_TheAgeMovedToTheFocusedPanesRule_issue128`: the age is back on the card by design (#176) — rewrite as `TestCard_TheAgeIsOnTheCardAndInTheHeader_issue176` asserting `lines[0]` and `stripSGR(m.CardOf("s1")[0])` both contain `4m`, and say in the commit message that #128's assertion was a display contract this slice changes, not a bug's regression test.
- `reach_test.go`: `strings.Contains(got, "» wstech")` → `strings.Contains(stripSGR(got), "▎wstech")`.
- `archive_test.go`: `strings.Contains(got, "» ")` → `strings.Contains(stripSGR(got), "▎")`.
- `meter_test.go`: `m.RowOf("s1")` → `strings.Join(m.CardOf("s1"), "")`.
- `status_test.go` `rowOf` keeps working (the glyph and title share line one).

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/ui -run 'Card_|Click|issue35|issue129' -v`
Expected: `undefined: ui.CardOf`, `ui.Rail`.

- [ ] **Step 3: Create card.go and wire the click**

```go
// The session card (#176): two lines per session where one row was, in the
// anatomy ade's TUI fixes for every card so the renderer and the mouse can
// share one height. Line one names it, line two says where it is and what
// it did.

package ui

import (
	"strings"
	"time"
)

// The card's columns at SidebarWidth 28: 27 of content, the last one blank
// so nothing touches the hairline. Line one spends the rail, the glyph, two
// spaces, the age and the blank; the title gets 18, three more than #155's
// fifteen. Line two spends the rail, two spaces, a space, the lane and the
// blank; the branch and the diffstat share the 16 between.
const (
	cardCols  = sidebarContentCols
	ageCols   = 4
	titleCols = cardCols - 1 - 1 - 1 - 1 - ageCols - 1
	metaCols  = cardCols - 1 - 2 - 1 - laneCells - 1
)

// rail is the cursor: an accent bar down the left of both lines of the
// selected card, the same idiom as the accent hairline on the keyboard owner
// (#174). A block element like the lane cells, so the East Asian width rule
// in lane.go covers it too.
const rail = "▎"

// renderRow draws a row's lines: one for a project header, two for a card.
//
//	▎● parser-fix           4m
//	▎  main      +12 −3 ▁▃▇█▅▂
func (m *Model) renderRow(row Row, now time.Time) []string {
	if row.Session == nil {
		return []string{m.renderHeaderRow(row.Project)}
	}
	r := m.rail(m.isSelected(row.Session.ID))
	return []string{
		r + m.cardTop(row, now),
		r + "  " + m.cardMeta(row.Session.ID) + " " + m.renderLane(row.Session.ID) + " ",
	}
}

// renderHeaderRow is a project's one line: the rail column, then the name in
// muted. The rail is drawn only on an empty project the cursor rests on, the
// one header that can be selected (#158).
func (m *Model) renderHeaderRow(project string) string {
	p, ok := m.sidebar.SelectedHeader()
	return m.rail(ok && p == project) + mutedStyle.Render(fitLine(project, cardCols-1))
}

// cardTop is line one past the rail: the glyph, the title, the age.
func (m *Model) cardTop(row Row, now time.Time) string {
	glyph := glyphStyle(row.Status).Render(statusGlyph(row.Status))
	title := m.titleStyle(row.Session.ID).Render(fitLine(row.Session.Title, titleCols))
	age := mutedStyle.Render(padLeft(clip(AgeString(now, m.status[row.Session.ID].At), ageCols), ageCols))
	return glyph + " " + title + " " + age + " "
}

// cardMeta is line two's middle. Until #180 wires the branch and the diffstat
// it is blank; the lane keeps its place at the right.
func (m *Model) cardMeta(_ string) string { return strings.Repeat(" ", metaCols) }

// rail is the accent bar on the selected card, a blank column on the rest.
func (m *Model) rail(selected bool) string {
	if selected {
		return accentStyle.Render(rail)
	}
	return " "
}

// titleStyle is ink and bold on the selected card, text on the rest.
func (m *Model) titleStyle(id string) lipgloss.Style {
	if m.isSelected(id) {
		return headerStyle
	}
	return textStyle
}

// isSelected reports whether the cursor rests on session id.
func (m *Model) isSelected(id string) bool {
	sel, ok := m.sidebar.Selected()
	return ok && sel.Session.ID == id
}
```

(add the `lipgloss` import). In `render.go`: delete `renderRow`, `headerMarker`, `rowChrome`; `renderSidebar` becomes

```go
func (m *Model) renderSidebar(rows int, now time.Time) string {
	lines := make([]string, 0, rows)
	for _, row := range m.sidebar.Window(rows - sidebarHeaderRows) {
		lines = append(lines, m.renderRow(row, now)...)
	}
	return fitBlock(lines, sidebarContentCols, rows)
}
```

and add beside `padRight`:

```go
// padLeft right-aligns s in width, ANSI-aware like padRight.
func padLeft(s string, width int) string {
	if n := width - lipgloss.Width(s); n > 0 {
		return strings.Repeat(" ", n) + s
	}
	return s
}
```

`sidebarclick.go`:

```go
// sidebarRowAt maps a window row to a row index: under the header row and
// the rule, then down the drawn rows by their heights, so either line of a
// card is the card (#45, #129, #176). ok is false for a header with sessions
// under it, an empty line, and anything outside the sidebar.
func (m *Model) sidebarRowAt(winY int) (int, bool) {
	m.sidebarOffset() // recompute the window the frame would draw
	i, ok := m.sidebar.rowAtLine(winY - sidebarTop())
	if !ok || !m.sidebar.landable(i) {
		return 0, false
	}
	return i, true
}
```

(`sidebarOffset` stays; its return value is no longer used here, so make it `func (m *Model) syncSidebarWindow()` with no return and rename the one call.) In `lane.go`, append to `laneCells`' comment: `#176 moved the lane to the card's second line; the title has eighteen columns on the first, and six cells is still the lane's budget.`

- [ ] **Step 4: Run everything and the gate**

Run: `go test ./internal/ui -race`, then the full gate. Check `render.go` and `card.go` are each under 500 lines and every function under 20.

- [ ] **Step 5: Commit and open the PR**

```bash
git add -A internal/ui
git commit -m "feat(#176): two-line session cards with the rail cursor"
git push -u origin feat/176-cards && gh pr create --base develop --title "feat(#176): cards" --body "..."
```
Smoke at 120x32 and 80x24: cards two lines, ages aligned, the rail on both lines of the selection, a click on the second line of a card selects it (add `PTY_KEYS=$'\x1b[<0;5;6M\x1b[<0;5;6m'` to send a left click at column 5, row 6 in SGR mouse encoding, 1-based: that is line 4 of the body, s2's second line).

---

## Slice D — Header row (#177), branch `feat/177-header`

Assumes A, B and PR #171 merged. PR #171 changed `tokensPart` to show `In + CacheRead + CacheWrite` through a shared helper; read `meter.go` first and reuse that helper's name below where `fedTokens` is written.

### Task D1: The breadcrumb, the collapse order and the modal names

**Files:**
- Create: `internal/ui/header.go`
- Modify: `internal/ui/render.go` (delete `terminalTitle`; `paneTitle` and `bodyColumns` call `header.go`), `internal/ui/meter.go` (split `tokensPart` into `meterPart` and `countsPart`; keep `tokensPart` only if something else still calls it), `internal/ui/modalview.go` (add `modalName`)
- Modify: `internal/ui/export_test.go` (`Collapse`, `ModalNames`)
- Test: create `internal/ui/header_test.go`; edit `meter_test.go`/`status_test.go` only if a string moved

**Interfaces:**
- Produces: `type headerParts struct{ crumb, branch, state, meter, counts string }`; `collapse(width int, p headerParts) string`; `(m *Model) paneSegment(now time.Time, width int, owns bool) string`; `(m *Model) breadcrumbBranch(id string) string` (stub returning ""; F fills); `modalName(md modal) string`; `(m *Model) sidebarSegment() string` = `projects · N`.

- [ ] **Step 1: Write the failing tests** (`header_test.go`)

```go
package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func headerOf(m *ui.Model) string { return stripSGR(frameLines(m)[0]) }

// The right side collapses in steps: the counts, then the meter, then the
// branch; last, the left side is clipped. Widths chosen around the parts:
// crumb 20, branch 6, state 12, meter 19, counts 21.
func TestCollapse_DropsTheRightSideInTheSpecsOrder_issue177(t *testing.T) {
	p := ui.HeaderParts{Crumb: "omatty ▎ parser-fix", Branch: "main", State: "● waiting 4m",
		Meter: "▰▰▰▰▰▱▱▱ 62% cached", Counts: "60.2k in / 62.6k out"}
	for _, tt := range []struct {
		width      int
		want, drop string
	}{
		{100, "60.2k in", ""},
		{70, "62% cached", "60.2k in"},
		{45, "main", "cached"},
		{38, "waiting", "main"},
		{20, "omatty", "waiting"},
	} {
		got := ui.Collapse(tt.width, p)
		if lipgloss.Width(got) != tt.width || !strings.Contains(got, tt.want) || (tt.drop != "" && strings.Contains(got, tt.drop)) {
			t.Errorf("Collapse(%d) = %q (%d cells): want %q kept, %q dropped", tt.width, got, lipgloss.Width(got), tt.want, tt.drop)
		}
	}
}

func TestHeader_ReadsProjectTitleBranchStatusAndUsage_issue177(t *testing.T) {
	m, _, _ := modelWithEvents(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: fixedNow.Add(-4 * time.Minute)})
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.UsageUpdated, At: fixedNow,
		Tokens: watcher.Tokens{In: 2000, CacheRead: 8000, Out: 500}})

	head := headerOf(m)

	for _, want := range []string{" projects · 2", "│ omatty ▎ main · ● waiting 4m", "▰▰▰▰▰▰▱▱ 80% cached · 10.0k in / 500 out"} {
		if !strings.Contains(head, want) {
			t.Errorf("header %q lacks %q", head, want)
		}
	}
	if !strings.HasSuffix(strings.TrimRight(head, " "), "out") || lipgloss.Width(frameLines(m)[0]) != 120 {
		t.Errorf("the usage is not right-aligned on a 120-cell row: %q", head)
	}
}

// At 60 columns the pane is 32: the counts and the meter go, the status
// stays, and the footer's exit key is still first.
func TestHeader_CollapsesAtSixtyColumnsAndTheExitKeyStays_issue177(t *testing.T) {
	m, _, _ := modelWithEvents(t)
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.PermissionRequested, At: fixedNow})
	m.Update(ui.StatusMsg{SessionID: "s1", Kind: watcher.UsageUpdated, At: fixedNow, Tokens: watcher.Tokens{In: 2000, Out: 500}})
	head := headerOf(m)
	if strings.Contains(head, "cached") || strings.Contains(head, " in /") || !strings.Contains(head, "waiting") {
		t.Errorf("header at 60 = %q, want the usage collapsed and the status kept", head)
	}
	if !strings.HasPrefix(stripSGR(footerOf(m)), " ctrl+o q quit") {
		t.Errorf("footer at 60 = %q, the exit key moved", stripSGR(footerOf(m)))
	}
}

func TestHeader_NamesEachModalSurface_issue177(t *testing.T) {
	for _, tt := range []struct {
		open tea.KeyPressMsg
		want string
	}{
		{key('n'), "new session"}, {key('N'), "new worktree session"}, {key('R'), "rename"},
		{key('x'), "confirm"}, {key('/'), "switch"}, {key('?'), "keys"},
	} {
		m, _ := modelWithFakes(t)
		m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
		leader(m, tt.open)
		if head := headerOf(m); !strings.Contains(head, "│ "+tt.want) {
			t.Errorf("after %q the header reads %q, want %q", tt.open.Keystroke(), head, tt.want)
		}
	}
	if names := ui.ModalNames(); len(names) != 8 {
		t.Errorf("%d modal names, want 8 (register project and adopt session need a scan to open)", len(names))
	}
}

func TestHeader_IsEmptyForThePaneWithNoSession_issue177(t *testing.T) {
	m := ui.NewModel(baseDeps(emptyState(), fakeTermsFor(emptyState())))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if head := headerOf(m); strings.TrimSpace(strings.TrimPrefix(head, " projects · 0")) != "│" {
		t.Errorf("header = %q, want the projects count, the hairline and nothing else", head)
	}
}
```

Exports:

```go
// HeaderParts and Collapse are the pane segment's pieces and the collapse
// order, for the width table (#177). ModalNames is every surface's name.
type HeaderParts = headerParts  // fields must be exported for the test: rename them Crumb, Branch, State, Meter, Counts
func Collapse(width int, p HeaderParts) string { return collapse(width, p) }
func ModalNames() []string {
	out := []string{}
	for _, md := range []modal{{Kind: modalPrompt}, {Kind: modalPrompt, Editor: lineEditor{Worktree: true}}, {Kind: modalRename},
		{Kind: modalConfirm}, {Kind: modalList}, {Kind: modalPicker}, {Kind: modalAdopt}, {Kind: modalHelp}} {
		out = append(out, modalName(md))
	}
	return out
}
```

Declare `headerParts` with exported fields (`Crumb, Branch, State, Meter, Counts`) so the alias works; it stays an unexported type.

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/ui -run 'Collapse|Header_' -v` → `undefined: ui.HeaderParts`.

- [ ] **Step 3: Create header.go, split the meter, name the modals**

```go
// The header row's pane segment (#177): the breadcrumb on the left, the
// usage on the right, or an open modal's name. ade's header reads app, lane,
// branch, chat; omatty's reads project, session, branch, status, because
// those are the four things a glance at a pane has to answer.

package ui

import (
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// headerParts is the pane segment before collapsing: three pieces on the
// left, two on the right, each droppable in the order collapse gives.
type headerParts struct{ Crumb, Branch, State, Meter, Counts string }

// sides joins the pieces present on each side with " · ".
func (p headerParts) sides() (left, right string) {
	return dots(p.Crumb, p.Branch, p.State), dots(p.Meter, p.Counts)
}

// steps is the collapse order: whole, without the counts, without the meter,
// without the branch. The status is never dropped: it is what the header
// exists to say.
func (p headerParts) steps() [4]headerParts {
	noCounts := p
	noCounts.Counts = ""
	noMeter := noCounts
	noMeter.Meter = ""
	noBranch := noMeter
	noBranch.Branch = ""
	return [4]headerParts{p, noCounts, noMeter, noBranch}
}

// collapse lays the parts on width cells, right side right-aligned, dropping
// pieces in steps until they fit; when even the last step does not, the left
// side is cut from the right (#177).
func collapse(width int, p headerParts) string {
	steps := p.steps()
	for _, try := range steps {
		if left, right := try.sides(); fits(left, right, width) {
			return joinEnds(left, right, width)
		}
	}
	left, _ := steps[3].sides()
	return fitLine(left, width)
}

// fits reports whether left and right sit on width cells with the two-space
// gap joinEnds keeps between them.
func fits(left, right string, width int) bool {
	gap := 0
	if right != "" {
		gap = 2
	}
	return lipgloss.Width(left)+gap+lipgloss.Width(right) <= width
}

// dots joins the non-empty parts with the middle dot the rule already used.
func dots(parts ...string) string {
	kept := parts[:0:0]
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " · ")
}

// paneSegment is the header row's pane share. A modal names itself; a pane
// with no session says nothing; otherwise the breadcrumb collapses into
// width. owns colours the crumb's rail: accent while the pane has the keys.
func (m *Model) paneSegment(now time.Time, width int, owns bool) string {
	if m.modalOpen() {
		return modalName(m.modal)
	}
	row, ok := m.sidebar.Selected()
	if !ok {
		return ""
	}
	return collapse(width, m.paneParts(row, now, owns))
}

// paneParts gathers the selected session's pieces. The title is left
// unstyled so the segment's own style - ink for the keyboard owner, muted
// otherwise - is what colours it.
func (m *Model) paneParts(row Row, now time.Time, owns bool) headerParts {
	st := m.status[row.Session.ID]
	p := headerParts{
		Crumb:  mutedStyle.Render(row.Project) + " " + crumbRail(owns) + " " + row.Session.Title,
		Branch: m.breadcrumbBranch(row.Session.ID),
	}
	if st.Status != "" {
		glyph := glyphStyle(st.Status).Render(statusGlyph(st.Status))
		p.State = strings.TrimSpace(glyph + " " + string(st.Status) + " " + AgeString(now, st.At))
	}
	p.Meter, p.Counts = meterPart(st.Tokens), countsPart(st.Tokens)
	return p
}

// crumbRail marks the session the segment is about: accent while the pane
// owns the keys, muted otherwise, the rail the card wears (#176).
func crumbRail(owns bool) string {
	if owns {
		return accentStyle.Render(rail)
	}
	return mutedStyle.Render(rail)
}

// breadcrumbBranch is the branch piece. Empty until #180 polls it, which is
// the third collapse step already, so the header is unchanged in shape.
func (m *Model) breadcrumbBranch(_ string) string { return "" }

// sidebarSegment is the header row's sidebar share: how many projects.
func (m *Model) sidebarSegment() string {
	return "projects · " + strconv.Itoa(len(m.state.Projects))
}
```

(add `strconv`). In `meter.go` replace `tokensPart` with two functions, keeping its comment split across them:

```go
// meterPart is the bar and its percentage, "" with no input to measure.
func meterPart(t watcher.Tokens) string {
	share, ok := cacheShare(t)
	if !ok {
		return ""
	}
	return renderMeter(share) + " " + mutedStyle.Render(strconv.Itoa(int(math.Round(share*100)))+"% cached")
}

// countsPart is the in/out counts the rule carried before (#39, #170), "" for
// a session that has reported no tokens at all.
func countsPart(t watcher.Tokens) string {
	if t == (watcher.Tokens{}) {
		return ""
	}
	return mutedStyle.Render(KString(fedTokens(t)) + " in / " + KString(t.Out) + " out")
}
```

(`fedTokens` is whatever #171 named the shared total; if #171 inlined it, add `fedTokens(t) int { return t.In + t.CacheRead + t.CacheWrite }` and make `cacheShare` use it.) In `modalview.go` add:

```go
// modalName is what the header row calls an open surface (#177): one name per
// surface as opened, in sentence case. The prompt is one kind opened two ways.
func modalName(md modal) string {
	switch md.Kind {
	case modalPrompt:
		if md.Editor.Worktree {
			return "new worktree session"
		}
		return "new session"
	case modalRename:
		return "rename"
	case modalConfirm:
		return "confirm"
	case modalList:
		return "switch"
	case modalPicker:
		return "register project"
	case modalAdopt:
		return "adopt session"
	case modalHelp:
		return "keys"
	}
	return ""
}
```

In `render.go`: delete `terminalTitle` and `paneTitle`; in `bodyColumns` the two segments become `{title: m.sidebarSegment(), width: sidebarContentCols}` and `{title: m.paneSegment(now, termW-2, edge == edgePane), width: termW, owns: edge == edgePane}` (`termW-2`: the leading space `headerRow` adds and one blank before the hairline).

- [ ] **Step 4: Run everything and the gate**

Run: `go test ./internal/ui -race`. `TestModel_TheRuleCarriesTheCacheMeter_issue153` and `TestModel_HeaderShowsTokens_issue39` keep passing (same strings on line 0). Then the full gate.

- [ ] **Step 5: Commit and open the PR**

```bash
git add -A internal/ui
git commit -m "feat(#177): the header row reads project, session, branch, status and usage, and names a modal"
git push -u origin feat/177-header && gh pr create --base develop --title "feat(#177): header breadcrumb" --body "..."
```
Smoke at 120x32, 80x24 and 60x20: the breadcrumb, the right-aligned usage, the collapse order, `ctrl+o ?` naming itself `keys`.

---

## Slice E — Footer facts (#178), branch `feat/178-footer`

Assumes B merged (`amberStyle`).

### Task E1: `N sessions · K waiting` on the right, dropped before the keys

**Files:**
- Modify: `internal/ui/render.go` (`renderFooter`; add `footerFacts`, `waitingCount`, `joinEnds`, `countNoun`)
- Modify: `internal/ui/export_test.go` (`Amber(s string) string`)
- Test: create `internal/ui/footer_test.go`

**Interfaces:**
- Produces: `joinEnds(left, right string, width int) string` (used again by slice D's header row).

- [ ] **Step 1: Write the failing tests**

```go
package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// footerOf lives in frame_test.go (slice A); it is not redefined here.

// The keymap is 77 cells with its leading space and the facts 22, so 120
// columns hold both; 100 would not, and that case is the drop test below.
func TestFooter_CarriesTheSessionCountAndTheWaitingCountOnTheRight_issue178(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	status(m, "s3", watcher.PermissionRequested, time.Now())

	got := footerOf(m)

	plain := stripSGR(got)
	if !strings.HasPrefix(plain, " ctrl+o q quit") || !strings.HasSuffix(plain, "3 sessions · 1 waiting") {
		t.Errorf("footer = %q, want the keys left and the facts right", plain)
	}
	if lipgloss.Width(got) != 120 {
		t.Errorf("footer is %d cells, want 120", lipgloss.Width(got))
	}
	if !strings.Contains(got, ui.Amber("1 waiting")) {
		t.Errorf("the waiting count is not amber: %q", got)
	}
}

func TestFooter_OmitsWaitingWhenNobodyWaits_issue178(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	plain := stripSGR(footerOf(m))
	if !strings.HasSuffix(plain, "3 sessions") || strings.Contains(plain, "waiting") {
		t.Errorf("footer = %q, want it to end in the session count alone", plain)
	}
}

// The keymap is 77 cells with its leading space; at 80 columns the facts do
// not fit and are dropped whole, so the exit key never moves (#28, #30).
func TestFooter_DropsTheFactsBeforeTheKeys_issue178(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	plain := stripSGR(footerOf(m))
	if strings.Contains(plain, "sessions") || !strings.HasPrefix(plain, " ctrl+o q quit") {
		t.Errorf("footer = %q at 80 columns, want the keys alone", plain)
	}
	if lipgloss.Width(footerOf(m)) != 80 {
		t.Errorf("footer is %d cells, want 80", lipgloss.Width(footerOf(m)))
	}
}

func TestFooter_OneSessionIsSingular_issue178(t *testing.T) {
	st := twoProjectState()
	st.Sessions = st.Sessions[:1]
	m := ui.NewModel(baseDeps(st, fakeTermsFor(st)))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	if plain := stripSGR(footerOf(m)); !strings.HasSuffix(plain, "1 session") {
		t.Errorf("footer = %q, want it to end in \"1 session\"", plain)
	}
}
```

Export: `func Amber(s string) string { return amberStyle.Render(s) }`.

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/ui -run Footer_ -v`
Expected: `undefined: ui.Amber`, then after adding the export, the suffix assertions fail.

- [ ] **Step 3: Implement in render.go**

Change the last line of `renderFooter` from `return footerStyle.Render(fitLine(" "+m.footerKeys(), m.width))` to:

```go
	return joinEnds(footerStyle.Render(" "+m.footerKeys()), m.footerFacts(), m.width)
```

Add below `footerKeys`:

```go
// footerFacts is the footer's right side (#178): how many sessions there
// are, and how many wait, the waiting count in amber because it is the
// waiting state and the colour rule allows exactly that (#175).
func (m *Model) footerFacts() string {
	facts := mutedStyle.Render(countNoun(len(m.state.Sessions), "session"))
	if k := m.waitingCount(); k > 0 {
		facts += mutedStyle.Render(" · ") + amberStyle.Render(strconv.Itoa(k) + " waiting")
	}
	return facts
}

// waitingCount is how many registered sessions are stopped for the operator.
func (m *Model) waitingCount() int {
	n := 0
	for id, st := range m.status {
		if st.Status == watcher.StatusWaiting && m.knownSession(id) {
			n++
		}
	}
	return n
}

// countNoun is "1 session" or "3 sessions".
func countNoun(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return strconv.Itoa(n) + " " + noun + "s"
}

// joinEnds lays left and right on one line of width cells with at least two
// spaces between. When they do not both fit, right is dropped whole and left
// is cut to the window: the footer's keys outrank its facts, and the header's
// title outranks its meter (#178, #177). Both sides may carry SGR.
func joinEnds(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if right == "" || gap < 2 {
		return fitLine(left, width)
	}
	return left + strings.Repeat(" ", gap) + right
}
```

Add `"strconv"` to render.go's imports.

- [ ] **Step 4: Run the ui tests and the gate**

Run: `go test ./internal/ui -race` then the full gate.
Expected: PASS. If `render.go` passes 500 lines, move `footerFacts`, `waitingCount`, `countNoun` and `joinEnds` into a new `internal/ui/footer.go` together with `exitKeyFor`, `footerLine`, `reviewFooterLine`, `treeFooterLine`, `renderFooter`, `footerKeys` (keep every comment).

- [ ] **Step 5: Commit and open the PR**

```bash
git add internal/ui/render.go internal/ui/footer.go internal/ui/footer_test.go internal/ui/export_test.go
git commit -m "feat(#178): the footer carries the session and waiting counts on the right"
git push -u origin feat/178-footer && gh pr create --base develop --title "feat(#178): footer facts" --body "..."
```

---

## Slice F — Branch and diffstat (#180), branch `feat/180-diffstat`

Assumes C and D merged: the card's second line exists with a 16-column blank between the rail and the lane (`cardMeta` stub from slice C), and the header row has a `breadcrumbBranch` hook returning "" (slice D). F fills both.

### Task F1: `vcs.Shortstat`

**Files:**
- Modify: `internal/vcs/git.go` (interface + `Shortstat` + `parseShortstat`)
- Create: `internal/vcs/export_test.go`
- Test: `internal/vcs/shortstat_test.go`, `internal/vcs/dir_test.go` (guard map)

**Interfaces:**
- Produces: `type Shortstat struct{ Files, Added, Removed int }`; `Shortstat(dir, commit string) (Shortstat, error)` on `Git` and `*CLI`.

- [ ] **Step 1: Write the failing tests**

`internal/vcs/export_test.go`:

```go
package vcs

// ParseShortstat is the parser behind CLI.Shortstat, for the table test: the
// integration test drives git for real, but git will not print a
// deletions-only or a binary-only line on demand (#180).
func ParseShortstat(line string) (Shortstat, error) { return parseShortstat(line) }
```

`internal/vcs/shortstat_test.go`:

```go
package vcs_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/vcs"
)

// Each clause is optional and singular or plural; a clean tree prints nothing.
func TestParseShortstat_Table_issue180(t *testing.T) {
	for _, tt := range []struct {
		line string
		want vcs.Shortstat
	}{
		{"", vcs.Shortstat{}},
		{" 1 file changed", vcs.Shortstat{Files: 1}},
		{" 3 files changed, 12 insertions(+), 4 deletions(-)", vcs.Shortstat{Files: 3, Added: 12, Removed: 4}},
		{" 1 file changed, 2 deletions(-)", vcs.Shortstat{Files: 1, Removed: 2}},
		{" 1 file changed, 1 insertion(+)", vcs.Shortstat{Files: 1, Added: 1}},
		{" 2 files changed, 1 insertion(+), 1 deletion(-)\n", vcs.Shortstat{Files: 2, Added: 1, Removed: 1}},
	} {
		got, err := vcs.ParseShortstat(tt.line)
		if err != nil || got != tt.want {
			t.Errorf("ParseShortstat(%q) = %+v, %v; want %+v", tt.line, got, err, tt.want)
		}
	}
}

func TestParseShortstat_RefusesANonNumericCount_issue180(t *testing.T) {
	_, err := vcs.ParseShortstat(" many files changed")
	if err == nil || !strings.Contains(err.Error(), "many") {
		t.Errorf("error = %v, want one quoting the bad clause", err)
	}
}

// The working tree against the merge-base, like Diff (#21): a commit on the
// branch and an edit on top of it count as one change.
func TestCLI_ShortstatCountsCommittedAndUncommittedTogether_issue180(t *testing.T) {
	repo := newRepo(t)
	writeFile(t, repo, "a.txt", "one\n")
	gitOut(t, repo, "add", "a.txt")
	gitOut(t, repo, "commit", "-m", "a")
	gitOut(t, repo, "checkout", "-b", "feat")
	writeFile(t, repo, "a.txt", "two\nthree\n")
	gitOut(t, repo, "commit", "-am", "two")
	writeFile(t, repo, "a.txt", "two\nthree\nfour\n")
	g := vcs.NewCLI()
	base, err := g.MergeBase(repo, "main")
	if err != nil {
		t.Fatal(err)
	}

	got, err := g.Shortstat(repo, base)

	if err != nil {
		t.Fatalf("Shortstat() error = %v", err)
	}
	if want := (vcs.Shortstat{Files: 1, Added: 3, Removed: 1}); got != want {
		t.Errorf("Shortstat() = %+v, want %+v", got, want)
	}
}

func TestCLI_ShortstatOfACleanTreeIsZero_issue180(t *testing.T) {
	got, err := vcs.NewCLI().Shortstat(newRepo(t), "HEAD")
	if err != nil || got != (vcs.Shortstat{}) {
		t.Errorf("Shortstat() = %+v, %v; want the zero value and no error", got, err)
	}
}
```

Add to the `checks` map in `dir_test.go`'s `TestCLI_AllCommandsValidateTheDirectory_issue29`:

```go
		"Shortstat":      func() error { _, err := g.Shortstat(missing, "HEAD"); return err }(),
```

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/vcs -run 'Shortstat|AllCommands'`
Expected: `undefined: vcs.ParseShortstat`, `g.Shortstat undefined`.

- [ ] **Step 3: Implement**

In `git.go`, add to the `Git` interface after `Diff`:

```go
	// Shortstat summarises the working tree against commit: files changed,
	// lines added and removed. A clean tree is the zero value (#180).
	Shortstat(dir, commit string) (Shortstat, error)
```

Add after `Diff`:

```go
// Shortstat is git's one-line summary of a diff.
type Shortstat struct{ Files, Added, Removed int }

// Shortstat is the numbers a session's sidebar card shows (#180), measured
// the way Diff measures: the working tree against commit, renames detected.
// git prints nothing for a clean tree, so the zero value is not an error.
//
//	st, err := vcs.NewCLI().Shortstat("/wt/parser-fix", base)
func (c *CLI) Shortstat(dir, commit string) (Shortstat, error) {
	out, err := c.run(dir, diffArgs("--shortstat", commit, "--")...)
	if err != nil {
		return Shortstat{}, err
	}
	return parseShortstat(out)
}

// parseShortstat reads " 3 files changed, 12 insertions(+), 4 deletions(-)".
// Every clause is optional - a pure deletion has no insertions clause and a
// binary change has neither - and each names itself, so the order is not
// trusted either.
func parseShortstat(line string) (Shortstat, error) {
	var st Shortstat
	for _, clause := range strings.Split(strings.TrimSpace(line), ",") {
		fields := strings.Fields(clause)
		if len(fields) < 2 {
			continue
		}
		n, err := strconv.Atoi(fields[0])
		if err != nil {
			return Shortstat{}, fmt.Errorf("vcs: shortstat clause %q: want a count first: %w", clause, err)
		}
		st = st.with(fields[1], n)
	}
	return st, nil
}

// with sets the counter a shortstat clause names: file(s), insertion(s) or
// deletion(s).
func (st Shortstat) with(noun string, n int) Shortstat {
	switch {
	case strings.HasPrefix(noun, "file"):
		st.Files = n
	case strings.HasPrefix(noun, "insertion"):
		st.Added = n
	case strings.HasPrefix(noun, "deletion"):
		st.Removed = n
	}
	return st
}
```

Add `"strconv"` to git.go's imports. Add `Shortstat` to `internal/review/fakes_test.go`'s `FakeGit`: field `ShortstatOut vcs.Shortstat` (import `internal/vcs`) and

```go
func (f *FakeGit) Shortstat(dir, commit string) (vcs.Shortstat, error) {
	return f.ShortstatOut, f.record("Shortstat", dir, commit)
}
```

- [ ] **Step 4: Run**

Run: `go test ./internal/vcs ./internal/review -race`
Expected: PASS (review compiles again with the wider interface).

- [ ] **Step 5: Commit**

```bash
git add internal/vcs internal/review/fakes_test.go
git commit -m "feat(#180): vcs.Shortstat summarises a tree against a commit"
```

### Task F2: `review.Source.Stat`

**Files:**
- Modify: `internal/review/source.go`
- Test: `internal/review/source_test.go`

**Interfaces:**
- Produces: `type Stat struct{ Branch string; Added, Removed int }`; `(*Source).Stat(sess registry.Session, projectRoot string) (Stat, error)`.

- [ ] **Step 1: Write the failing tests** (append to source_test.go)

```go
// Stat reads through the same base commit as Load and never lists untracked
// files, so a card can read lower than the review column: the column is the
// truth when opened (#180).
func TestSource_StatCountsTrackedChangesAgainstTheSameBaseAsLoad_issue180(t *testing.T) {
	g := &FakeGit{Branch: "parser-fix", MergeBaseOut: "abc123",
		ShortstatOut: vcs.Shortstat{Files: 2, Added: 12, Removed: 3}}

	st, err := review.NewSource(g).Stat(worktreeSession, "/p/omatty")

	if err != nil {
		t.Fatal(err)
	}
	want := "CurrentBranch(/wt/parser-fix) MergeBase(/wt/parser-fix,develop) Shortstat(/wt/parser-fix,abc123)"
	if calls(g) != want {
		t.Errorf("calls = %s\nwant  %s", calls(g), want)
	}
	if st != (review.Stat{Branch: "parser-fix", Added: 12, Removed: 3}) {
		t.Errorf("Stat() = %+v", st)
	}
}

func TestSource_StatFailureNamesTheSessionAndRef_issue180(t *testing.T) {
	g := &FakeGit{Branch: "main", Errs: map[string]error{"Shortstat": errors.New("boom")}}

	_, err := review.NewSource(g).Stat(registry.Session{ID: "s9", Dir: "/p"}, "/p")

	if err == nil || !strings.Contains(err.Error(), "s9") || !strings.Contains(err.Error(), "HEAD") {
		t.Errorf("error = %v, want one naming session s9 and ref HEAD", err)
	}
}

func TestSource_StatBranchFailureNamesTheDirectory_issue180(t *testing.T) {
	g := &FakeGit{Errs: map[string]error{"CurrentBranch": errors.New("not a repository")}}

	_, err := review.NewSource(g).Stat(registry.Session{ID: "s9", Dir: "/gone"}, "/p")

	if err == nil || !strings.Contains(err.Error(), "/gone") {
		t.Errorf("error = %v, want one naming /gone", err)
	}
}
```

Add `"github.com/WilsonSousajr/omatty/internal/vcs"` to source_test.go's imports.

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/review -run Stat`
Expected: `undefined: review.Stat`.

- [ ] **Step 3: Implement** (append to source.go)

```go
// Stat is what a session's sidebar card shows about its checkout (#180): the
// branch, and the lines added and removed against the same base Load diffs
// against. Tracked changes only - Load also renders untracked files as
// additions - so a card can read lower than the review column, and the
// column is the truth when opened.
type Stat struct {
	Branch         string
	Added, Removed int
}

// Stat reads sess's branch and shortstat through baseCommit, so the base
// resolution lives once (#180).
//
//	st, err := src.Stat(sess, projectRoot)
func (s *Source) Stat(sess registry.Session, projectRoot string) (Stat, error) {
	branch, err := s.git.CurrentBranch(sess.Dir)
	if err != nil {
		return Stat{}, fmt.Errorf("review: branch of session %s in %q: %w", sess.ID, sess.Dir, err)
	}
	ref, err := s.baseCommit(sess, projectRoot)
	if err != nil {
		return Stat{}, err
	}
	short, err := s.git.Shortstat(sess.Dir, ref)
	if err != nil {
		return Stat{}, fmt.Errorf("review: shortstat of session %s against %s: %w", sess.ID, ref, err)
	}
	return Stat{Branch: branch, Added: short.Added, Removed: short.Removed}, nil
}
```

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/review -race` → PASS.

```bash
git add internal/review
git commit -m "feat(#180): review.Source.Stat beside Load"
```

### Task F3: The poll in the model

**Files:**
- Create: `internal/ui/repostat.go`
- Modify: `internal/ui/deps.go` (`RepoStatFunc`, `Deps.Stat`), `internal/ui/run.go` (`RunDeps.Stat`, the `NewModel(Deps{...})` literal gains `Stat: d.Stat`), `internal/ui/model.go` (fields `stat`, `repoStat`, `statPending`, `statFailed`; `withSources` sets `m.stat = d.Stat`; `withRuntimeMaps` allocates the three maps; `Init` appends `m.onStatTick()`; `Update` gains `case StatTickMsg: return m, m.onStatTick()`; `onSessionMsg` gains `case RepoStatMsg: return m.onRepoStat(typed)`), `internal/ui/status.go` (`afterStatus` adds `m.refreshStat(e.SessionID, before, after)`), `internal/ui/archive.go` (`dropSession` also `delete(m.repoStat, id)`), `cmd/omatty/wiring.go` (`Stat: review.NewSource(git).Stat,` beside `Diff`)
- Modify: `internal/ui/fakes_test.go` (`FakeStat`), `internal/ui/export_test.go` (`PollAll`, `RepoStatOf`)
- Test: create `internal/ui/repostat_test.go`

**Interfaces:**
- Produces: `type RepoStatFunc func(sess registry.Session, projectRoot string) (review.Stat, error)`; `type RepoStatMsg struct{ SessionID string; Stat review.Stat; Err error }`; `type StatTickMsg time.Time`; `m.repoStat map[string]review.Stat` read by F4.

- [ ] **Step 1: Write the failing tests**

`fakes_test.go`:

```go
// FakeStat stands in for the git-backed reader behind ui.RepoStatFunc (#180).
type FakeStat struct {
	Stats map[string]review.Stat // by session id
	Err   error
	Asked []string
	Roots []string
}

func (f *FakeStat) Stat(sess registry.Session, root string) (review.Stat, error) {
	f.Asked = append(f.Asked, sess.ID)
	f.Roots = append(f.Roots, root)
	return f.Stats[sess.ID], f.Err
}
```

`export_test.go`:

```go
// PollAll is one stat tick's worth of polls without the tick that re-arms
// it, so a test can run them without blocking on tea.Tick (#180). RepoStatOf
// is what the model holds for a session.
func (m *Model) PollAll() tea.Cmd                     { return m.pollAll() }
func (m *Model) RepoStatOf(id string) (review.Stat, bool) { s, ok := m.repoStat[id]; return s, ok }
```

`repostat_test.go`:

```go
package ui_test

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

func modelWithStat(t *testing.T) (*ui.Model, *FakeStat) {
	t.Helper()
	terms, _ := fakeTerms(t)
	stat := &FakeStat{Stats: map[string]review.Stat{"s1": {Branch: "main", Added: 12, Removed: 3}}}
	d := baseDeps(twoProjectState(), terms)
	d.Stat = stat.Stat
	return ui.NewModel(d), stat
}

func TestModel_AStatTickPollsEverySessionWithItsProjectRoot_issue180(t *testing.T) {
	m, stat := modelWithStat(t)

	deliver(m, m.PollAll())

	if len(stat.Asked) != 3 || stat.Roots[0] == "" {
		t.Fatalf("asked %v with roots %v, want all three sessions and their project roots", stat.Asked, stat.Roots)
	}
	if st, ok := m.RepoStatOf("s1"); !ok || st.Branch != "main" || st.Added != 12 {
		t.Errorf("RepoStatOf(s1) = %+v, %v; want the polled stat", st, ok)
	}
}

func TestModel_AStatTickReArmsItself_issue180(t *testing.T) {
	m, _ := modelWithStat(t)
	if _, cmd := m.Update(ui.StatTickMsg(fixedNow)); cmd == nil {
		t.Error("a stat tick returned no command; the poll would run once and never again")
	}
}

// A session with a poll in flight is not polled again until it answers.
func TestModel_APollInFlightIsNotRepeated_issue180(t *testing.T) {
	m, stat := modelWithStat(t)
	first := m.PollAll()

	deliver(m, m.PollAll())
	if len(stat.Asked) != 0 {
		t.Fatalf("the second poll asked %v while the first was in flight", stat.Asked)
	}
	deliver(m, first)
	if len(stat.Asked) != 3 {
		t.Errorf("after the first poll landed, asked %v, want all three", stat.Asked)
	}
	deliver(m, m.PollAll())
	if len(stat.Asked) != 6 {
		t.Errorf("after the answers landed a new poll asked %d in total, want 6", len(stat.Asked))
	}
}

// Done and waiting are the moments the numbers change; a tool start is not.
func TestModel_DoneAndWaitingPollAtOnce_issue180(t *testing.T) {
	m, stat := modelWithStat(t)
	statusDeliver(m, "s1", watcher.ToolStarted, fixedNow)
	if len(stat.Asked) != 0 {
		t.Fatalf("a tool start polled %v", stat.Asked)
	}
	statusDeliver(m, "s1", watcher.TurnEnded, fixedNow.Add(time.Second))
	statusDeliver(m, "s2", watcher.PermissionRequested, fixedNow.Add(2*time.Second))
	if strings.Join(stat.Asked, " ") != "s1 s2" {
		t.Errorf("asked %v, want s1 on done then s2 on waiting", stat.Asked)
	}
}

// A failure keeps the last stat, is logged once per session, and logs again
// only after a success in between.
func TestModel_AFailedPollKeepsTheLastStatAndWarnsOnce_issue180(t *testing.T) {
	var log bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	m, stat := modelWithStat(t)
	deliver(m, m.PollAll())
	stat.Err = errors.New("boom")

	deliver(m, m.PollAll())
	deliver(m, m.PollAll())

	if st, ok := m.RepoStatOf("s1"); !ok || st.Added != 12 {
		t.Errorf("RepoStatOf(s1) = %+v, %v; want the last good stat kept", st, ok)
	}
	if n := strings.Count(log.String(), "reading repo stat"); n != 3 {
		t.Errorf("logged %d warnings after two failed rounds over three sessions, want 3 (once each)", n)
	}
	stat.Err = nil
	deliver(m, m.PollAll())
	stat.Err = errors.New("boom again")
	deliver(m, m.PollAll())
	if n := strings.Count(log.String(), "reading repo stat"); n != 6 {
		t.Errorf("logged %d warnings, want 6: the once-flag clears on success", n)
	}
}

func TestModel_NoStatReaderPollsNothing_issue180(t *testing.T) {
	m, _ := modelWithFakes(t)
	if cmd := m.PollAll(); cmd != nil {
		deliver(m, cmd) // must not panic
	}
	if _, ok := m.RepoStatOf("s1"); ok {
		t.Error("a model with no stat reader holds a stat")
	}
}
```

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/ui -run 'issue180'`
Expected: `d.Stat undefined`, `undefined: ui.StatTickMsg`.

- [ ] **Step 3: Implement**

`deps.go`, after `DiffFunc`'s neighbours (put the type beside `DiffFunc` in review.go or here; here is fine):

```go
// RepoStatFunc reads a session's branch and diffstat for its sidebar card
// (#180). Injected so ui never touches git (invariant 4). Nil is the switch,
// as ModelName's is: with nothing wired the card draws its lane alone, which
// is what every test's Deps gets.
type RepoStatFunc func(sess registry.Session, projectRoot string) (review.Stat, error)
```

and in `Deps`, after `Preview`:

```go
	// Stat reads a session's branch and diffstat for its card; nil means no
	// git to ask (#180).
	Stat RepoStatFunc
```

`run.go`: the same field on `RunDeps` beside `Diff`/`Files`, and `Stat: d.Stat,` in the `NewModel(Deps{...})` literal.

`model.go`: fields

```go
	// stat reads a card's branch and diffstat; repoStat is the last answer per
	// session, display-only like the lane and never persisted - state.json
	// must suffice alone (invariant 9). statPending guards one poll in flight
	// per session; statFailed makes the warning once per outage (#180).
	stat        RepoStatFunc
	repoStat    map[string]review.Stat
	statPending map[string]bool
	statFailed  map[string]bool
```

`withSources`: `m.stat = d.Stat`. `withRuntimeMaps`: allocate the three maps. `Init`: `cmds = append(cmds, scheduleTick(), m.onStatTick())`. `Update`: `case StatTickMsg: return m, m.onStatTick()`. `onSessionMsg`: `case RepoStatMsg: return m.onRepoStat(typed)`. `status.go` `afterStatus`: add `m.refreshStat(e.SessionID, before, after)` to the batch. `archive.go` `dropSession` (find it: `grep -n "func (m \*Model) dropSession" internal/ui/*.go`): add `delete(m.repoStat, id)`.

`repostat.go`:

```go
// Each session card's branch and diffstat (#180). New data, so a new seam
// following the diff's route: vcs.Shortstat under review.Source.Stat under
// a typed func here, polled off the render path and kept in memory only.

package ui

import (
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// RepoStatMsg carries one poll's answer into Update. Exported so tests can
// send one.
type RepoStatMsg struct {
	SessionID string
	Stat      review.Stat
	Err       error
}

// StatTickMsg is the heartbeat that polls every session's checkout.
// Exported so tests can send one.
type StatTickMsg time.Time

// statEvery is the poll's period. Ten seconds: a diffstat changes when a
// turn ends, and those moments poll at once (refreshStat); the tick catches
// edits made outside claude.
const statEvery = 10 * time.Second

func scheduleStatTick() tea.Cmd {
	return tea.Tick(statEvery, func(t time.Time) tea.Msg { return StatTickMsg(t) })
}

// onStatTick polls every session and re-arms the tick.
func (m *Model) onStatTick() tea.Cmd { return tea.Batch(m.pollAll(), scheduleStatTick()) }

// pollAll is one poll per session, nil for a model with no reader.
func (m *Model) pollAll() tea.Cmd {
	if m.stat == nil {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(m.state.Sessions))
	for _, sess := range m.state.Sessions {
		cmds = append(cmds, m.pollStat(sess.ID))
	}
	return tea.Batch(cmds...)
}

// pollStat reads one session's stat off the Update goroutine, the shape
// loadDiff uses. A poll already in flight is not repeated.
func (m *Model) pollStat(id string) tea.Cmd {
	sess, ok := m.session(id)
	if !ok || m.stat == nil || m.statPending[id] {
		return nil
	}
	m.statPending[id] = true
	root, read := m.projectRoot(sess.Project), m.stat
	return func() tea.Msg {
		st, err := read(sess, root)
		return RepoStatMsg{SessionID: id, Stat: st, Err: err}
	}
}

// refreshStat polls a session the moment its turn ends or it stops for a
// question: the two moments its numbers change (the rule refreshReview uses).
func (m *Model) refreshStat(id string, before, after watcher.Status) tea.Cmd {
	if before == after || (after != watcher.StatusDone && after != watcher.StatusWaiting) {
		return nil
	}
	return m.pollStat(id)
}

// onRepoStat stores an answer. A failure keeps the last stat - the card says
// what it last knew rather than nothing - and is logged once per outage; the
// once-flag clears on the next success so a checkout that breaks again logs
// again. The footer says nothing: a broken checkout is loud in the review
// column, which is where it is actionable.
func (m *Model) onRepoStat(msg RepoStatMsg) tea.Cmd {
	delete(m.statPending, msg.SessionID)
	if msg.Err == nil {
		delete(m.statFailed, msg.SessionID)
		m.repoStat[msg.SessionID] = msg.Stat
		return nil
	}
	if !m.statFailed[msg.SessionID] {
		slog.Warn("reading repo stat", "session", msg.SessionID, "err", msg.Err)
	}
	m.statFailed[msg.SessionID] = true
	return nil
}
```

`cmd/omatty/wiring.go` in `tuiDeps`: `Stat:    review.NewSource(git).Stat,` after `Diff`.

- [ ] **Step 4: Run and gate**

Run: `go test ./internal/ui -race -run 'issue180|Init|Tick'` then the full gate. If `gocyclo` flags `Update` or `onSessionMsg`, move the `RepoStatMsg` case into a new `onStatMsg(msg) tea.Cmd` table called from `onSessionMsg`'s fall-through, mirroring how `onSessionMsg` was split from `onDataMsg`.

- [ ] **Step 5: Commit**

```bash
git add internal/ui cmd/omatty/wiring.go
git commit -m "feat(#180): poll each session's branch and diffstat off the render path"
```

### Task F4: The card's second line and the header's branch

**Files:**
- Modify: `internal/ui/render.go` (or `card.go` if slice C created it): `cardMeta`, `diffstat`
- Modify: the header builder from slice D: `breadcrumbBranch(id string) string` returns `m.repoStat[id].Branch`
- Test: `internal/ui/repostat_test.go` (append)

- [ ] **Step 1: Write the failing tests**

```go
func TestCard_LineTwoSharesSixteenColumnsBetweenBranchAndDiffstat_issue180(t *testing.T) {
	m, _ := modelWithStat(t)
	for _, tt := range []struct {
		stat review.Stat
		want string // the 16 columns between the rail's two spaces and the lane
	}{
		{review.Stat{Branch: "main", Added: 12, Removed: 3}, "main      +12 −3"},
		{review.Stat{Branch: "feature/very-long-branch-name", Added: 1, Removed: 0}, "feature/ve +1 −0"},
		{review.Stat{Branch: "main"}, "main            "},
		{review.Stat{Branch: "main", Added: 1234, Removed: 5}, "main   +1.2k −5"},
	} {
		m.Update(ui.RepoStatMsg{SessionID: "s1", Stat: tt.stat})
		line := []rune(stripSGR(m.CardOf("s1")[1])) // runes: the rail and the minus sign are multi-byte
		if got := string(line[3 : 3+16]); got != tt.want {
			t.Errorf("stat %+v: line two middle = %q, want %q (line %q)", tt.stat, got, tt.want, string(line))
		}
		if lipgloss.Width(m.CardOf("s1")[1]) != ui.SidebarWidth-1 {
			t.Errorf("line two is %d cells, want %d", lipgloss.Width(m.CardOf("s1")[1]), ui.SidebarWidth-1)
		}
	}
}

func TestCard_AnUnknownBranchLeavesLineTwoToTheLane_issue180(t *testing.T) {
	m, _ := modelWithStat(t)
	if got := stripSGR(m.CardOf("s2")[1]); strings.TrimSpace(got) != "" {
		t.Errorf("line two of an unpolled session = %q, want blanks and the empty lane", got)
	}
}

func TestCard_TheDiffstatWearsTheDiffColours_issue180(t *testing.T) {
	m, _ := modelWithStat(t)
	m.Update(ui.RepoStatMsg{SessionID: "s1", Stat: review.Stat{Branch: "main", Added: 12, Removed: 3}})
	line := m.CardOf("s1")[1]
	if !strings.Contains(line, ui.Added("+12")) || !strings.Contains(line, ui.Removed("−3")) {
		t.Errorf("line two %q does not carry +12 in green and −3 in red", line)
	}
}

func TestHeader_CarriesTheBranchOncePolled_issue180(t *testing.T) {
	m, _ := modelWithStat(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m.Update(ui.RepoStatMsg{SessionID: "s1", Stat: review.Stat{Branch: "main"}})
	if head := stripSGR(strings.Split(m.View().Content, "\n")[0]); !strings.Contains(head, "main · main") {
		t.Errorf("header %q does not read <title> · <branch> for s1 (titled main on branch main)", head)
	}
}
```

Exports needed: `Added(s) string { return addedStyle.Render(s) }`, `Removed(s) string { return removedStyle.Render(s) }`; `CardOf` comes from slice C. Add `tea` and `lipgloss` imports.

- [ ] **Step 2: Run to see them fail**

Run: `go test ./internal/ui -run 'Card_|Header_CarriesTheBranch'` → the middle is 16 blanks.

- [ ] **Step 3: Implement**

Replace slice C's `cardMeta` stub with:

```go
// metaCols is what line two gives the branch and the diffstat together: the
// 27 columns minus the rail, two spaces, a space, the lane and the trailing
// blank (#176, #180).
const metaCols = 16

// cardMeta is line two's middle: the branch, then the diffstat right-aligned.
// The diffstat is drawn whole and the branch clipped to what remains; a clean
// tree gives the branch all sixteen, and an unpolled session gives blanks.
func (m *Model) cardMeta(id string) string {
	st, ok := m.repoStat[id]
	if !ok {
		return strings.Repeat(" ", metaCols)
	}
	stat := diffstat(st)
	return fitLine(st.Branch, metaCols-lipgloss.Width(stat)) + stat
}

// diffstat is "+12 −3" in the diff colours, "" for a clean tree - never
// "+0 −0" - with KString keeping a large count inside the budget (#180).
func diffstat(st review.Stat) string {
	if st.Added == 0 && st.Removed == 0 {
		return ""
	}
	return addedStyle.Render("+"+KString(st.Added)) + " " + removedStyle.Render("−"+KString(st.Removed))
}
```

Make slice D's `breadcrumbBranch(id string) string` return `m.repoStat[id].Branch` (a missing entry is the zero `Stat`, so "" and the header omits the branch, as before).

- [ ] **Step 4: Full gate, then the smoke**

Run the gate. Then the smoke with a real repo so a real branch and diffstat appear (scratch HOME per `docs/superpowers/specs/...` "Smoke", `PTY_WAIT=12s` so one stat tick lands, or rely on the startup poll):

```bash
S=$SCRATCH; T=$S/h; mkdir -p "$T/.omatty" "$T/bin"; ln -sf "$PWD/testdata/fake-claude" "$T/bin/claude"
# state.json with two projects whose "path"/"root" is a real repo (this one), sessions with "base":"develop"
go build -o "$S/omatty" ./cmd/omatty
env -i HOME="$T" PATH="$T/bin:/usr/bin:/bin:$(go env GOROOT)/bin" TERM=xterm-256color COLORTERM=truecolor \
  PTY_COLS=120 PTY_ROWS=32 PTY_WAIT=6s "$(go env GOROOT)/bin/go" run ./testdata/ptyrun "$S/omatty" > "$S/cap.txt"
"$(go env GOROOT)/bin/go" run ./testdata/screen "$S/cap.txt" 120 32
```
Read: line two of each card shows `develop` (or the checkout's branch) and, after touching a tracked file in the repo, `+1 −0`.

- [ ] **Step 5: Commit and open the PR**

```bash
git add internal/ui
git commit -m "feat(#180): cards show the branch and diffstat; the header reads the branch"
git push -u origin feat/180-diffstat && gh pr create --base develop --title "feat(#180): branch and diffstat on every card" --body "..."
```

---

## Slice G — Close-out (#179), branch `docs/179-m8-closeout`

### Task G1: The three smokes, read by a person

- [ ] **Step 1:** Build `develop` after F merges and run the three sizes in the scratch HOME (recipe in Task F4 step 4), with `PTY_KEYS=$'\x0fd'` on the 120x32 run to open the review column, and a second 120x32 run with `PTY_KEYS=$'\x0f?'` to open a modal:

```bash
PTY_COLS=120 PTY_ROWS=32 PTY_KEYS=$'\x0fd'   # review column: two hairlines, the accent on the pane/review one
PTY_COLS=120 PTY_ROWS=32 PTY_KEYS=$'\x0f?'   # modal: the header names "keys", the pane hairline is accent
PTY_COLS=80  PTY_ROWS=24                      # counts collapsed from the header, facts dropped from the footer
PTY_COLS=60  PTY_ROWS=20                      # meter collapsed too, exit key still first
```
- [ ] **Step 2:** For each capture run `testdata/screen` and check: every row is exactly the width plus the trailing `|`; the accent hairline is on the keyboard owner; cards are two lines with ages and diffstats aligned; the header collapses counts, then meter, then branch; the exit key is on screen at 60; a modal names itself; and once, with the real `claude` binary in a throwaway project, Claude's own prompt box sits in the pane without an omatty frame.
- [ ] **Step 3:** Attach the four screen dumps to #179 as a comment. Any defect found gets its own `fix` issue with a regression test before #172 closes.

### Task G2: ROADMAP and AGENTS.md

**Files:** `docs/ROADMAP.md`, `AGENTS.md`

- [ ] **Step 1:** Add `## M8 - Surface` after M7 in the shape of the M7 section: what is in it (one bullet per slice, each naming its PR), what was decided (direction A of three, the colour rule, the git poll; the M7 hues it replaced), what was cut (the spec's "Out of scope" list, verbatim). Point the status table at "What is left"; write no counts.
- [ ] **Step 2:** In AGENTS.md's label list change `` `M1` `M2` `M3` `M4` `M5` `M6` `M7` `` to include `` `M8` ``.
- [ ] **Step 3:** Commit `docs(#179): M8 in the roadmap and the label list`, push, open the PR with `Closes #172` and `Closes #179` in the body (two separate `Closes` lines, per the close-out memory), board to Review.

---

## Verification, end to end

Per slice, before its PR is opened:

1. The gate, exactly as CI runs it: `gofmt -l .` empty, `go vet ./...`, `golangci-lint run`, `go test ./... -race`, `./scripts/check-coverage.sh 90`.
2. The real-PTY smoke in a scratch short-path HOME with `testdata/fake-claude` on PATH (recipe in Task F4 step 4; `state.json` with two projects and three sessions, the projects' roots pointing at a real repository so slice F has a branch to show), read through `testdata/screen`, at the sizes each slice names. A row is right when it is exactly the width and the `|` ruler sits in the same column on every line.
3. Board moves: issue In Progress at start, PR in Review at open.

After slice G merges, the whole is verified by the three smokes in Task G1 plus one run with the real `claude` binary in a throwaway project, and by the spec's read list: no box, `┼` under every hairline, the accent on the keyboard owner, two-line cards aligned, the header collapsing counts then meter then branch, the exit key first at 60 columns, a modal naming itself, and Claude's own prompt box in the pane with no frame around it.

## Files touched, by slice

| Slice | Create | Modify | Delete |
|---|---|---|---|
| A | `internal/ui/hairline.go`, `internal/ui/frame_test.go` | `layout.go`, `render.go`, `reviewview.go`, `style.go`, `pan.go`, `wheel.go`, `export_test.go`, `layout_test.go`, `caret_test.go`, `sidebarclick_test.go`, `quit_test.go`, `wheel_test.go`, `wheelpan_test.go`, `model_test.go`, `render_test.go` | `internal/ui/panebox.go` |
| B | — | `style.go`, `ramp.go`, `style_test.go`, `ramp_test.go`, `status_test.go`, `export_test.go` | — |
| C | `internal/ui/card.go`, `internal/ui/card_test.go` | `sidebar.go`, `sidebarclick.go`, `render.go`, `lane.go`, `export_test.go`, `sidebar_test.go`, `sidebarclick_test.go`, `render_test.go`, `layout_test.go`, `reach_test.go`, `archive_test.go`, `meter_test.go` | — |
| D | `internal/ui/header.go`, `internal/ui/header_test.go` | `render.go`, `meter.go`, `modalview.go`, `export_test.go` | — |
| E | `internal/ui/footer_test.go` (and `footer.go` if render.go passes 500 lines) | `render.go`, `export_test.go` | — |
| F | `internal/vcs/export_test.go`, `internal/vcs/shortstat_test.go`, `internal/ui/repostat.go`, `internal/ui/repostat_test.go` | `vcs/git.go`, `vcs/dir_test.go`, `review/source.go`, `review/source_test.go`, `review/fakes_test.go`, `ui/deps.go`, `ui/run.go`, `ui/model.go`, `ui/status.go`, `ui/archive.go`, `ui/card.go`, `ui/header.go`, `ui/fakes_test.go`, `ui/export_test.go`, `cmd/omatty/wiring.go` | — |
| G | — | `docs/ROADMAP.md`, `AGENTS.md` | — |

Existing code reused rather than rewritten: `fitBlock`/`fitLine`/`padRight`/`clip` (render.go), `ScrollOffset`'s semantics (reimplemented in lines, not called), `revealHeader`, `landable`, `session`/`projectRoot`/`sessionIndex` (review.go), the `loadDiff` command shape and `refreshReview` trigger rule, the `namePending` guard pattern (autotitle.go), `review.Source.baseCommit`, `vcs.diffArgs`, `KString`/`AgeString`, `renderLane`, `renderMeter`/`cacheShare`, `modalFooter`'s kind switch as the model for `modalName`.
