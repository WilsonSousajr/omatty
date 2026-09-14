package depgraph

import (
	"sort"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/golist"
)

// TestCycles returns import cycles that run through a package's tests.
//
// The production graph is not checked, deliberately: the Go compiler refuses a
// cycle there, so a check could never fire and would be dead code pretending to
// be a gate. The test graph is the one a Go repository can actually have -
// `a_test` may import `b` while `b` imports `a`, which compiles, runs, and
// couples two packages in a direction their production code does not admit to.
// Nothing looks at it today.
func TestCycles(modulePath string, pkgs []golist.Package) [][]string {
	edges := map[string][]string{}
	for _, pkg := range pkgs {
		all := append(append([]string{}, pkg.Imports...), pkg.TestImports...)
		all = append(all, pkg.XTestImports...)
		edges[pkg.ImportPath] = without(internalImports(modulePath, all), pkg.ImportPath)
	}
	var found [][]string
	for _, start := range sorted(edges) {
		if path, cyclic := walk(edges, start, start, nil, map[string]bool{}); cyclic {
			found = append(found, path)
		}
	}
	return found
}

// walk looks for a path from current back to target.
func walk(edges map[string][]string, target, current string, path []string, seen map[string]bool) ([]string, bool) {
	for _, next := range edges[current] {
		if next == target {
			return append(append([]string{}, path...), current, target), true
		}
		if seen[next] {
			continue
		}
		seen[next] = true
		if found, cyclic := walk(edges, target, next, append(path, current), seen); cyclic {
			return found, true
		}
	}
	return nil, false
}

// without drops a package's reference to itself.
//
// `package foo_test` importing `foo` is the standard external-test-package
// idiom, and go list reports it in XTestImports - so every tested package in
// the repository looks like a one-node cycle unless this is dropped. A real
// cycle runs through at least one other package.
func without(imports []string, self string) []string {
	kept := make([]string, 0, len(imports))
	for _, imported := range imports {
		if imported != self {
			kept = append(kept, imported)
		}
	}
	return kept
}

// internalImports keeps only the imports belonging to this module.
func internalImports(modulePath string, imports []string) []string {
	prefix := strings.TrimSuffix(modulePath, "/") + "/"
	var own []string
	for _, imported := range imports {
		if strings.HasPrefix(imported, prefix) {
			own = append(own, imported)
		}
	}
	return own
}

// sorted keys a map in a stable order, so two runs report the same cycle first.
func sorted(edges map[string][]string) []string {
	keys := make([]string, 0, len(edges))
	for key := range edges {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
