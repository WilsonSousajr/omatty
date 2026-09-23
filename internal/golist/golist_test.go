package golist_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/golist"
)

// The gate tools need the file set the compiler actually agrees to, which is
// why they ask go list rather than walking the tree: internal/gate holds
// procgroup_other.go behind //go:build !unix, and a walk would hand a scorer a
// file neither CI runner ever compiles.
func TestList_reportsOnlyTheFilesThisPlatformCompiles(t *testing.T) {
	pkgs, err := golist.List("..", "./gate")
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("got %d packages, want 1: %+v", len(pkgs), pkgs)
	}

	files := strings.Join(pkgs[0].GoFiles, " ")
	if !strings.Contains(files, "gate.go") {
		t.Errorf("GoFiles = %v, want it to hold gate.go", pkgs[0].GoFiles)
	}
	if strings.Contains(files, "_test.go") {
		t.Errorf("GoFiles = %v, want no test files; go list keeps those apart", pkgs[0].GoFiles)
	}
	if strings.Contains(files, "procgroup_other.go") {
		t.Error("GoFiles holds a file excluded by a build constraint on this platform")
	}
}

// go list -json writes a stream of concatenated objects rather than a JSON
// array, so anything decoding it with Unmarshal gets only the first package.
func TestList_readsEveryPackageInTheStream(t *testing.T) {
	pkgs, err := golist.List("..", "./paths", "./fuzzy", "./keys")

	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) != 3 {
		t.Fatalf("got %d packages, want 3: %+v", len(pkgs), pkgs)
	}
	for _, p := range pkgs {
		if p.ImportPath == "" || p.Dir == "" || len(p.GoFiles) == 0 {
			t.Errorf("package %+v came back incomplete", p)
		}
	}
}

// A pattern matching nothing is not a failure - the caller asked a question and
// the answer is "no packages" - but a pattern go list rejects outright is, and
// the message has to name what went wrong or the gate step is unreadable.
func TestList_surfacesWhatGoListSaidWhenItFails(t *testing.T) {
	_, err := golist.List("..", "./no-such-package-anywhere")

	if err == nil {
		t.Fatal("List() error = nil, want the failure surfaced")
	}
	if !strings.Contains(err.Error(), "golist:") {
		t.Errorf("error = %q, want it to name the package doing the work", err)
	}
	if !strings.Contains(err.Error(), "no-such-package-anywhere") {
		t.Errorf("error = %q, want it to name the pattern that failed", err)
	}
}

// A coverage profile keys its records by import path, so translating between a
// profile and a file name needs the module path from the toolchain rather than
// a constant somebody has to remember to update.
func TestModule_answersTheModulePath(t *testing.T) {
	module, err := golist.Module("..")

	if err != nil {
		t.Fatal(err)
	}
	if want := "github.com/WilsonSousajr/omatty"; module != want {
		t.Errorf("Module() = %q, want %q", module, want)
	}
}

// internal/gate holds procgroup_other.go behind //go:build !unix. A report that
// never mentions the files it skipped is how a metric silently stops covering
// half a package.
func TestList_reportsFilesABuildConstraintExcluded(t *testing.T) {
	pkgs, err := golist.List("..", "./gate")
	if err != nil {
		t.Fatal(err)
	}

	if len(pkgs[0].IgnoredGoFiles) == 0 {
		t.Skip("no build-constrained files in internal/gate on this platform")
	}
	if !strings.Contains(strings.Join(pkgs[0].IgnoredGoFiles, " "), "procgroup_other.go") {
		t.Errorf("IgnoredGoFiles = %v, want procgroup_other.go", pkgs[0].IgnoredGoFiles)
	}
}

// Outside a module there is no module path, and that has to surface: a caller
// silently handed "" would translate every profile record to the wrong file.
func TestModule_outsideAModuleIsAnError(t *testing.T) {
	_, err := golist.Module(t.TempDir())

	if err == nil {
		t.Fatal("Module() error = nil, want the failure surfaced")
	}
	if !strings.Contains(err.Error(), "golist:") {
		t.Errorf("error = %q, want it to name the package doing the work", err)
	}
}

// Imports must be direct, not transitive: efferent coupling computed from
// `go list -deps` would count the whole reachable graph as one package's
// dependencies and make every metric derived from it meaningless.
func TestList_reportsDirectImportsOnly(t *testing.T) {
	pkgs, err := golist.List("..", "./paths")
	if err != nil {
		t.Fatal(err)
	}

	for _, imported := range pkgs[0].Imports {
		if strings.Contains(imported, "/internal/") {
			t.Errorf("internal/paths imports %s; it is meant to be a pure leaf", imported)
		}
	}
}

// Test imports are a separate question from production structure, and go list
// keeps them in separate fields precisely so a caller need not untangle them.
func TestList_keepsTestImportsApartFromProductionImports(t *testing.T) {
	pkgs, err := golist.List("..", "./golist")
	if err != nil {
		t.Fatal(err)
	}

	production := strings.Join(pkgs[0].Imports, " ")
	if strings.Contains(production, "testing") {
		t.Errorf("Imports = %v, want no testing package in production imports", pkgs[0].Imports)
	}
	if len(pkgs[0].TestImports)+len(pkgs[0].XTestImports) == 0 {
		t.Error("this package has tests, so some test imports were expected")
	}
}
