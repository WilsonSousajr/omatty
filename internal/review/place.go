package review

// Placed is where each queued comment sits against the current diff. Indexes
// are into the comment slice given to Place.
type Placed struct {
	// At lists the comments on each resolved line, in queue order.
	At map[Position][]int
	// Where is the resolved line of each placed comment.
	Where map[int]Position
	// Orphans lists, per file index, the comments whose line no longer exists
	// there; they float to the top of the file marked moved (#22).
	Orphans map[int][]int
	// Lost holds comments whose file is no longer in the diff at all.
	Lost []int
}

// Place resolves every comment against d: the exact anchor first, then the
// same content anywhere in the same file when the hunk header shifted, else an
// orphan of the file (#22).
//
//	p := review.Place(d, cs.All())
func Place(d Diff, comments []Comment) Placed {
	p := Placed{At: map[Position][]int{}, Where: map[int]Position{}, Orphans: map[int][]int{}}
	for i, c := range comments {
		p.put(d, i, c.Anchor)
	}
	return p
}

func (p *Placed) put(d Diff, i int, a Anchor) {
	fi, ok := fileIndex(d, a.File)
	if !ok {
		p.Lost = append(p.Lost, i)
		return
	}
	pos, ok := resolve(d.Files[fi], fi, a)
	if !ok {
		p.Orphans[fi] = append(p.Orphans[fi], i)
		return
	}
	p.At[pos] = append(p.At[pos], i)
	p.Where[i] = pos
}

func fileIndex(d Diff, path string) (int, bool) {
	for i, f := range d.Files {
		if f.Path == path {
			return i, true
		}
	}
	return 0, false
}

// resolve finds a in f: its own hunk and occurrence, else the first line in
// any hunk with the same content. The second pass exists because the hunk
// header carries line numbers, so any edit above a hunk rewrites it while the
// commented line itself is untouched.
func resolve(f File, fi int, a Anchor) (Position, bool) {
	for hi, h := range f.Hunks {
		if h.Header != a.Hunk {
			continue
		}
		if li, ok := nthLine(h, a.Hash, a.Nth); ok {
			return Position{fi, hi, li}, true
		}
	}
	for hi, h := range f.Hunks {
		if li, ok := nthLine(h, a.Hash, 0); ok {
			return Position{fi, hi, li}, true
		}
	}
	return Position{}, false
}

// nthLine returns the index of the nth line in h whose content hashes to hash.
func nthLine(h Hunk, hash string, nth int) (int, bool) {
	seen := 0
	for i, l := range h.Lines {
		if LineHash(l) != hash {
			continue
		}
		if seen == nth {
			return i, true
		}
		seen++
	}
	return 0, false
}
