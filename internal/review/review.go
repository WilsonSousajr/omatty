// Package review turns a session's diff into hunks, anchors comments on their
// content rather than their line numbers (invariant 7), and composes the one
// message that sends them back to Claude (M3).
package review

// LineKind is what a diff line did.
type LineKind int

// The three kinds of unified-diff line.
const (
	LineContext LineKind = iota
	LineAdded
	LineRemoved
)

// Line is one line of a hunk. OldNo and NewNo are 1-based numbers in the old
// and new file; 0 means the line does not exist on that side.
type Line struct {
	Kind  LineKind
	Text  string
	OldNo int
	NewNo int
}

// Hunk is one @@ block. Header is git's own "@@ -a,b +c,d @@ context" line,
// the middle element of a comment anchor (invariant 7).
type Hunk struct {
	Header string
	Lines  []Line
}

// FileStatus is how a file changed.
type FileStatus int

// File statuses, as git reports them.
const (
	FileModified FileStatus = iota
	FileAdded
	FileDeleted
	FileRenamed
)

// File is one changed file. Path is the new name, or the old name of a deleted
// file, so a comment always names a path the operator can open.
type File struct {
	Path    string
	OldPath string
	Status  FileStatus
	Binary  bool
	Hunks   []Hunk
}

// Counts returns the lines added and removed in f, for the file header.
//
//	added, removed := f.Counts() // 2, 1
func (f File) Counts() (added, removed int) {
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			added, removed = count(l.Kind, added, removed)
		}
	}
	return added, removed
}

func count(k LineKind, added, removed int) (int, int) {
	switch k {
	case LineAdded:
		return added + 1, removed
	case LineRemoved:
		return added, removed + 1
	}
	return added, removed
}

// Diff is everything a session changed.
//
//	d, err := review.ParseDiff(strings.NewReader(raw))
type Diff struct {
	Files []File
}

// Position indexes a line: Files[File].Hunks[Hunk].Lines[Line].
type Position struct{ File, Hunk, Line int }

// LineAt returns the line at p. p must come from this diff's own entries.
//
//	l := d.LineAt(review.Position{File: 0, Hunk: 0, Line: 2})
func (d Diff) LineAt(p Position) Line {
	return d.Files[p.File].Hunks[p.Hunk].Lines[p.Line]
}
