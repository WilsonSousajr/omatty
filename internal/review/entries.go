package review

// EntryKind is what a row of the review pane shows.
type EntryKind int

// Row kinds, in the order they can appear under a file.
const (
	EntryFile EntryKind = iota
	EntryOrphan
	EntryHunk
	EntryLine
	EntryComment
)

// Entry is one row of the flattened diff: the unit the cursor moves over. Pos
// is the line for EntryLine and EntryComment, and the file for EntryFile and
// EntryOrphan. Comment indexes the comment slice for EntryComment and
// EntryOrphan.
type Entry struct {
	Kind    EntryKind
	Pos     Position
	Text    string
	Comment int
}

// Flatten lays d out as rows: each file header, its orphans, then each hunk
// header and its lines with their comments directly beneath them.
//
//	rows := review.Flatten(d, review.Place(d, cs.All()))
func Flatten(d Diff, p Placed) []Entry {
	var out []Entry
	for fi, f := range d.Files {
		out = append(out, Entry{Kind: EntryFile, Pos: Position{File: fi}, Text: f.Path})
		for _, ci := range p.Orphans[fi] {
			out = append(out, Entry{Kind: EntryOrphan, Pos: Position{File: fi}, Comment: ci})
		}
		out = append(out, flattenHunks(fi, f, p)...)
	}
	return out
}

func flattenHunks(fi int, f File, p Placed) []Entry {
	var out []Entry
	for hi, h := range f.Hunks {
		out = append(out, Entry{Kind: EntryHunk, Pos: Position{File: fi, Hunk: hi}, Text: h.Header})
		for li, l := range h.Lines {
			pos := Position{File: fi, Hunk: hi, Line: li}
			out = append(out, Entry{Kind: EntryLine, Pos: pos, Text: l.Text})
			out = append(out, commentEntries(pos, p.At[pos])...)
		}
	}
	return out
}

func commentEntries(pos Position, idx []int) []Entry {
	out := make([]Entry, 0, len(idx))
	for _, ci := range idx {
		out = append(out, Entry{Kind: EntryComment, Pos: pos, Comment: ci})
	}
	return out
}
