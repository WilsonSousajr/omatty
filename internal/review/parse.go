package review

import (
	"fmt"
	"io"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// ParseDiff parses git's unified output. go-gitdiff is confined to this file,
// in the spirit of invariant 4: the rest of omatty sees only review's own
// types.
//
//	d, err := review.ParseDiff(strings.NewReader(raw))
func ParseDiff(r io.Reader) (Diff, error) {
	files, _, err := gitdiff.Parse(r)
	if err != nil {
		return Diff{}, fmt.Errorf("review: parsing unified diff: %w", err)
	}
	d := Diff{Files: make([]File, 0, len(files))}
	for _, f := range files {
		d.Files = append(d.Files, convertFile(f))
	}
	return d, nil
}

func convertFile(f *gitdiff.File) File {
	out := File{Path: f.NewName, OldPath: f.OldName, Status: statusOf(f), Binary: f.IsBinary}
	if f.IsDelete {
		out.Path = f.OldName
	}
	for _, frag := range f.TextFragments {
		out.Hunks = append(out.Hunks, convertHunk(frag))
	}
	return out
}

func statusOf(f *gitdiff.File) FileStatus {
	switch {
	case f.IsNew:
		return FileAdded
	case f.IsDelete:
		return FileDeleted
	case f.IsRename:
		return FileRenamed
	default:
		return FileModified
	}
}

// convertHunk numbers every line on both sides while walking the fragment;
// that is how a comment can later say file:line for the version Claude sees.
func convertHunk(frag *gitdiff.TextFragment) Hunk {
	h := Hunk{Header: strings.TrimSpace(frag.Header()), Lines: make([]Line, 0, len(frag.Lines))}
	oldNo, newNo := int(frag.OldPosition), int(frag.NewPosition)
	for _, l := range frag.Lines {
		line := Line{Kind: kindOf(l.Op), Text: strings.TrimSuffix(l.Line, "\n")}
		if l.Op != gitdiff.OpAdd {
			line.OldNo, oldNo = oldNo, oldNo+1
		}
		if l.Op != gitdiff.OpDelete {
			line.NewNo, newNo = newNo, newNo+1
		}
		h.Lines = append(h.Lines, line)
	}
	return h
}

func kindOf(op gitdiff.LineOp) LineKind {
	switch op {
	case gitdiff.OpAdd:
		return LineAdded
	case gitdiff.OpDelete:
		return LineRemoved
	default:
		return LineContext
	}
}
