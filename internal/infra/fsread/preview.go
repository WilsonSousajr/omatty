package fsread

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/WilsonSousajr/omatty/internal/domain/review"
)

// ReadPreview loads rel under dir for display. The read moved here from
// internal/review in migration step 5.8 (#653); what the bytes are - text,
// binary, truncated - is review.PreviewOf's to say. Paths come from git, so they
// are relative; anything absolute or climbing out of dir is refused anyway,
// because the tree must never show a file the worktree does not contain.
//
//	p, err := fsread.ReadPreview(sess.Dir, "internal/ui/model.go")
func ReadPreview(dir, rel string) (review.Preview, error) {
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return review.Preview{}, fmt.Errorf("review: preview path %q leaves the worktree %q", rel, dir)
	}
	f, err := os.Open(filepath.Join(dir, clean))
	if err != nil {
		return review.Preview{}, fmt.Errorf("review: opening %q for preview: %w", rel, err)
	}
	defer func() { _ = f.Close() }()
	buf, err := io.ReadAll(io.LimitReader(f, review.PreviewLimit+1))
	if err != nil {
		return review.Preview{}, fmt.Errorf("review: reading %q for preview: %w", rel, err)
	}
	return review.PreviewOf(clean, buf), nil
}
