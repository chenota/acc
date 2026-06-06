package parser

import (
	"github.com/chenota/acc/internal/ir"
)

func (p *parser) parseExpr() (*ir.Node, bool) {
	loc := p.t.Mark()
	pos := p.t.Pos()

	intVal, ok := p.t.ExpectInteger()
	if !ok {
		p.markErr("expected integer literal")
		p.t.Restore(loc)
		return nil, false
	}

	// We're purposely leaving this untyped
	return &ir.Node{
		Op:  ir.OpInt,
		Pos: pos,
		Val: intVal,
	}, true
}
