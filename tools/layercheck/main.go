// Command layercheck reports ADR 0001's layer rule against omatty's own
// import graph (#620).
//
//	go run ./tools/layercheck            # report; always exits 0
//	go run ./tools/layercheck -enforce   # exit 1 on any finding
//
// Report-only until the migration's last PR adds -enforce to CI. Until then the
// count is the migration's backlog: each PR that moves a package should take
// it down, and one that raises it has broken the direction ADR 0001 set. The
// same measure-then-enforce order took the Stable Dependencies check from
// report to gate (#263, #269).
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/WilsonSousajr/omatty/internal/infra/golist"
)

func main() {
	enforce := flag.Bool("enforce", false, "exit 1 when any finding is reported")
	flag.Parse()

	findings, err := run(os.Stdout, *enforce)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if *enforce && findings > 0 {
		os.Exit(1)
	}
}

// run lists the module, prints every finding and the count, and returns it.
func run(out io.Writer, enforce bool) (int, error) {
	module, err := golist.Module(".")
	if err != nil {
		return 0, err
	}
	pkgs, err := golist.List(".", "./...")
	if err != nil {
		return 0, err
	}
	findings := check(module, pkgs)
	report(out, findings, enforce)
	return len(findings), nil
}

// report prints the findings sorted, then the count and the mode, so the last
// line says both how much is left and whether it failed the run.
func report(out io.Writer, findings []Finding, enforce bool) {
	lines := make([]string, len(findings))
	for i, f := range findings {
		lines[i] = f.String()
	}
	sort.Strings(lines)
	for _, l := range lines {
		_, _ = fmt.Fprintln(out, l)
	}
	mode := "report only"
	if enforce {
		mode = "enforced"
	}
	_, _ = fmt.Fprintf(out, "layer findings: %d (%s)\n", len(findings), mode)
}
