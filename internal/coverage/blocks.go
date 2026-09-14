package coverage

import (
	"bufio"
	"io"
	"strings"
)

// Block is one record of a Go coverage profile, kept whole.
//
//	blocks, err := coverage.ParseGoBlocks(f, "github.com/WilsonSousajr/omatty")
//	for _, b := range blocks { ... }
//
// Path is repo-relative, the same shape Profile keys by. Covered rather than a
// raw count because that is the only question anything here asks of it: a block
// that ran at all ran all of its statements.
type Block struct {
	Path                string
	StartLine, StartCol int
	EndLine, EndCol     int
	NumStmt             int
	Covered             bool
}

// ParseGoBlocks reads `go test -coverprofile` output without collapsing it.
//
// ParseGo answers "did line 42 run", which is the question a diff overlay asks.
// Scoring a *function* asks a different one - how many statements it has and
// how many ran - and that needs the columns and the statement count ParseGo
// discards. Columns matter because two functions may share a line
// (`func f() {}; func g() {}` is legal), so a line-only attribution would hand
// one of them the other's statements.
//
// Same tolerance as ParseGo, for the same reason: a profile is written by
// another tool, and one odd record is not worth losing the rest of the file.
func ParseGoBlocks(r io.Reader, modulePath string) ([]Block, error) {
	var blocks []Block
	prefix := strings.TrimSuffix(modulePath, "/") + "/"
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 0, 64*1024), maxLineBytes)
	for scan.Scan() {
		if path, s, ok := goRecord(scan.Text(), prefix); ok {
			blocks = append(blocks, block(path, s))
		}
	}
	if err := scan.Err(); err != nil {
		return nil, wrap("reading a Go coverage profile", err)
	}
	return blocks, nil
}

// block widens one parsed record into the exported shape.
func block(path string, s span) Block {
	return Block{
		Path: path, StartLine: s.first, StartCol: s.firstCol,
		EndLine: s.last, EndCol: s.lastCol,
		NumStmt: s.numStmt, Covered: s.covered,
	}
}
