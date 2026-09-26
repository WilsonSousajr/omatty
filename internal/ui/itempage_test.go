package ui_test

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/WilsonSousajr/omatty/internal/forge"
	"github.com/WilsonSousajr/omatty/internal/ui"
)

const sgrMutedText = "\x1b[38;5;245m"

// openItem reads the row under the cursor in full.
func openItem(m *ui.Model) { pressDeliver(m, special(tea.KeyEnter)) }

// The item reads like a page: its title bold, who opened it when muted (#433).
func TestItem_TitleIsBoldAndTheByLineMuted_issue433(t *testing.T) {
	m, _ := itemModel(t)
	openItem(m)

	body := m.View().Content
	if !strings.Contains(body, ui.Bold("#397  read one item")) {
		t.Errorf("the item's title is not bold:\n%s", body)
	}
	if !strings.Contains(body, sgrMutedText+"opened by WilsonSousajr") {
		t.Errorf("the by-line is not muted:\n%s", body)
	}
}

// Light markdown, and only what reads better in a terminal: a heading bold, a
// bullet a bullet, fenced code muted, **strong** bold. No renderer: these are
// line rules on text #483 has already made plain.
func TestItem_BodyGetsLightMarkdown_issue433(t *testing.T) {
	m, items := itemModel(t)
	d := items.Issues[397]
	d.Body = "# Why\n- first point\n* second point\n```\ncode here\n```\nthis is **strong** text"
	items.Issues[397] = d
	openItem(m)

	body := m.View().Content
	plain := stripSGR(body)
	for _, want := range []string{"• first point", "• second point"} {
		if !strings.Contains(plain, want) {
			t.Errorf("no line reads %q:\n%s", want, plain)
		}
	}
	if strings.Contains(plain, "# Why") || !strings.Contains(body, ui.Bold("Why")) {
		t.Errorf("the heading is not drawn bold without its #:\n%s", body)
	}
	if !strings.Contains(body, sgrMutedText+"code here") || strings.Contains(plain, "```") {
		t.Errorf("fenced code is not muted, or its fence is still drawn:\n%s", body)
	}
	if !strings.Contains(body, ui.Strong("strong")) || strings.Contains(plain, "**") {
		t.Errorf("**strong** is not drawn bold:\n%s", body)
	}
}

// A pull request lists its checks in the gate's row style, failing first,
// under a count - the gate and the forge speak one vocabulary (#425, #433).
func TestItem_APullRequestListsItsChecks_issue433(t *testing.T) {
	m, items := itemModel(t)
	d := items.PRs[400]
	d.Checks = []forge.Check{
		{Name: "lint", State: forge.CIPassing, Took: 3 * time.Second},
		{Name: "test", State: forge.CIFailing, Took: 102 * time.Second},
	}
	items.PRs[400] = d
	pressDeliver(m, key('j')) // #400, past the pull requests heading
	openItem(m)

	plain := stripSGR(m.View().Content)
	summary, fail, pass := strings.Index(plain, "checks · 1 failing · 1 passed"), strings.Index(plain, "✗ test"), strings.Index(plain, "✓ lint")
	if summary < 0 || fail < 0 || pass < 0 || summary >= fail || fail >= pass {
		t.Errorf("want the count, then test (failing), then lint:\n%s", plain)
	}
	if !strings.Contains(plain, "1m42s") {
		t.Errorf("a check's duration is not shown:\n%s", plain)
	}
}
