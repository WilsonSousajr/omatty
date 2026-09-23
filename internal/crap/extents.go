package crap

import (
	"go/ast"
	"go/parser"
	"go/token"

	"github.com/WilsonSousajr/omatty/internal/coverage"
)

// extent is one function declaration's span, in the same coordinates a
// coverage profile uses.
type extent struct {
	name                string
	startLine, startCol int
	endLine, endCol     int
	complexity          int
	// statements is counted from the source, and is only ever used when the
	// coverage profile has nothing for this file. It separates the two facts
	// a zero denominator would otherwise merge: a function with nothing in it
	// is covered, a function nothing profiled is not.
	statements int
}

// extentsIn parses one file and returns a span per function it declares.
//
// Functions without a body - assembly stubs - are skipped, because cmd/cover
// skips them too and a function with no Go source has no statements to cover.
func extentsIn(path string) ([]extent, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, wrap("parsing "+path, err)
	}
	var found []extent
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Body != nil {
			found = append(found, extentOf(fset, fn))
		}
	}
	return found, nil
}

// extentOf measures one function. The position is the `func` keyword, not the
// doc comment, which is the position a coverage profile's blocks are relative
// to and the line `go tool cover -func` prints.
func extentOf(fset *token.FileSet, fn *ast.FuncDecl) extent {
	start, end := fset.Position(fn.Pos()), fset.Position(fn.End())
	return extent{
		name:      fn.Name.Name,
		startLine: start.Line, startCol: start.Column,
		endLine: end.Line, endCol: end.Column,
		complexity: complexityOf(fn),
		statements: statementsOf(fn),
	}
}

// statementsOf counts the statements in a function's source.
//
// It does not try to reproduce cmd/cover's own count, and does not need to:
// this number is a fallback denominator for a file the profile never mentions,
// where every function scores zero coverage whatever the denominator is. Its
// only real job is to be zero for `func f() {}` and non-zero for everything
// else. Containers are skipped so a block does not count as a statement
// alongside the statements inside it.
func statementsOf(fn *ast.FuncDecl) int {
	statements := 0
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause, *ast.LabeledStmt:
		default:
			if _, isStatement := n.(ast.Stmt); isStatement {
				statements++
			}
		}
		return true
	})
	return statements
}

// holds reports whether a coverage block falls inside this function.
//
// This is cmd/cover's own test, kept rather than reduced to "same line range",
// because two functions can share a line - `func f() {}; func g() {}` is legal
// Go - and a line-only rule would hand one of them the other's statements.
func (e extent) holds(b coverage.Block) bool {
	startsAfter := b.StartLine > e.endLine ||
		(b.StartLine == e.endLine && b.StartCol >= e.endCol)
	endsBefore := b.EndLine < e.startLine ||
		(b.EndLine == e.startLine && b.EndCol <= e.startCol)
	return !startsAfter && !endsBefore
}
