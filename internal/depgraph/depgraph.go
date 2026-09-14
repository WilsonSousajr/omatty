// Package depgraph measures the shape of omatty's own import graph, using
// Robert Martin's package metrics.
//
//	g := depgraph.Build(module, pkgs)
//	violations, edge, margin := g.Violations(), g.Margin()
//
// The metrics are afferent coupling (Ca, how many packages import this one),
// efferent coupling (Ce, how many it imports), and instability
// I = Ce / (Ca + Ce). A package nothing depends on and which depends on much is
// unstable - it is cheap to change. A pure leaf everything depends on is stable
// - changing it is expensive.
//
// The Stable Dependencies Principle follows: for every edge A -> B,
// I(A) >= I(B). Depend in the direction of stability, never against it, or an
// easily-changed package is pinned by something that cannot move.
//
// omatty already obeys it. cmd -> ui -> supervisor -> agent -> watcher ->
// registry -> {gate, paths, vcs} is a clean monotonic descent, which is why
// this package exists: the property is currently accidental, and writing it
// down is what keeps it.
//
// What is deliberately NOT here is distance from the main sequence. Go declares
// interfaces at the consumer and usually unexported, so a stable pure leaf like
// internal/paths scores maximum distance while being exactly what AGENTS.md
// designed it to be. Gating on that number would demand precisely the
// speculative interfaces AGENTS.md bans (#263).
package depgraph

import "github.com/WilsonSousajr/omatty/internal/golist"

// Node is one package's coupling.
type Node struct {
	// ImportPath names the package.
	ImportPath string
	// Afferent is Ca: how many packages in this module import it.
	Afferent int
	// Efferent is Ce: how many packages in this module it imports.
	Efferent int
}

// Instability is Ce / (Ca + Ce), from 0 (nothing depends outward, everything
// depends on it) to 1.
//
// A package with no coupling at all is stable rather than undefined: nothing
// can break by depending on something that depends on nothing.
func (n Node) Instability() float64 {
	total := n.Afferent + n.Efferent
	if total == 0 {
		return 0
	}
	return float64(n.Efferent) / float64(total)
}

// Edge is one import, from the package that declares it to the one it names.
type Edge struct{ From, To string }

// Violation is an edge that depends against the direction of stability.
type Violation struct{ From, To Node }

// Graph is the module's own import structure.
type Graph struct {
	Nodes map[string]Node
	Edges []Edge
}

// Build reads the module-internal import graph out of go list's answer.
//
// Only imports carrying the module's own prefix are counted. Counting fmt and
// os as efferent coupling would make every leaf look unstable and the metric
// would say nothing about this repository's design.
func Build(modulePath string, pkgs []golist.Package) Graph {
	g := Graph{Nodes: map[string]Node{}}
	for _, pkg := range pkgs {
		g.Nodes[pkg.ImportPath] = Node{ImportPath: pkg.ImportPath}
	}
	for _, pkg := range pkgs {
		for _, imported := range internalImports(modulePath, pkg.Imports) {
			if _, known := g.Nodes[imported]; !known {
				continue
			}
			g.Edges = append(g.Edges, Edge{From: pkg.ImportPath, To: imported})
			g.bump(pkg.ImportPath, imported)
		}
	}
	return g
}

// bump records one edge's contribution to both endpoints' coupling.
func (g Graph) bump(from, to string) {
	source := g.Nodes[from]
	source.Efferent++
	g.Nodes[from] = source

	target := g.Nodes[to]
	target.Afferent++
	g.Nodes[to] = target
}
