package review

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Anchor names a diff line by its content, not its number (invariant 7): the
// file path, the hunk header, a hash of the line, and which occurrence of that
// hash within the hunk it is, so five closing braces in one hunk are five
// different lines.
//
//	a := review.AnchorAt(d, review.Position{File: 0, Hunk: 0, Line: 2})
type Anchor struct {
	File string
	Hunk string
	Hash string
	Nth  int
}

// LineHash is a short content hash of a line: its kind and its text, so a line
// that flips from removed to added is a different line. Line numbers are
// deliberately absent - Claude edits files while you read them.
//
//	h := review.LineHash(review.Line{Kind: review.LineAdded, Text: "\tb := 3"})
func LineHash(l Line) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%d:%s", l.Kind, l.Text))
	return hex.EncodeToString(sum[:6])
}

// AnchorAt builds the anchor for the line at p in d.
func AnchorAt(d Diff, p Position) Anchor {
	f := d.Files[p.File]
	h := f.Hunks[p.Hunk]
	hash := LineHash(h.Lines[p.Line])
	return Anchor{File: f.Path, Hunk: h.Header, Hash: hash, Nth: occurrence(h, hash, p.Line)}
}

// occurrence counts the lines before upto in h that share hash.
func occurrence(h Hunk, hash string, upto int) int {
	n := 0
	for _, l := range h.Lines[:upto] {
		if LineHash(l) == hash {
			n++
		}
	}
	return n
}
