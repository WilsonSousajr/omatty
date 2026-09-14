package coverage

import (
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
	blocks, err := ParseGoBlocks(r, modulePath)
	if err != nil {
		return Profile{}, err
	}
	p := Profile{Files: map[string]File{}}
	for _, b := range blocks {
		markBlock(p, b)
	}
	return p, nil
}

// span is one profile record: where it starts and ends, how many statements it
// holds, and whether it ran. Columns and numStmt are here for ParseGoBlocks;
// ParseGo reads only the lines.
type span struct {
	first, last       int
	firstCol, lastCol int
	numStmt           int
	covered           bool
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
	s, ok := goRange(fields[0])
	if !ok {
		return span{}, false
	}
	numStmt, stmtErr := strconv.Atoi(fields[1])
	count, countErr := strconv.Atoi(fields[2])
	if stmtErr != nil || countErr != nil || numStmt < 0 {
		return span{}, false
	}
	s.numStmt, s.covered = numStmt, count > 0
	return s, true
}

// goRange reads "53.35,58.2" as line 53 column 35 through line 58 column 2.
func goRange(r string) (span, bool) {
	from, to, found := strings.Cut(r, ",")
	if !found {
		return span{}, false
	}
	first, firstCol, ok := positionOf(from)
	if !ok {
		return span{}, false
	}
	last, lastCol, ok := positionOf(to)
	if !ok || last < first {
		return span{}, false
	}
	return span{first: first, firstCol: firstCol, last: last, lastCol: lastCol}, true
}

// positionOf reads a "line.column" pair. A column of zero is accepted: only the
// line is required to be real, and cmd/cover has emitted column 0 for a
// synthesised position.
func positionOf(pos string) (int, int, bool) {
	line, column, found := strings.Cut(pos, ".")
	if !found {
		return 0, 0, false
	}
	n, lineErr := strconv.Atoi(line)
	col, colErr := strconv.Atoi(column)
	if lineErr != nil || colErr != nil || n <= 0 || col < 0 {
		return 0, 0, false
	}
	return n, col, true
}

// markBlock records a verdict for every line the block spans.
func markBlock(p Profile, b Block) {
	for line := b.StartLine; line <= b.EndLine; line++ {
		p.mark(b.Path, line, b.Covered)
	}
}
