// Package gitdiff is omatty's only route to go-gitdiff: it parses git's
// unified output into internal/domain/review's review.Diff, so nothing else in omatty
// ever sees go-gitdiff's types (ADR 0001, migration step 3.6b; invariant 4 in
// spirit).
//
//	d, err := gd.ParseDiff(strings.NewReader(raw))
package gitdiff

import (
	"fmt"
	"io"
	"strings"

	gd "github.com/bluekeyes/go-gitdiff/gitdiff"

	"github.com/WilsonSousajr/omatty/internal/domain/review"
)

// ParseDiff parses git's unified output. go-gitdiff is confined to this file,
// in the spirit of invariant 4: the rest of omatty sees only review's own
// types.
//
//	d, err := gitdiff.ParseDiff(strings.NewReader(raw))
func ParseDiff(r io.Reader) (review.Diff, error) {
	files, _, err := gd.Parse(r)
	if err != nil {
		return review.Diff{}, fmt.Errorf("review: parsing unified diff: %w", err)
	}
	d := review.Diff{Files: make([]review.File, 0, len(files))}
	for _, f := range files {
		d.Files = append(d.Files, convertFile(f))
	}
	return d, nil
}

func convertFile(f *gd.File) review.File {
	out := review.File{Path: f.NewName, OldPath: f.OldName, Status: statusOf(f), Binary: f.IsBinary}
	if f.IsDelete {
		out.Path = f.OldName
	}
	for _, frag := range f.TextFragments {
		out.Hunks = append(out.Hunks, convertHunk(frag))
	}
	return out
}

func statusOf(f *gd.File) review.FileStatus {
	switch {
	case f.IsNew:
		return review.FileAdded
	case f.IsDelete:
		return review.FileDeleted
	case f.IsRename:
		return review.FileRenamed
	default:
		return review.FileModified
	}
}

// convertHunk numbers every line on both sides while walking the fragment;
// that is how a comment can later say file:line for the version Claude sees.
func convertHunk(frag *gd.TextFragment) review.Hunk {
	h := review.Hunk{Header: strings.TrimSpace(frag.Header()), Lines: make([]review.Line, 0, len(frag.Lines))}
	oldNo, newNo := int(frag.OldPosition), int(frag.NewPosition)
	for _, l := range frag.Lines {
		line := review.Line{Kind: kindOf(l.Op), Text: strings.TrimSuffix(l.Line, "\n")}
		if l.Op != gd.OpAdd {
			line.OldNo, oldNo = oldNo, oldNo+1
		}
		if l.Op != gd.OpDelete {
			line.NewNo, newNo = newNo, newNo+1
		}
		h.Lines = append(h.Lines, line)
	}
	return h
}

func kindOf(op gd.LineOp) review.LineKind {
	switch op {
	case gd.OpAdd:
		return review.LineAdded
	case gd.OpDelete:
		return review.LineRemoved
	default:
		return review.LineContext
	}
}
