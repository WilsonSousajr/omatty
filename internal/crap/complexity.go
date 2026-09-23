package crap

import (
	"go/ast"
	"go/token"
)

// complexityOf counts a function's cyclomatic complexity.
//
// The node set is fzipp/gocyclo's, deliberately and not incidentally:
// .golangci.yml already enforces `gocyclo: min-complexity: 10`, and two tools
// reporting different complexities for the same function would leave a reader
// with no way to know which to believe. Any change here has to be a change
// there too.
//
// *ast.FuncLit is not special-cased, so branches written inside a closure count
// toward the function that contains it. That is required rather than merely
// convenient: cmd/cover attributes a closure's statements to the enclosing
// function as well, so treating them differently would put the two halves of
// the metric out of step. internal/ui, which is largely tea.Cmd closures, is
// where that would have shown up.
func complexityOf(fn *ast.FuncDecl) int {
	complexity := 1
	ast.Inspect(fn, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.CaseClause, *ast.CommClause:
			complexity++
		case *ast.BinaryExpr:
			if n.Op == token.LAND || n.Op == token.LOR {
				complexity++
			}
		}
		return true
	})
	return complexity
}
