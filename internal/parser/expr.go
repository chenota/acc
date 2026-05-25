package parser

import (
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/lexer"
)

func parseExpr(t *lexer.TokenList) (*ir.Node, bool) {
	loc := t.Mark()
	pos := t.Pos()

	intVal, ok := t.ExpectInteger()
	if !ok {
		t.Restore(loc)
		return nil, false
	}

	// We're purposely leaving this untyped
	return &ir.Node{
		Op:  ir.OpInt,
		Pos: pos,
		Val: intVal,
	}, true
}
