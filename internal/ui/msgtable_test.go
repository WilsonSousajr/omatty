package ui_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/gate"
	"github.com/WilsonSousajr/omatty/internal/review"
	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
	"github.com/WilsonSousajr/omatty/internal/watcher"
)

// routedMsg is one representative value per case in msgroute.go, named the
// way the case spells its type so the table can be checked against the file.
type routedMsg struct {
	caseName string
	msg      tea.Msg
}

func routedMsgs() []routedMsg {
	now := fixedNow
	out := []routedMsg{
		{"TickMsg", ui.TickMsg(now)}, {"SpinTickMsg", ui.SpinTickMsg(now)},
		{"StatTickMsg", ui.StatTickMsg(now)}, {"PRTickMsg", ui.PRTickMsg(now)},
		{"IssueTickMsg", ui.IssueTickMsg(now)}, {"SweepTickMsg", ui.SweepTickMsg(now)},
		{"tea.KeyPressMsg", key('j')}, {"tea.MouseMsg", tea.MouseClickMsg{X: 5, Y: 3, Button: tea.MouseLeft}},
		{"tea.PasteMsg", tea.PasteMsg{Content: "pasted"}},
		{"tea.PasteStartMsg", tea.PasteStartMsg{}}, {"tea.PasteEndMsg", tea.PasteEndMsg{}},
		{"tea.WindowSizeMsg", tea.WindowSizeMsg{Width: 100, Height: 28}},
		{"DiffLoadedMsg", ui.DiffLoadedMsg{SessionID: "s1", Diff: review.Diff{}}},
		{"FilesLoadedMsg", ui.FilesLoadedMsg{SessionID: "s1", Paths: []string{"go.mod"}}},
		{"WorktreeRemovedMsg", ui.WorktreeRemovedMsg{SessionID: "s1", Dir: "/p/omatty"}},
		{"TurnLoadedMsg", ui.TurnLoadedMsg{SessionID: "s1"}}, {"TurnSnappedMsg", ui.TurnSnappedMsg{SessionID: "s1"}},
		{"NamedMsg", ui.NamedMsg{SessionID: "s1", From: "main", Title: "named"}},
		{"ModelNamedMsg", ui.ModelNamedMsg{SessionID: "s1", From: "main", Title: "model named"}},
		{"BranchNamedMsg", ui.BranchNamedMsg{SessionID: "s1", Branch: "feat/x", Renamed: true}},
		{"StatusMsg", ui.StatusMsg(watcher.Event{SessionID: "s1", Kind: watcher.TurnEnded, At: now})},
		{"GateMsg", ui.GateMsg(gate.Report{ID: "s1"})},
		{"RevertedMsg", ui.RevertedMsg{SessionID: "s1", Files: 1}},
		{"ShippedMsg", ui.ShippedMsg{SessionID: "s1", Number: 7}},
		{"ClipboardMsg", ui.ClipboardMsg{SessionID: "s1"}},
		{"ProjectsProposedMsg", ui.ProjectsProposedMsg{}}, {"SessionsProposedMsg", ui.SessionsProposedMsg{}},
		{"RepoStatMsg", ui.RepoStatMsg{SessionID: "s1", Stat: review.Stat{Added: 3, Removed: 1}}},
		{"PRsLoadedMsg", ui.PRsLoadedMsg{Project: "/p/omatty", PRs: []forge.PR{{Number: 9, Title: "a pr", State: forge.Open}}}},
		{"IssuesLoadedMsg", ui.IssuesLoadedMsg{Project: "/p/omatty", Issues: []forge.Issue{{Number: 8, Title: "an issue"}}}},
		{"ItemLoadedMsg", ui.ItemLoadedMsg{}}, {"BrowsedMsg", ui.BrowsedMsg{Number: 9}},
		{"tea.FocusMsg", tea.FocusMsg{}}, {"tea.BlurMsg", tea.BlurMsg{}},
	}
	for name, msg := range ui.UnexportedRouted("s1") {
		out = append(out, routedMsg{name, msg})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].caseName < out[j].caseName })
	return append(out, routedMsg{"(unrouted)", struct{ unrouted bool }{true}})
}

// Regression net, issue #620: pins what every message Update routes does to
// the model before ADR 0001 replaces the on* chain with a root router and
// screens. Each row: the message, what its command yields one level down, and
// the model's fingerprint after it, from the diff scene.
func TestUpdate_everyRoutedMessageIsPinned_issue620(t *testing.T) {
	var rows []string
	for _, r := range routedMsgs() {
		rows = append(rows, fmt.Sprintf("%-20s -> %s", r.caseName, msgOutcome(t, r.msg)))
	}
	assertGolden(t, "msgs.golden", []byte(strings.Join(rows, "\n")+"\n"))
}

// msgOutcome delivers msg to a fresh diff scene and says what it did: what the
// command yields one level down, whether the frame changed, and the model's
// fingerprint afterwards.
//
// pty is how many messages reached the fake terminals: Update's fallthrough
// broadcasts to every terminal, so a case the router drops shows up here even
// when the model does not change.
func msgOutcome(t *testing.T, msg tea.Msg) string {
	terms, fakes := fakeTerms(t)
	m := diffScene(t, terms, 120, 30)
	sentBefore := ptyTotal(fakes)
	before := m.View().Content
	_, cmd := m.Update(msg)
	return fmt.Sprintf("cmd=%s pty=%d frameChanged=%t %s", cmdYield(cmd), ptyTotal(fakes)-sentBefore,
		m.View().Content != before, observe(m))
}

func ptyTotal(fakes map[string]*termwrap.Fake) int {
	n := 0
	for _, f := range fakes {
		n += len(f.Msgs)
	}
	return n
}

// A routed message whose row reads exactly like the unrouted one pins nothing:
// the router could drop its case and the table would not move. Each routed
// row must differ from the fallthrough's (#620 review).
func TestUpdate_everyRoutedRowDiffersFromTheFallthrough_issue620(t *testing.T) {
	msgs := routedMsgs()
	fallthroughRow := msgOutcome(t, msgs[len(msgs)-1].msg)
	for _, r := range msgs[:len(msgs)-1] {
		if msgOutcome(t, r.msg) == fallthroughRow {
			t.Errorf("%s reads exactly like an unrouted message: its row pins nothing", r.caseName)
		}
	}
}

// The table must have a row for every case in msgroute.go, so a message type
// added to the router without one fails here.
func TestUpdate_tableCoversEveryRoutedCase_issue620(t *testing.T) {
	have := map[string]bool{}
	for _, r := range routedMsgs() {
		have[r.caseName] = true
	}
	for _, name := range routedCases(t) {
		if !have[name] {
			t.Errorf("msgroute.go routes %s but the message table has no row for it", name)
		}
	}
}

// routedCases is every type named in a type-switch case in msgroute.go.
func routedCases(t *testing.T) []string {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "msgroute.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	ast.Inspect(f, func(n ast.Node) bool {
		if cc, ok := n.(*ast.CaseClause); ok {
			for _, e := range cc.List {
				names = append(names, exprName(e))
			}
		}
		return true
	})
	return names
}

func exprName(e ast.Expr) string {
	if sel, ok := e.(*ast.SelectorExpr); ok {
		return exprName(sel.X) + "." + sel.Sel.Name
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return fmt.Sprintf("%T", e)
}

// cmdDeadline is how long cmdYield waits for a command before calling it a
// timer. It must sit well under every tick the model arms - a test holds it to
// half of the shortest - and well over what a fake takes, which is
// microseconds.
const cmdDeadline = 50 * time.Millisecond

// cmdYield names what cmd produces one level down, sorted. A leaf that has not
// answered within cmdDeadline is a timer, recorded as pending, not waited for.
func cmdYield(cmd tea.Cmd) string {
	if cmd == nil {
		return "nil"
	}
	msg, ok := runWithin(cmd, cmdDeadline)
	if !ok {
		return "pending"
	}
	batch, isBatch := msg.(tea.BatchMsg)
	if !isBatch {
		return typeName(msg)
	}
	var parts []string
	for _, c := range batch {
		parts = append(parts, cmdYield(c))
	}
	sort.Strings(parts)
	return "[" + strings.Join(parts, ",") + "]"
}

func runWithin(cmd tea.Cmd, d time.Duration) (tea.Msg, bool) {
	out := make(chan tea.Msg, 1)
	go func() { out <- cmd() }()
	select {
	case msg := <-out:
		return msg, true
	case <-time.After(d):
		return nil, false
	}
}

func typeName(msg tea.Msg) string {
	if msg == nil {
		return "none"
	}
	return reflect.TypeOf(msg).String()
}

// The first version of this table assumed the shortest real tick was a
// second; previewRest is 250 ms, exactly the deadline, so a row that armed it
// would flip between pending and previewRestMsg from run to run (#620 review).
func TestUpdate_deadlineIsWellUnderEveryTick_issue620(t *testing.T) {
	for _, p := range ui.TickPeriods() {
		if p < 2*cmdDeadline {
			t.Errorf("a %v tick is within twice the %v deadline: a row arming it could flip", p, cmdDeadline)
		}
	}
}
