package fsread

import (
	"io"
	"os"
	"path/filepath"
)

// Head reads at most limit bytes from the start of rel under dir: the
// generated-file sniff's read (#338), moved here from internal/review in
// migration step 5.8 (#653) because opening a file is infra's business.
//
//	head, err := fsread.Head(sess.Dir, "zz_generated.go", 4<<10)
func Head(dir, rel string, limit int64) ([]byte, error) {
	f, err := os.Open(filepath.Join(dir, rel)) //nolint:gosec // a path git listed in the session's own worktree
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(io.LimitReader(f, limit))
}
