package parser

import "github.com/chenota/acc/internal/lexer"

func parseExpr(t *lexer.TokenList) (Expr, bool) {
	loc := t.Mark()

	intVal, ok := t.ExpectInteger()
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	return ExprInt{Value: intVal}, true
}
