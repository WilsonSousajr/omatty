package vcs

import (
	"fmt"
	"strconv"
	"strings"
)

// Blob is path as it was at rev - a commit, or a turn baseline's tree - read
// whole, for the diff's highlighter to start at the file's first line rather
// than the hunk's (#800). A blob over limit bytes is not read at all: whole
// is false and the caller does without, so one generated file cannot cost a
// diff load its size in memory.
//
//	text, whole, err := vcs.NewCLI().Blob("/wt/parser-fix", base, "main.go", 256<<10)
func (c *CLI) Blob(dir, rev, path string, limit int) ([]byte, bool, error) {
	object := rev + ":" + path
	size, err := c.run(dir, "cat-file", "-s", object)
	if err != nil {
		return nil, false, fmt.Errorf("vcs: sizing %s in %q: %w", object, dir, err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(size))
	if err != nil {
		return nil, false, fmt.Errorf("vcs: %s in %q has size %q, want a number: %w", object, dir, size, err)
	}
	if n > limit {
		return nil, false, nil
	}
	out, err := c.capture(dir, 0, "cat-file", "blob", object)
	if err != nil {
		return nil, false, fmt.Errorf("vcs: reading %s in %q: %w", object, dir, err)
	}
	return []byte(out), true, nil
}
