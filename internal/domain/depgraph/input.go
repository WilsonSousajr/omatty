package depgraph

// Package is one package as depgraph measures it: its import path and what it
// imports, in production and in its tests. tools/depcheck fills it from go
// list (ADR 0001, migration step 3.9), so depgraph itself never runs anything.
//
//	g := depgraph.Build(module, []depgraph.Package{{ImportPath: "m/a", Imports: []string{"m/b"}}})
type Package struct {
	ImportPath   string
	Imports      []string
	TestImports  []string
	XTestImports []string
}
