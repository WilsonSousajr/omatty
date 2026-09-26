package ui_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// Every leader key must be reachable from inside omatty. The footer shows a
// working subset because it is truncated to the window (#30); this is where
// the rest lives (#103).
// The keys come from ui.LeaderKeys, the table helpLines itself renders, not
// from a list written out here: a hand-copied list is a third place the keymap
// lives, and the drift between two of them is what #103 was. A binding added
// to the router but not to that table is caught by
// TestModel_everyBoundLeaderKeyIsDocumented_issue103 below.
func TestModel_helpListsEveryLeaderKey_issue103(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})

	press(m, ctrl('o'))
	press(m, key('?'))

	got := m.View().Content
	for _, k := range ui.LeaderKeys() {
		if !strings.Contains(got, ui.DefaultLeader+" "+k) {
			t.Errorf("the help modal does not list %q:\n%s", ui.DefaultLeader+" "+k, got)
		}
	}
}

// Regression, issue #103: `ctrl+o f files` was invisible because the footer
// truncated and nothing tied the documented keymap to the bound one. This is
// that tie - every key modalCommand and paneCommand answer must appear in the
// help table, so adding a binding without documenting it fails here rather
// than shipping undiscoverable.
func TestModel_everyBoundLeaderKeyIsDocumented_issue103(t *testing.T) {
	documented := make(map[string]bool)
	for _, k := range ui.LeaderKeys() {
		for _, part := range strings.Split(k, " / ") {
			documented[strings.TrimSpace(part)] = true
		}
	}

	for _, bound := range boundLeaderKeys(t) {
		if !documented[bound] {
			t.Errorf("%q is bound in routing.go but not listed in the help modal", bound)
		}
	}
}

// boundLeaderKeys reads the case values out of routing.go's command
// switches. Parsing the source rather than probing a model is what makes this
// exhaustive: a switch arm cannot be enumerated at runtime, so a probe would
// only ever confirm the keys it already thought to try - which is the same
// blind spot that let a bound-but-undocumented key ship (#103).
func boundLeaderKeys(t *testing.T) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "routing.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing routing.go: %v", err)
	}
	var keys []string
	for _, name := range []string{"navigate", "cursorMove", "paneCommand", "lifecycleCommand", "modalCommand"} {
		keys = append(keys, caseStrings(findFunc(t, file, name))...)
	}
	return keys
}

// helpFitsHeight is a window tall enough to show the whole help modal without
// scrolling, for the tests that look for its last section. It grows with the
// keymap; #422's per-face sections put it at 65, and every key documented after
// that costs a row - #337's v and #338's g took it past 66, which is how
// TestModel_helpSaysHowToCopyOutOfAPane_issue190 noticed. Deliberately generous
// now: nothing here asserts the modal is *exactly* this tall, and the test that
// cares about scrolling uses a short window instead. #424's g / G and
// ctrl+d / ctrl+u took the minimum to 71, so it is 80: room for a few more.
const helpFitsHeight = 80

// Regression, issue #422: the help modal's review-column table was written for
// the diff and the tree, and nothing tied it to the handlers, so the gate's
// enter and S, the tracker's enter, n, a and b, and the diff's d all shipped
// with no row. This is #103's tie for the column: every key a face's handler
// switches on must appear in that face's table or in the shared one.
func TestHelp_everyColumnKeyIsDocumented_issue422(t *testing.T) {
	tables := ui.ColumnKeyTables()
	shared := documentedSet(tables["column"])
	for face, funcs := range columnHandlers {
		documented := documentedSet(tables[face])
		for _, bound := range boundKeysIn(t, funcs) {
			canonical := arrowAliases[bound]
			if canonical == "" {
				canonical = bound
			}
			if !documented[canonical] && !shared[canonical] {
				t.Errorf("%s: %q is handled but the help modal has no row for it", face, bound)
			}
		}
	}
}

// columnHandlers is every function that switches on a review-column key, by
// the face whose help section documents it. A handler split or renamed fails
// findFunc loudly, which is the point: the list cannot quietly go stale.
var columnHandlers = map[string]map[string][]string{
	"column":  {"pan.go": {"panKey"}, "listwindow.go": {"pageDelta"}},
	"diff":    {"reviewkeys.go": {"onReviewKey", "reviewAction"}},
	"tree":    {"treekeys.go": {"onTreeKey", "treeActionKey", "treeCursorKey", "treeMarkKey", "onPreviewKey"}},
	"gate":    {"gatepane.go": {"onGateKey"}},
	"tracker": {"tracker.go": {"onTrackerKey", "trackerCursorKey"}, "trackerwork.go": {"trackerAction"}, "trackeritem.go": {"onTrackerItemKey"}},
}

// arrowAliases are the arrow keys' spellings of j, k, h and l: one key an
// operator presses, documented once under its letter.
var arrowAliases = map[string]string{"down": "j", "up": "k", "left": "h", "right": "l"}

// documentedSet splits help rows such as "j / k" into the keys they name.
func documentedSet(rows []string) map[string]bool {
	set := make(map[string]bool)
	for _, row := range rows {
		for _, part := range strings.Split(row, " / ") {
			set[strings.TrimSpace(part)] = true
		}
	}
	return set
}

// boundKeysIn is the case strings of the named functions, file by file.
func boundKeysIn(t *testing.T, funcs map[string][]string) []string {
	t.Helper()
	var keys []string
	for name, fns := range funcs {
		file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, fn := range fns {
			keys = append(keys, caseStrings(findFunc(t, file, fn))...)
		}
	}
	return keys
}

// findFunc returns the named method's body.
func findFunc(t *testing.T, file *ast.File, name string) ast.Node {
	t.Helper()
	for _, d := range file.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return fn.Body
		}
	}
	t.Fatalf("no func %s in the parsed file; the keymap test needs updating", name)
	return nil
}

// caseStrings is every string literal in a switch case inside n, minus the
// modifier spellings: "shift+r" and "R" are one key an operator presses, and
// only the bare form is what the help modal lists.
func caseStrings(n ast.Node) []string {
	var out []string
	ast.Inspect(n, func(node ast.Node) bool {
		clause, ok := node.(*ast.CaseClause)
		if !ok {
			return true
		}
		for _, expr := range clause.List {
			lit, ok := expr.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			if key, err := strconv.Unquote(lit.Value); err == nil && !strings.Contains(key, "+") {
				out = append(out, key)
			}
		}
		return true
	})
	return out
}

// Regression, issue #103: the help modal first closed on *any* key, so the
// ctrl+o of `ctrl+o q` closed the help and the q went straight to Claude as
// text - the M4 smoke test found a literal q sitting in the pane. No key may
// leak into the PTY from inside a modal.
func TestModel_helpLeaksNoKeyIntoTheSession_issue103(t *testing.T) {
	m, fakes := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('?'))

	press(m, ctrl('o'))
	press(m, key('q'))

	if n := len(fakes["s1"].Msgs); n != 0 {
		t.Errorf("%d keys reached the PTY from inside the help modal, want 0", n)
	}
}

// The header row names the modal now (#177), so the body does not repeat it:
// its first line is the first binding, and the two rows the title and its
// blank spent go to the keymap (#188).
func TestModel_helpBodyDoesNotRepeatTheHeadersName_issue188(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	leader(m, key('?'))

	lines := frameLines(m)

	if head := stripSGR(lines[0]); !strings.Contains(head, "│ keys") {
		t.Fatalf("the header row does not name the modal: %q", head)
	}
	if body := stripSGR(lines[2]); strings.Contains(body, ui.DefaultLeader+" keys") || !strings.Contains(body, ui.DefaultLeader+" j / k") {
		t.Errorf("the help body's first line = %q, want the first binding, not the title", body)
	}
}

// esc closes the help modal; the header row stops naming it (#177).
func TestModel_escClosesTheHelpModal_issue103(t *testing.T) {
	m, _ := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('?'))

	press(m, special(tea.KeyEscape))

	if got := stripSGR(m.View().Content); strings.Contains(got, "│ keys") {
		t.Errorf("esc did not close the help modal:\n%s", got)
	}
}

// Regression, issue #103: this replaces an assertion that the leader was
// *swallowed* inside a modal. That behaviour was never right - it was the
// original bug wearing a fix. A modal leaves the terminal unfocused, so
// keys.Router could not arm the leader while one was open, which made `ctrl+o
// q` - the app's documented only exit - a silent no-op in the help box and a
// literal "q" appended to the buffer in the rename box. The leader now closes
// the surface and arms the next key, so the pair completes from inside any
// modal. Nothing still reaches the PTY, which is what the old test was really
// protecting.
func TestModel_theLeaderCompletesACommandFromInsideAModal_issue103(t *testing.T) {
	m, _ := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('?'))

	press(m, ctrl('o'))
	if got := stripSGR(m.View().Content); strings.Contains(got, "│ keys") {
		t.Errorf("the leader did not close the help modal:\n%s", got)
	}
	_, cmd := m.Update(key('q'))

	if !isQuit(cmd) {
		t.Error("ctrl+o q from inside the help modal did not quit")
	}
}

// The same contract for the rename box, where the old behaviour did not merely
// do nothing: it typed the command key into the session's new title.
func TestModel_theLeaderQuitsFromInsideTheRenameBox_issue41(t *testing.T) {
	m, _ := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('R'))

	press(m, ctrl('o'))
	_, cmd := m.Update(key('q'))

	if !isQuit(cmd) {
		t.Error("ctrl+o q from inside the rename box did not quit")
	}
	if got := m.View().Content; strings.Contains(got, "mainq") {
		t.Errorf("the command key was typed into the title instead:\n%s", got)
	}
}

// Regression, issue #103: the footer was 114 columns and truncated at 100, so
// `ctrl+o f files` was invisible and M4's four new keys would have taken it to
// 183. It is now a subset that fits, with the key that reaches the rest of the
// keymap early enough that truncation cannot reach it.
// The assertion is on the constants, not on the rendered line: renderFooter
// passes every footer through fitLine, which caps the result at m.width, so a
// check on the drawn frame passes for a footer of any length whatsoever and
// could not fail for this bug.
func TestModel_footerFitsTheDefaultWindow_issue103(t *testing.T) {
	for name, s := range ui.Footers(ui.DefaultLeader) {
		if w := lipgloss.Width(s); w > ui.DefaultWidth {
			t.Errorf("%s is %d columns wide, want at most %d:\n%s", name, w, ui.DefaultWidth, s)
		}
	}
}

// The help key must survive whatever truncation there is, or the rest of the
// keymap is unreachable from the screen.
func TestModel_theFooterShowsTheHelpKey_issue103(t *testing.T) {
	m, _ := modelWithFakes(t)
	m.Update(tea.WindowSizeMsg{Width: ui.DefaultWidth, Height: ui.DefaultHeight})

	lines := strings.Split(m.View().Content, "\n")
	last := lines[len(lines)-1]

	if !strings.Contains(last, ui.DefaultLeader+" ?") {
		t.Errorf("the footer does not show the help key at %d columns:\n%s", ui.DefaultWidth, last)
	}
}

// Issue #28, for the help modal.
func TestModel_ctrlCQuitsWhileHelpIsOpen_issue28(t *testing.T) {
	m, _ := modelWithFakes(t)
	press(m, ctrl('o'))
	press(m, key('?'))

	_, cmd := m.Update(ctrl('c'))

	if !isQuit(cmd) {
		t.Error("ctrl+c while the help modal is open did not quit")
	}
}
