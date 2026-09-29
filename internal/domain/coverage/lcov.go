package coverage

import (
	"bufio"
	"io"
	"path/filepath"
	"strconv"
	"strings"
)

// ParseLCOV reads an lcov tracefile.
//
//	p, err := coverage.ParseLCOV(f, sess.Dir)
//
// lcov is the one format that reaches everything that is not Go: c8 and nyc
// for JS and TypeScript, tarpaulin and llvm-cov for Rust, coverage.py's
// --format=lcov for Python, gcov for C and C++.
//
// root is required because writers disagree about absolute versus relative
// SF paths, and a diff names repo-relative ones. A file resolving outside
// root is skipped: it is not this diff's business, and inventing a path for
// it would put markers on the wrong lines.
//
// Only DA records carry a line verdict. FN, FNDA, BRDA, LF and LH are
// function, branch and summary counts, and reading any of them as a line
// would mark lines the file does not have.
func ParseLCOV(r io.Reader, root string) (Profile, error) {
	p := Profile{Files: map[string]File{}}
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	current := ""
	for scan.Scan() {
		current = lcovRecord(p, strings.TrimSpace(scan.Text()), root, current)
	}
	if err := scan.Err(); err != nil {
		return Profile{}, wrap("reading an lcov profile", err)
	}
	return p, nil
}

// lcovRecord folds one line in and returns the file still open, if any.
func lcovRecord(p Profile, line, root, current string) string {
	switch {
	case strings.HasPrefix(line, "SF:"):
		return relativeTo(root, strings.TrimPrefix(line, "SF:"))
	case line == "end_of_record":
		return ""
	case strings.HasPrefix(line, "DA:") && current != "":
		markDA(p, current, strings.TrimPrefix(line, "DA:"))
	}
	return current
}

// markDA reads "line,count", dropping anything that will not parse - a
// profile is written by a tool, and one odd record is not worth the file.
func markDA(p Profile, path, rest string) {
	number, count, found := strings.Cut(rest, ",")
	if !found {
		return
	}
	line, err := strconv.Atoi(number)
	if err != nil || line <= 0 {
		return
	}
	// Fields, not the raw text: some writers append a checksum after the
	// count. Fields first, because Fields("") is empty and indexing it is a
	// panic - and a panic here would take the model down mid-render, where
	// invariant 6's recover does not reach.
	fields := strings.Fields(count)
	if len(fields) == 0 {
		return
	}
	hits, err := strconv.Atoi(fields[0])
	if err != nil {
		return
	}
	p.mark(path, line, hits > 0)
}

// relativeTo makes an SF path repo-relative, or "" when it escapes the root.
func relativeTo(root, path string) string {
	if !filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || strings.HasPrefix(rel, "..") {
		return ""
	}
	return rel
}
