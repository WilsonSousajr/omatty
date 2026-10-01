package review

import (
	"bytes"
	"strings"
)

// PreviewLimit bounds what a preview reads; a generated file must not stall
// the frame.
const PreviewLimit = 256 << 10

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

// PreviewOf classifies the bytes: a NUL means binary, more than the limit
// means truncated back to the last whole line, so the view never shows half a
// rune or half a statement.
//
//	p := review.PreviewOf("main.go", head)
func PreviewOf(path string, buf []byte) Preview {
	p := Preview{Path: path}
	if bytes.IndexByte(buf, 0) >= 0 {
		p.Binary = true
		return p
	}
	if len(buf) > PreviewLimit {
		p.Truncated = true
		buf = buf[:bytes.LastIndexByte(buf[:PreviewLimit], '\n')+1]
	}
	p.Lines = strings.Split(strings.TrimSuffix(string(buf), "\n"), "\n")
	return p
}
