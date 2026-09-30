package main

import (
	"os"
	"time"

	"github.com/WilsonSousajr/omatty/internal/domain/crap"
	"github.com/WilsonSousajr/omatty/internal/infra/golist"
)

// diskFiles is the real filesystem, which crap reads through rather than
// touching itself: crap is domain, and this tool is where the I/O belongs
// (ADR 0001, migration step 3.9).
func diskFiles() crap.Files {
	return crap.Files{Read: os.ReadFile, ModTime: modTime}
}

func modTime(path string) (time.Time, error) {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}, err
	}
	return info.ModTime(), nil
}

// forCrap is go list's answer in the shape crap reads.
func forCrap(pkgs []golist.Package) []crap.Package {
	out := make([]crap.Package, len(pkgs))
	for i, p := range pkgs {
		out[i] = crap.Package{ImportPath: p.ImportPath, Dir: p.Dir, GoFiles: p.GoFiles,
			TestGoFiles: p.TestGoFiles, XTestGoFiles: p.XTestGoFiles}
	}
	return out
}
