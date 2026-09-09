package review

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// previewLimit bounds what a preview reads; a generated file must not stall
// the frame.
const previewLimit = 256 << 10

// Preview is a file's text for the preview view (#24). Deleted marks a row
// the diff removed: there is no file to read, so the view explains instead
// of reporting a read error (#196).
type Preview struct {
	Path      string
	Lines     []string
	Binary    bool
	Truncated bool
	Deleted   bool
	// Styled is Lines with syntax colouring, one for one, or nil when the
	// file was not highlighted; Lines stays plain for measuring width. Set by
	// the ui after the read, so this package needs no highlighter (#197).
	Styled []string
	// Unhighlighted says the file was too large to highlight, which the
	// preview notes under its last line; an unknown file type is silent.
	Unhighlighted bool
}

// ReadPreview loads rel under dir for display. Paths come from git, so they
// are relative; anything absolute or climbing out of dir is refused anyway,
// because the tree must never show a file the worktree does not contain.
//
//	p, err := review.ReadPreview(sess.Dir, "internal/ui/model.go")
func ReadPreview(dir, rel string) (Preview, error) {
	clean := filepath.Clean(rel)
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return Preview{}, fmt.Errorf("review: preview path %q leaves the worktree %q", rel, dir)
	}
	f, err := os.Open(filepath.Join(dir, clean))
	if err != nil {
		return Preview{}, fmt.Errorf("review: opening %q for preview: %w", rel, err)
	}
	defer func() { _ = f.Close() }()
	buf, err := io.ReadAll(io.LimitReader(f, previewLimit+1))
	if err != nil {
		return Preview{}, fmt.Errorf("review: reading %q for preview: %w", rel, err)
	}
	return previewOf(clean, buf), nil
}

// previewOf classifies the bytes: a NUL means binary, more than the limit
// means truncated back to the last whole line, so the view never shows half a
// rune or half a statement.
func previewOf(path string, buf []byte) Preview {
	p := Preview{Path: path}
	if bytes.IndexByte(buf, 0) >= 0 {
		p.Binary = true
		return p
	}
	if len(buf) > previewLimit {
		p.Truncated = true
		buf = buf[:bytes.LastIndexByte(buf[:previewLimit], '\n')+1]
	}
	p.Lines = strings.Split(strings.TrimSuffix(string(buf), "\n"), "\n")
	return p
}
