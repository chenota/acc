package parser

import (
	"github.com/chenota/acc/internal/ast"
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/types"
)

func parseType(t *lexer.TokenList) (*ast.Node, bool) {
	loc := t.Mark()
	pos := t.Pos()

	if _, ok := t.Expect(lexer.KindIntKw); ok {
		// "int" aliases to "int32"
		return &ast.Node{
			Op:   ast.OpType,
			Pos:  pos,
			Type: types.Int32,
		}, true
	}

	t.Restore(loc)
	return nil, false
}
