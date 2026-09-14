package depgraph

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// Report writes the coupling table, the tightest edge, and any violations.
//
//	depgraph.Report(os.Stdout, g)
//
// The table is what belongs in docs/ARCHITECTURE.md: the seams that page
// describes in prose, as numbers. The margin line is what a reader watches
// between releases.
func Report(w io.Writer, g Graph) {
	_, _ = fmt.Fprintf(w, "%-34s %4s %4s %6s\n", "PACKAGE", "Ca", "Ce", "I")
	for _, node := range byInstability(g) {
		_, _ = fmt.Fprintf(w, "%-34s %4d %4d %6.2f\n",
			short(node.ImportPath), node.Afferent, node.Efferent, node.Instability())
	}
	writeMargin(w, g)
	writeViolations(w, g)
}

// writeMargin states how close the tightest edge is to breaking.
func writeMargin(w io.Writer, g Graph) {
	edge, slack := g.Margin()
	if edge.From == "" {
		return
	}
	_, _ = fmt.Fprintf(w, "\ntightest edge: %s -> %s, margin %+.3f over %d edges\n",
		short(edge.From), short(edge.To), slack, len(g.Edges))
}

// writeViolations names every edge running against stability, with both
// endpoints' full coupling - without those numbers a reader cannot tell a
// broken architecture from an unrelated import that shifted a ratio.
func writeViolations(w io.Writer, g Graph) {
	violations := g.Violations()
	if len(violations) == 0 {
		_, _ = fmt.Fprintln(w, "no package depends against the direction of stability")
		return
	}
	for _, v := range violations {
		_, _ = fmt.Fprintf(w,
			"SDP: %s (Ca=%d Ce=%d I=%.2f) depends on %s (Ca=%d Ce=%d I=%.2f)\n",
			short(v.From.ImportPath), v.From.Afferent, v.From.Efferent, v.From.Instability(),
			short(v.To.ImportPath), v.To.Afferent, v.To.Efferent, v.To.Instability())
	}
}

// byInstability orders the table most unstable first, which reads as the
// dependency direction: callers at the top, leaves at the bottom.
func byInstability(g Graph) []Node {
	nodes := make([]Node, 0, len(g.Nodes))
	for _, node := range g.Nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Instability() != nodes[j].Instability() {
			return nodes[i].Instability() > nodes[j].Instability()
		}
		return nodes[i].ImportPath < nodes[j].ImportPath
	})
	return nodes
}

// Short drops the module prefix from each path, leaving the names a person
// uses. Exported so a caller's own output reads like this package's table.
func Short(importPaths ...string) []string {
	names := make([]string, len(importPaths))
	for i, importPath := range importPaths {
		names[i] = short(importPath)
	}
	return names
}

// short drops the module prefix, leaving the name a person uses.
func short(importPath string) string {
	if _, rest, found := strings.Cut(importPath, "/internal/"); found {
		return "internal/" + rest
	}
	return importPath
}
