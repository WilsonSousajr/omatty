package coverage_test

import (
	"strings"
	"testing"

	"github.com/WilsonSousajr/omatty/internal/coverage"
)

// ParseGo answers "did line 42 run", which is what a diff overlay asks. CRAP
// asks a different question - how many statements a *function* has and how many
// of them ran - and that needs the columns and the statement count ParseGo
// throws away. Both read the same records, so they share one parser (#262).
func TestParseGoBlocks_keepsColumnsAndStatementCounts(t *testing.T) {
	profile := "mode: set\n" +
		omattyModule + "/internal/gate/run.go:30.10,33.4 2 0\n" +
		omattyModule + "/internal/gate/run.go:35.2,35.16 1 7\n"

	blocks, err := coverage.ParseGoBlocks(strings.NewReader(profile), omattyModule)

	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 2 {
		t.Fatalf("got %d blocks, want 2: %+v", len(blocks), blocks)
	}
	want := coverage.Block{
		Path: "internal/gate/run.go", StartLine: 30, StartCol: 10,
		EndLine: 33, EndCol: 4, NumStmt: 2, Covered: false,
	}
	if blocks[0] != want {
		t.Errorf("blocks[0] = %+v, want %+v", blocks[0], want)
	}
	if !blocks[1].Covered || blocks[1].NumStmt != 1 {
		t.Errorf("blocks[1] = %+v, want 1 statement, covered", blocks[1])
	}
}

// A block whose whole extent is one line still carries both columns, because
// two functions can share a line - `func f() {}; func g() {}` is legal Go, and
// attributing by line alone would give one of them the other's statements.
func TestParseGoBlocks_keepsBothColumnsOnASingleLineBlock(t *testing.T) {
	blocks, err := coverage.ParseGoBlocks(
		strings.NewReader("mode: set\nm/a.go:8.13,8.27 1 3\n"), "m")

	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(blocks))
	}
	if b := blocks[0]; b.StartCol != 13 || b.EndCol != 27 {
		t.Errorf("columns = %d..%d, want 13..27", b.StartCol, b.EndCol)
	}
}

// The same tolerance ParseGo has: a record written by another tool that will
// not parse is skipped, never fatal, because the rest of the file is still
// worth reading.
func TestParseGoBlocks_skipsRecordsItCannotTrust(t *testing.T) {
	profile := "mode: set\nm/a.go:nonsense 1 1\nother/b.go:1.1,2.2 1 1\nm/c.go:1.1,2.2 1 1\n"

	blocks, err := coverage.ParseGoBlocks(strings.NewReader(profile), "m")

	if err != nil {
		t.Fatal(err)
	}
	if len(blocks) != 1 || blocks[0].Path != "c.go" {
		t.Errorf("blocks = %+v, want only c.go", blocks)
	}
}

// ParseGo is now built on ParseGoBlocks, so this pins that the collapse to
// per-line verdicts still happens - the diff overlay depends on it (#251).
func TestParseGo_stillMarksEveryLineABlockSpans(t *testing.T) {
	p, err := coverage.ParseGo(
		strings.NewReader("mode: set\nm/a.go:10.2,13.4 2 1\n"), "m")

	if err != nil {
		t.Fatal(err)
	}
	for line := 10; line <= 13; line++ {
		if covered, known := p.Files["a.go"].Lines[line]; !known || !covered {
			t.Errorf("line %d: covered=%v known=%v, want covered", line, covered, known)
		}
	}
}
