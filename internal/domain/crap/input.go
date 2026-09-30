package crap

import "time"

// Package is one package as crap scores it: where its files are and which
// they are. tools/crapcheck fills it from go list (ADR 0001, migration step
// 3.9), so crap itself never runs anything.
//
//	pkgs := []crap.Package{{ImportPath: "m/f", Dir: "/src/f", GoFiles: []string{"f.go"}}}
type Package struct {
	ImportPath   string
	Dir          string
	GoFiles      []string
	TestGoFiles  []string
	XTestGoFiles []string
}

// Files is how crap reads the tree it scores. crap is pure (ADR 0001, step
// 3.9): the tool that runs it passes the real filesystem, and a test passes
// memory.
//
//	scores, err := crap.Scores(module, pkgs, blocks, crap.Files{Read: os.ReadFile, ModTime: modTime})
type Files struct {
	Read    func(path string) ([]byte, error)
	ModTime func(path string) (time.Time, error)
}
