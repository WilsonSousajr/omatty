package crap

import (
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/coverage"
	"github.com/WilsonSousajr/omatty/internal/golist"
)

// Scores reads every function in pkgs and joins it to a coverage profile,
// worst score first.
//
//	scores, err := crap.Scores("github.com/you/repo", pkgs, blocks)
//
// modulePath is needed because a profile keys its records by *import path*
// while a report names a file, which is the same translation
// coverage.ParseGoBlocks does at the other end.
//
// A function whose file contributes no blocks is scored at zero coverage rather
// than skipped. "Absent from the profile" and "nothing ran" are the same fact
// here, and the second is the one worth printing: a scorer that quietly drops
// what it cannot find reports a clean tree.
func Scores(modulePath string, pkgs []golist.Package, blocks []coverage.Block) ([]Score, error) {
	byFile := blocksByFile(blocks)
	var scores []Score
	for _, pkg := range pkgs {
		for _, file := range pkg.GoFiles {
			relative := path.Join(relativeDir(pkg.ImportPath, modulePath), file)
			blocks, profiled := byFile[relative]
			found, err := fileScores(pkg, file, relative, blocks, profiled)
			if err != nil {
				return nil, err
			}
			scores = append(scores, found...)
		}
	}
	sortScores(scores)
	return scores, nil
}

// fileScores scores every function in one file.
//
// profiled says whether the profile mentioned this file at all. When it did
// not, the statement count comes from the source instead, so that a function
// nothing tested scores zero coverage rather than passing as one with nothing
// in it to test.
func fileScores(pkg golist.Package, file, relative string, blocks []coverage.Block, profiled bool) ([]Score, error) {
	extents, err := extentsIn(filepath.Join(pkg.Dir, file))
	if err != nil {
		return nil, err
	}
	scores := make([]Score, 0, len(extents))
	for _, e := range extents {
		statements, covered := e.tally(blocks)
		if !profiled {
			statements, covered = e.statements, 0
		}
		scores = append(scores, Score{
			Package: pkg.ImportPath, File: relative, Line: e.startLine,
			Name: e.name, Complexity: e.complexity,
			Statements: statements, Covered: covered,
		})
	}
	return scores, nil
}

// tally sums the statements of the blocks this function holds.
func (e extent) tally(blocks []coverage.Block) (statements, covered int) {
	for _, b := range blocks {
		if !e.holds(b) {
			continue
		}
		statements += b.NumStmt
		if b.Covered {
			covered += b.NumStmt
		}
	}
	return statements, covered
}

// blocksByFile groups a profile by the path a report names.
func blocksByFile(blocks []coverage.Block) map[string][]coverage.Block {
	byFile := map[string][]coverage.Block{}
	for _, b := range blocks {
		byFile[b.Path] = append(byFile[b.Path], b)
	}
	return byFile
}

// relativeDir turns an import path into the directory a profile record names.
// The main module's own root package answers "", which path.Join drops.
func relativeDir(importPath, modulePath string) string {
	prefix := strings.TrimSuffix(modulePath, "/") + "/"
	if relative, found := strings.CutPrefix(importPath, prefix); found {
		return relative
	}
	return ""
}

// sortScores puts the worst first, then breaks ties by position so that two
// runs over an unchanged tree print the same report.
func sortScores(scores []Score) {
	sort.Slice(scores, func(i, j int) bool {
		a, b := scores[i], scores[j]
		if a.Value() != b.Value() {
			return a.Value() > b.Value()
		}
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
}
