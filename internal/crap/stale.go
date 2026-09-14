package crap

import (
	"os"
	"path/filepath"
	"time"

	"github.com/WilsonSousajr/omatty/internal/golist"
)

// NewerSource returns the first source file modified after cutoff.
//
//	newer, stale, err := crap.NewerSource(pkgs, profileInfo.ModTime())
//
// Scoring reuses the profile the coverage gate already wrote, which is what
// makes this check nearly free - and is also the one way it can report a
// confident wrong number. Coverage blocks carry line and column positions, so
// an old profile against an edited tree attributes them to whatever now sits at
// those lines, and the report looks exactly as trustworthy as a correct one.
// Refusing to score beats scoring the wrong thing.
func NewerSource(pkgs []golist.Package, cutoff time.Time) (string, bool, error) {
	for _, pkg := range pkgs {
		for _, file := range pkg.GoFiles {
			path := filepath.Join(pkg.Dir, file)
			info, err := os.Stat(path)
			if err != nil {
				return "", false, wrap("checking whether the profile is current", err)
			}
			if info.ModTime().After(cutoff) {
				return path, true, nil
			}
		}
	}
	return "", false, nil
}
