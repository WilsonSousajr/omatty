// Command crapcheck scores every function in ./internal/... by the C.R.A.P.
// metric and fails when one reaches the threshold.
//
//	go run ./tools/crapcheck -threshold 15 -profile cover.out
//
// It lives in tools/ rather than internal/ because it is a main: inside ./...
// so gofmt, vet, lint and build all cover it, outside ./internal/... so that an
// untestable entry point does not pull the 90% coverage gate down. Everything
// worth testing is in internal/crap; this is wiring.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/WilsonSousajr/omatty/internal/coverage"
	"github.com/WilsonSousajr/omatty/internal/crap"
	"github.com/WilsonSousajr/omatty/internal/golist"
)

func main() {
	threshold := flag.Float64("threshold", 15, "fail when a function's CRAP score reaches this")
	profile := flag.String("profile", "cover.out", "coverage profile to read")
	pattern := flag.String("pattern", "./internal/...", "packages to score")
	flag.Parse()

	over, err := run(os.Stdout, *threshold, *profile, *pattern)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if over > 0 {
		os.Exit(1)
	}
}

// run scores the tree and reports, returning how many functions crossed the
// threshold. Exit 2 for a broken run and exit 1 for a failing gate are kept
// apart so that "the scorer could not read the profile" never looks like "the
// code is bad".
func run(out io.Writer, threshold float64, profilePath, pattern string) (int, error) {
	pkgs, err := golist.List(".", pattern)
	if err != nil {
		return 0, err
	}
	blocks, err := readProfile(profilePath, pkgs)
	if err != nil {
		return 0, err
	}
	module, err := golist.Module(".")
	if err != nil {
		return 0, err
	}
	scores, err := crap.Scores(module, pkgs, blocks)
	if err != nil {
		return 0, err
	}
	reportSkipped(out, pkgs)
	return crap.Report(out, scores, threshold), nil
}

// readProfile opens the coverage profile, refusing one older than the source it
// would be scored against.
func readProfile(path string, pkgs []golist.Package) ([]coverage.Block, error) {
	if err := refuseStale(path, pkgs); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("crapcheck: %w", err)
	}
	defer func() { _ = f.Close() }()

	module, err := golist.Module(".")
	if err != nil {
		return nil, err
	}
	return coverage.ParseGoBlocks(f, module)
}

// refuseStale rejects a profile older than the source it would be scored
// against, rather than reporting the confident wrong number that scoring it
// would produce.
func refuseStale(path string, pkgs []golist.Package) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("crapcheck: %w\nrun ./scripts/check-coverage.sh first", err)
	}
	newer, stale, err := crap.NewerSource(pkgs, info.ModTime())
	if err != nil {
		return err
	}
	if stale {
		return fmt.Errorf("crapcheck: %s is older than %s\n"+
			"a stale profile attributes blocks to whatever now sits at those "+
			"lines; re-run ./scripts/check-coverage.sh", path, newer)
	}
	return nil
}

// reportSkipped says how many files a build constraint kept out of the score,
// so their absence is never silent.
func reportSkipped(out io.Writer, pkgs []golist.Package) {
	skipped := 0
	for _, pkg := range pkgs {
		skipped += len(pkg.IgnoredGoFiles)
	}
	if skipped > 0 {
		_, _ = fmt.Fprintf(out, "%d file(s) excluded by build constraints on this platform\n", skipped)
	}
}
