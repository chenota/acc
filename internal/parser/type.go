package parser

import (
	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/types"
)

func (p *parser) parseType() (*ir.Node, bool) {
	loc := p.t.Mark()
	pos := p.t.Pos()

	if _, ok := p.t.Expect(lexer.KIntKw); ok {
		return &ir.Node{
			Op:   ir.OpType,
			Pos:  pos,
			Type: types.Int(),
		}, true
	}

	p.markErr("expected int keyword")
	p.t.Restore(loc)
	return nil, false
}
