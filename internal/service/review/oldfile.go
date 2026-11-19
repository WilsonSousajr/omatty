package review

import (
	"fmt"

	dreview "github.com/WilsonSousajr/omatty/internal/domain/review"
	"github.com/WilsonSousajr/omatty/internal/domain/session"
)

// OldFile is path as it was at rev in sess's checkout: the old side of a diff,
// read whole so its highlighter can start at the file's first line rather
// than the hunk's (#800). Bounded as a preview is; a file over the bound comes
// back Truncated with no lines, and its hunks are lexed alone.
//
//	p, err := src.OldFile(sess, d.Base, "internal/ui/model.go")
func (s *Source) OldFile(sess session.Session, rev, path string) (dreview.Preview, error) {
	buf, whole, err := s.git.Blob(sess.Dir, rev, path, dreview.PreviewLimit)
	if err != nil {
		return dreview.Preview{}, fmt.Errorf("review: reading %s at %s for session %s: %w", path, rev, sess.ID, err)
	}
	if !whole {
		return dreview.Preview{Path: path, Truncated: true}, nil
	}
	return dreview.PreviewOf(path, buf), nil
}
