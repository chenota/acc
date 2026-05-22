package parser

import (
	"github.com/chenota/acc/internal/ast"
	"github.com/chenota/acc/internal/lexer"
)

func parseExpr(t *lexer.TokenList) (*ast.Node, bool) {
	loc := t.Mark()
	pos := t.Pos()

	intVal, ok := t.ExpectInteger()
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	// We're purposely leaving this untyped
	return &ast.Node{
		Op:  ast.OpInt,
		Pos: pos,
		Val: intVal,
	}, true
}
