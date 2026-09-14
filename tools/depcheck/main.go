// Command depcheck reports omatty's own package dependency structure and
// enforces the rules that cannot oscillate.
//
//	go run ./tools/depcheck            # report, and fail on a test-graph cycle
//	go run ./tools/depcheck -sdp       # also fail on a stable->unstable edge
//
// Two rules, enforced differently on purpose. An import cycle through the test
// graph is a yes-or-no fact, so it fails the run today. The Stable Dependencies
// Principle is a comparison of ratios over small integers, so it lands
// report-only: the numbers are green now, and one or two ordinary PRs are worth
// watching before a moving margin can fail a build (#263).
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
	sdp := flag.Bool("sdp", false, "fail when an import runs against the direction of stability")
	pattern := flag.String("pattern", "./internal/...", "packages to measure")
	flag.Parse()

	failures, err := run(os.Stdout, *sdp, *pattern)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if failures > 0 {
		os.Exit(1)
	}
}

// run measures the graph and returns how many enforced rules it broke.
func run(out *os.File, enforceSDP bool, pattern string) (int, error) {
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
	if enforceSDP {
		failures += len(graph.Violations())
	}
	return failures, nil
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
