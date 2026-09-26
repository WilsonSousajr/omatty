package ui_test

import (
	"strings"
	"testing"
)

// c cuts the tree to what the session changed, the title says the listing is
// narrowed, and c again brings the rest back (#430, broot's :gs).
func TestTree_CShowsOnlyTheChangedFiles_issue430(t *testing.T) {
	m, _, _, _ := modelWithTree(t)
	leader(m, key('f'))
	if !strings.Contains(m.View().Content, "go.mod") {
		t.Fatalf("the fixture's unchanged go.mod is not listed to begin with")
	}

	press(m, key('c'))

	body := m.View().Content
	if strings.Contains(body, "go.mod") || !strings.Contains(body, "model.go") {
		t.Errorf("c did not cut the tree to the changed files:\n%s", stripSGR(body))
	}
	if title := columnTitle(m); !strings.Contains(title, "changed") {
		t.Errorf("a narrowed tree does not say so: %q", title)
	}

	press(m, key('c'))
	if !strings.Contains(m.View().Content, "go.mod") {
		t.Errorf("a second c did not bring the unchanged files back")
	}
}
