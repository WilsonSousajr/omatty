package ui_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/termwrap"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

// keyContext is one place a key can land: a scene plus, for the in-column
// editors, the keys that open them. want is a piece of the model's
// Fingerprint the context must show before any key is pressed, so a context
// that silently fails to set up is an error rather than 186 rows of "nothing
// happened".
type keyContext struct {
	name string
	in   sceneIn
	want string
}

var keyContexts = []keyContext{
	{"terminal", terminalScene, "focus=0/true armed=false modal=\"\" view=0 sel=s1 open=false"},
	{"leader", func(t *testing.T, terms map[string]termwrap.Terminal, w, h int) *ui.Model {
		m := terminalScene(t, terms, w, h)
		press(m, ctrl('o'))
		return m
	}, "armed=true"},
	{"diff", pressingIn(diffScene, key('j'), key('j')), "focus=1/true armed=false modal=\"\" view=0 sel=s1 open=true focused=true zoom=false diff=2"},
	{"diff_note", pressingIn(diffScene, key('j'), key('j'), key('j'), key('j'), key('j'), key('c')), "diff=5 files=0 gate=0 tracker=-1 filter=\"\" note=true"},
	{"tree", pressingIn(treeScene, key('j')), "focus=1/true armed=false modal=\"\" view=1 sel=s1 open=true focused=true zoom=false diff=0 files=1"},
	{"tree_filter", pressingIn(treeScene, key('/')), "focus=3/true"},
	{"preview", previewScene, "view=2"},
	{"gate", pressingIn(gateScene, key('j')), "view=3 sel=s1 open=true focused=true zoom=false diff=0 files=0 gate=1"},
	{"tracker", pressingIn(trackerScene, key('j')), "view=4 sel=s1 open=true focused=true zoom=false diff=0 files=0 gate=0 tracker=1"},
	{"tracker_item", trackerItemScene, "view=5"},
	{"help", then(terminalScene, '?'), "modal=\"keys\""},
	{"new_session", then(terminalScene, 'n'), "modal=\"new session\""},
	{"switcher", then(terminalScene, '/'), "modal=\"switch\""},
}

// pressingIn runs in and then presses keys in the pane.
func pressingIn(in sceneIn, keys ...tea.KeyPressMsg) sceneIn {
	return func(t *testing.T, terms map[string]termwrap.Terminal, w, h int) *ui.Model {
		m := in(t, terms, w, h)
		for _, k := range keys {
			pressAndSettle(m, k)
		}
		return m
	}
}

// allKeys is every key a person can plausibly press. A shifted letter comes
// in three spellings - modifier and upper text, a legacy terminal's bare upper
// code, and the modifier alone - and the router has accepted each (#87), so
// each gets its own row.
func allKeys() []tea.KeyPressMsg {
	var out []tea.KeyPressMsg
	for r := 'a'; r <= 'z'; r++ {
		up := string(r - 'a' + 'A')
		out = append(out, key(r),
			tea.KeyPressMsg{Code: r, Mod: tea.ModShift, Text: up},
			tea.KeyPressMsg{Code: r - 'a' + 'A', Text: up},
			tea.KeyPressMsg{Code: r, Mod: tea.ModShift},
			ctrl(r))
	}
	for _, r := range "0123456789`~!@#$%^&*()-_=+[]{}\\|;:'\",.<>/? " {
		out = append(out, key(r))
	}
	for _, c := range []rune{tea.KeyEnter, tea.KeyEscape, tea.KeyTab, tea.KeyBackspace, tea.KeyUp, tea.KeyDown,
		tea.KeyLeft, tea.KeyRight, tea.KeyPgUp, tea.KeyPgDown, tea.KeyHome, tea.KeyEnd} {
		out = append(out, special(c))
	}
	return append(out, tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift})
}

// Regression net, issue #620: pins what every key does in every focus context
// before ADR 0001 moves the 24 switch-key sites onto key.Binding. A binding
// that drops one spelling changes a row here; so does a key that stops
// reaching the PTY while the terminal has focus (invariant 1).
func TestKeys_everyKeyInEveryContextIsPinned_issue620(t *testing.T) {
	var rows []string
	for _, c := range keyContexts {
		for _, k := range allKeys() {
			rows = append(rows, c.name+" "+keyRow(t, c, k))
		}
	}
	if got, want := len(rows), len(keyContexts)*len(allKeys()); got != want {
		t.Fatalf("%d rows, want %d", got, want)
	}
	assertGolden(t, "keys.golden", []byte(strings.Join(rows, "\n")+"\n"))
}

// observe is what a row records of the model: its fingerprint, which names
// the state, plus a hash of the frame, which catches what the fingerprint
// cannot name - typed text, folded directories, pan, scroll.
func observe(m *ui.Model) string {
	sum := sha256.Sum256([]byte(m.View().Content))
	return m.Fingerprint() + " frame=" + hex.EncodeToString(sum[:4])
}

// keyRow presses k in a fresh c and says what happened: how many messages the
// selected session's PTY received, which message types the key's commands
// produced, and the model's fingerprint afterwards.
func keyRow(t *testing.T, c keyContext, k tea.KeyPressMsg) string {
	terms, fakes := fakeTerms(t)
	m := c.in(t, terms, 120, 30)
	if fp := m.Fingerprint(); !strings.Contains(fp, c.want) {
		t.Fatalf("context %s did not set up: fingerprint %s lacks %s", c.name, fp, c.want)
	}
	sel := m.Selected()
	before := ptyCount(fakes, sel)
	_, cmd := m.Update(k)
	msgs := settleRecording(m, cmd, 0)
	return fmt.Sprintf("%q code=%d mod=%d text=%q -> pty=%d msgs=%s %s",
		k.String(), k.Code, k.Mod, k.Text, ptyCount(fakes, sel)-before, strings.Join(msgs, ","), observe(m))
}

func ptyCount(fakes map[string]*termwrap.Fake, id string) int {
	if f, ok := fakes[id]; ok {
		return len(f.Msgs)
	}
	return 0
}

// settleRecording is settle that names each message type it feeds back,
// sorted, so a key's side effects (a quit, a clipboard write, a load) are in
// the row. A spin tick is dropped, as settle drops it, and the depth bound
// turns a command that re-arms itself into a visible "..." instead of a hang.
func settleRecording(m *ui.Model, cmd tea.Cmd, depth int) []string {
	if cmd == nil {
		return nil
	}
	if depth > 16 {
		return []string{"..."}
	}
	var seen []string
	switch msg := cmd().(type) {
	case nil, ui.SpinTickMsg:
	case tea.BatchMsg:
		for _, c := range msg {
			seen = append(seen, settleRecording(m, c, depth+1)...)
		}
	default:
		seen = append(seen, reflect.TypeOf(msg).String())
		_, next := m.Update(msg)
		seen = append(seen, settleRecording(m, next, depth+1)...)
	}
	sort.Strings(seen)
	return seen
}

// In a context that takes text, a typed letter must change the row: a table
// that cannot tell "inserted a" from "dropped a" pins nothing about the one
// behaviour a key.Binding migration is likeliest to break (#620 review).
func TestKeys_typingIsObservableInTextContexts_issue620(t *testing.T) {
	for _, c := range keyContexts {
		if !textContexts[c.name] {
			continue
		}
		terms, _ := fakeTerms(t)
		m := c.in(t, terms, 120, 30)
		before := observe(m)
		pressAndSettle(m, key('a'))
		if observe(m) == before {
			t.Errorf("typing a in %s changes nothing the table records", c.name)
		}
	}
}

var textContexts = map[string]bool{"diff_note": true, "tree_filter": true, "new_session": true, "switcher": true}
