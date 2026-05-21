package parser

import (
	"github.com/chenota/acc/internal/ast"
	"github.com/chenota/acc/internal/lexer"
)

func parseExpr(t *lexer.TokenList) (ast.Expr, bool) {
	loc := t.Mark()

	intVal, ok := t.ExpectInteger()
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	// Don't try to size constants in initial parsing phase
	return &ast.ExprInt{Value: intVal}, true
}
