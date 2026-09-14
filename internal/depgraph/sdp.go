package depgraph

import "sort"

// Violations returns every edge that breaks the Stable Dependencies Principle -
// an import running from a more stable package to a less stable one.
//
// Sorted by how badly, so the worst reads first and two runs over an unchanged
// tree print the same thing.
func (g Graph) Violations() []Violation {
	var found []Violation
	for _, e := range g.Edges {
		from, to := g.Nodes[e.From], g.Nodes[e.To]
		if from.Instability() < to.Instability() {
			found = append(found, Violation{From: from, To: to})
		}
	}
	sort.Slice(found, func(i, j int) bool {
		return margin(found[i]) < margin(found[j])
	})
	return found
}

// Margin returns the tightest edge in the graph and its slack.
//
// It is reported on every run, clean or not, because instability is a ratio of
// small integers and so moves in jumps: internal/config is Ca=1, Ce=1, and one
// new importer takes it from 0.50 to 0.33. A gate that only ever says "clean"
// gives no warning that the next import will break it, and the reader would
// have no way to tell a broken architecture from an unrelated import.
func (g Graph) Margin() (Edge, float64) {
	tightest, slack := Edge{}, 0.0
	for i, e := range g.Edges {
		gap := g.Nodes[e.From].Instability() - g.Nodes[e.To].Instability()
		if i == 0 || gap < slack {
			tightest, slack = e, gap
		}
	}
	return tightest, slack
}

// margin is one violation's shortfall, as a positive number.
func margin(v Violation) float64 {
	return v.From.Instability() - v.To.Instability()
}
