package main

import (
	"github.com/WilsonSousajr/omatty/internal/domain/depgraph"
	"github.com/WilsonSousajr/omatty/internal/infra/golist"
)

// forDepgraph is go list's answer in the shape depgraph reads: depgraph is
// domain and runs nothing, so this tool lists the packages and hands them
// over (ADR 0001, migration step 3.9).
func forDepgraph(pkgs []golist.Package) []depgraph.Package {
	out := make([]depgraph.Package, len(pkgs))
	for i, p := range pkgs {
		out[i] = depgraph.Package{ImportPath: p.ImportPath, Imports: p.Imports,
			TestImports: p.TestImports, XTestImports: p.XTestImports}
	}
	return out
}
