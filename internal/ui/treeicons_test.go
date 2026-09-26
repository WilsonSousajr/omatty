package ui_test

import (
	"strings"
	"testing"
)

// With icons = "nerd" each tree row carries a glyph for its kind: a folder
// open or shut, a Go file, and a fallback for anything unknown (#431).
func TestTree_NerdIconsMarkEachRowsKind_issue431(t *testing.T) {
	m, _, _, _ := modelWithTree(t)
	m.UseNerdIcons()
	leader(m, key('f'))

	body := stripSGR(m.View().Content)
	for _, want := range []string{"\uf07c internal/", "\ue627 model.go", "\ue627 go.mod", "\uf15c new.txt"} {
		if !strings.Contains(body, want) {
			t.Errorf("no row reads %q:\n%s", want, body)
		}
	}
}

// Without the key nothing changes: no private-use glyph anywhere in the
// tree, since in a terminal without a Nerd Font each would be a tofu box.
func TestTree_PlainIconsLeaveTheTreeAsItWas_issue431(t *testing.T) {
	m, _, _, _ := modelWithTree(t)
	leader(m, key('f'))

	for _, r := range stripSGR(m.View().Content) {
		if r >= 0xe000 && r <= 0xf8ff {
			t.Fatalf("a private-use glyph %U is drawn without icons = \"nerd\"", r)
		}
	}
	if !strings.Contains(stripSGR(m.View().Content), "▾ internal/") {
		t.Errorf("the plain tree lost its shape:\n%s", stripSGR(m.View().Content))
	}
}
