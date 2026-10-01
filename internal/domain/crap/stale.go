package crap

import (
	"path/filepath"
	"time"
)

// NewerSource returns the first source file modified after cutoff.
//
//	newer, stale, err := crap.NewerSource(pkgs, profileInfo.ModTime(), files)
//
// Scoring reuses the profile the coverage gate already wrote, which is what
// makes this check nearly free - and is also the one way it can report a
// confident wrong number. Coverage blocks carry line and column positions, so
// an old profile against an edited tree attributes them to whatever now sits at
// those lines, and the report looks exactly as trustworthy as a correct one.
// Refusing to score beats scoring the wrong thing.
func NewerSource(pkgs []Package, cutoff time.Time, files Files) (string, bool, error) {
	for _, pkg := range pkgs {
		// Test files as well as source: a test added after the profile changes
		// coverage without touching a line of source, which is exactly the
		// stale profile this guard exists to refuse (#385).
		for _, group := range [][]string{pkg.GoFiles, pkg.TestGoFiles, pkg.XTestGoFiles} {
			name, found, err := newerIn(pkg.Dir, group, cutoff, files)
			if err != nil || found {
				return name, found, err
			}
		}
	}
	return "", false, nil
}

// newerIn reports the first of dir's files modified after cutoff.
func newerIn(dir string, names []string, cutoff time.Time, files Files) (string, bool, error) {
	for _, file := range names {
		path := filepath.Join(dir, file)
		modTime, err := files.ModTime(path)
		if err != nil {
			return "", false, wrap("checking whether the profile is current", err)
		}
		if modTime.After(cutoff) {
			return path, true, nil
		}
	}
	return "", false, nil
}
