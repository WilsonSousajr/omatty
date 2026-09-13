// Package coverage turns a coverage profile into per-line verdicts, so the
// review column can say which of the lines a session just added are not
// exercised by anything.
//
// M9 reads a coverage *percentage* for the session card. This is the same data
// at the granularity a diff is read at.
//
// Three states, and the third is the one that earns the package its shape:
// a line can be covered, uncovered, or have no verdict at all. Lines in no
// block are declarations, braces and comments - not untested statements - and
// marking them would be noise that trains the eye to ignore the marker. So an
// absent line is silence, never a claim.
package coverage

// File is one file's line verdicts, keyed by 1-based line number. A line that
// is absent is not a statement and has no verdict; the zero value answers
// "unknown" for every line, so a caller needs no nil check.
type File struct{ Lines map[int]bool }

// Profile is per-file coverage, keyed by repo-relative path - the same path a
// diff names, which is the whole reason parsing does any path work at all.
//
//	p, err := coverage.ParseGo(f, "github.com/you/repo")
//	covered, known := p.Files["internal/gate/run.go"].Lines[42]
type Profile struct{ Files map[string]File }

// mark records a verdict, letting covered win. One line can host two
// statements, and one of them running is enough to say the line was reached -
// so the order blocks appear in the profile cannot change the answer.
func (p Profile) mark(path string, line int, covered bool) {
	f, seen := p.Files[path]
	if !seen {
		f = File{Lines: map[int]bool{}}
		p.Files[path] = f
	}
	if was, known := f.Lines[line]; known && was {
		return
	}
	f.Lines[line] = covered
}
