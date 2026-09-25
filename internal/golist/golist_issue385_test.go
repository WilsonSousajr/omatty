package golist_test

import (
	"slices"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/golist"
)

// The staleness guard compares the profile against every file that can change
// coverage, and a test file changes it without touching any source. go list
// keeps those in two further fields; carrying only GoFiles is what let a stale
// profile through (#385).
func TestList_carriesTestFilesSoStalenessCanSeeThem_issue385(t *testing.T) {
	pkgs, err := golist.List("..", "./golist")
	if err != nil {
		t.Fatal(err)
	}
	pkg, ok := packageNamed(pkgs, "github.com/WilsonSousajr/omatty/internal/golist")
	if !ok {
		t.Fatal("internal/golist is not in its own listing")
	}

	if !slices.Contains(pkg.XTestGoFiles, "golist_issue385_test.go") {
		t.Errorf("XTestGoFiles = %v, want this file; package golist_test is an external test package",
			pkg.XTestGoFiles)
	}
	for _, name := range pkg.GoFiles {
		if slices.Contains(pkg.XTestGoFiles, name) || slices.Contains(pkg.TestGoFiles, name) {
			t.Errorf("GoFiles and the test lists both hold %q; go list keeps them apart", name)
		}
	}
}

func packageNamed(pkgs []golist.Package, path string) (golist.Package, bool) {
	for _, p := range pkgs {
		if p.ImportPath == path {
			return p, true
		}
	}
	return golist.Package{}, false
}
