package coverage

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ModulePath is the module path declared by the go.mod at root, or "" when
// there is no module there.
//
//	p, err := coverage.Load(path, dir, coverage.ModulePath(dir))
//
// ParseGo needs it because Go keys a profile by import path while a diff names
// a file. It is read from go.mod rather than asked of `go list -m`: the answer
// is one line of a file the checkout already holds, while the toolchain would
// be a subprocess in front of a diff being drawn, and would fail on a machine
// that has a checkout but no Go.
//
// Empty is a legitimate answer, not an error. An lcov profile needs no module
// path, so a Node or Cargo checkout asks for this and gets silence.
func ModulePath(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "go.mod")) //nolint:gosec // the session's own checkout
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if path, ok := moduleDirective(line); ok {
			return path
		}
	}
	return ""
}

// moduleDirective reads the path out of one line, if that line is the module
// directive. Fields rather than TrimPrefix, so "modules are not this" is not
// mistaken for one.
func moduleDirective(line string) (string, bool) {
	fields := strings.Fields(strings.SplitN(line, "//", 2)[0])
	if len(fields) < 2 || fields[0] != "module" {
		return "", false
	}
	// go.mod permits a quoted path, and the quoting is Go string syntax.
	if unquoted, err := strconv.Unquote(fields[1]); err == nil {
		return unquoted, true
	}
	return fields[1], true
}
