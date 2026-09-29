package review

// Two diffs of one session (#311): the whole session, and one turn of it.
// An anchor written against one does not place in the other by its hunk
// header - the turn's old side counts from its baseline, not the base - nor
// by its hash, which includes the line's kind, and a line the session added
// is context in a later turn. The new-side line number and the text do carry
// across, because both diffs end at the same working tree. These map between
// the two on that, so a comment is never moved onto another line that merely
// reads the same, which is what invariant 7 exists to prevent.

// AnchorFor anchors a comment written on the turn's line p against the
// session diff, the one Compose and PruneSent read, so the file:line Claude
// is told is the session's. A line the session diff does not hold keeps the
// turn's own anchor.
//
//	a := review.AnchorFor(sessionDiff, turnDiff, pos)
func AnchorFor(session, turn Diff, p Position) Anchor {
	if sp, ok := sameLine(turn, p, session); ok {
		return AnchorAt(session, sp)
	}
	return AnchorAt(turn, p)
}

// PlaceIn places comments for the turn view. A comment anchored on the turn
// itself places exactly; any other resolves against the session diff and then
// moves to the turn's line with the same path, new-side number and text. What
// places on neither is outside this turn, and is left out rather than
// orphaned: it has not moved, it is elsewhere in the session.
//
//	p := review.PlaceIn(sessionDiff, turnDiff, cs.All())
func PlaceIn(session, turn Diff, comments []Comment) Placed {
	p := Placed{At: map[Position][]int{}, Where: map[int]Position{}, Orphans: map[int][]int{}}
	for i, c := range comments {
		if pos, ok := placeInTurn(session, turn, c.Anchor); ok {
			p.At[pos] = append(p.At[pos], i)
			p.Where[i] = pos
		}
	}
	return p
}

func placeInTurn(session, turn Diff, a Anchor) (Position, bool) {
	if pos, ok := exactIn(turn, a); ok {
		return pos, true
	}
	fi, ok := fileIndex(session, a.File)
	if !ok {
		return Position{}, false
	}
	sp, ok := resolve(session.Files[fi], fi, a)
	if !ok {
		return Position{}, false
	}
	return sameLine(session, sp, turn)
}

// exactIn is resolve's first pass alone: the anchor's own hunk header and
// occurrence, with no fallback to the first line that reads the same.
func exactIn(d Diff, a Anchor) (Position, bool) {
	fi, ok := fileIndex(d, a.File)
	if !ok {
		return Position{}, false
	}
	for hi, h := range d.Files[fi].Hunks {
		if h.Header != a.Hunk {
			continue
		}
		if li, ok := nthLine(h, a.Hash, a.Nth); ok {
			return Position{fi, hi, li}, true
		}
	}
	return Position{}, false
}

// sameLine finds in to the line at p in from: the same path, new-side number
// and text. A removed line has no new-side number, so it never matches.
func sameLine(from Diff, p Position, to Diff) (Position, bool) {
	l := from.LineAt(p)
	if l.NewNo == 0 {
		return Position{}, false
	}
	fi, ok := fileIndex(to, from.Files[p.File].Path)
	if !ok {
		return Position{}, false
	}
	for hi, h := range to.Files[fi].Hunks {
		for li, tl := range h.Lines {
			if tl.NewNo == l.NewNo && tl.Text == l.Text && tl.Kind != LineRemoved {
				return Position{fi, hi, li}, true
			}
		}
	}
	return Position{}, false
}
