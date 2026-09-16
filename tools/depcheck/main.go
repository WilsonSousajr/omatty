// Command depcheck reports omatty's own package dependency structure and
// enforces the two rules that hold across it.
//
//	go run ./tools/depcheck            # report, and fail on either rule
//
// An import cycle through the test graph is a yes-or-no fact and has failed the
// run since #263. The Stable Dependencies Principle is a comparison of ratios
// over small integers, so it landed report-only behind -sdp, deliberately: a
// gate that fails on a margin nobody has watched move is a gate people learn to
// --no-verify past.
//
// The margin has now been watched. Across every merge from #263 to #278 - eight
// pull requests, and one of them adding a package edge that took the graph from
// 35 to 36 - the tightest edge stayed watcher -> registry at exactly +0.071. So
// the flag is gone and the rule is enforced like the other one (#269).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/depgraph"
	"github.com/WilsonSousajr/omatty/internal/golist"
)

func main() {
	pattern := flag.String("pattern", "./internal/...", "packages to measure")
	flag.Parse()

	failures, err := run(os.Stdout, *pattern)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if failures > 0 {
		os.Exit(1)
	}
}

// run measures the graph and returns how many enforced rules it broke.
func run(out *os.File, pattern string) (int, error) {
	module, err := golist.Module(".")
	if err != nil {
		return 0, err
	}
	pkgs, err := golist.List(".", pattern)
	if err != nil {
		return 0, err
	}
	graph := depgraph.Build(module, pkgs)
	depgraph.Report(out, graph)

	failures := reportCycles(out, depgraph.TestCycles(module, pkgs))
	return failures + len(graph.Violations()), nil
}

// reportCycles names any cycle running through a package's tests. The compiler
// refuses one in production imports, so this is the only kind a Go repository
// can actually have - and nothing looked for it before.
func reportCycles(out *os.File, cycles [][]string) int {
	for _, cycle := range cycles {
		_, _ = fmt.Fprintf(out, "cycle through the test graph: %s\n",
			strings.Join(depgraph.Short(cycle...), " -> "))
	}
	return len(cycles)
}
