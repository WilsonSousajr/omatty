package coverage

import (
	"bufio"
	"io"
	"strconv"
	"strings"
)

// ParseGo reads `go test -coverprofile` output.
//
//	p, err := coverage.ParseGo(f, "github.com/WilsonSousajr/omatty")
//
// modulePath is required because Go keys a profile by *import path* while a
// diff names a file:
//
//	github.com/WilsonSousajr/omatty/internal/gate/run.go:30.10,33.4 2 0
//	                                 └─ the diff says internal/gate/run.go
//
// A record that does not carry the prefix belongs to another module - vendored
// or cross-module - and is skipped rather than guessed at, because a guessed
// path puts markers on the wrong lines.
//
// A malformed record is skipped too. A profile is written by a tool, and one
// odd line is not worth losing the rest of the file over.
func ParseGo(r io.Reader, modulePath string) (Profile, error) {
	p := Profile{Files: map[string]File{}}
	prefix := strings.TrimSuffix(modulePath, "/") + "/"
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	for scan.Scan() {
		path, block, ok := goRecord(scan.Text(), prefix)
		if ok {
			markBlock(p, path, block)
		}
	}
	if err := scan.Err(); err != nil {
		return Profile{}, wrap("reading a Go coverage profile", err)
	}
	return p, nil
}

// span is one profile record's line range and whether it ran.
type span struct {
	first, last int
	covered     bool
}

// goRecord splits one line into the file it names and the block it describes.
func goRecord(line, prefix string) (string, span, bool) {
	colon := strings.LastIndex(line, ":")
	if colon < 0 || !strings.HasPrefix(line, prefix) {
		return "", span{}, false
	}
	path := line[len(prefix):colon]
	block, ok := goBlock(line[colon+1:])
	return path, block, ok && path != ""
}

// goBlock reads "startLine.startCol,endLine.endCol numStmts count".
func goBlock(rest string) (span, bool) {
	fields := strings.Fields(rest)
	if len(fields) != 3 {
		return span{}, false
	}
	first, last, ok := goRange(fields[0])
	if !ok {
		return span{}, false
	}
	count, err := strconv.Atoi(fields[2])
	if err != nil {
		return span{}, false
	}
	return span{first: first, last: last, covered: count > 0}, true
}

// goRange reads "53.35,58.2" as the lines 53 to 58.
func goRange(r string) (int, int, bool) {
	from, to, found := strings.Cut(r, ",")
	if !found {
		return 0, 0, false
	}
	first, ok := lineOf(from)
	if !ok {
		return 0, 0, false
	}
	last, ok := lineOf(to)
	return first, last, ok && last >= first
}

// lineOf reads the line number out of a "line.column" pair.
func lineOf(pos string) (int, bool) {
	line, _, found := strings.Cut(pos, ".")
	if !found {
		return 0, false
	}
	n, err := strconv.Atoi(line)
	return n, err == nil && n > 0
}

// markBlock records a verdict for every line the block spans.
func markBlock(p Profile, path string, b span) {
	for line := b.first; line <= b.last; line++ {
		p.mark(path, line, b.covered)
	}
}
