package parser

import (
	"github.com/chenota/acc/internal/ast"
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/types"
)

func parseType(t *lexer.TokenList) (*ast.Node, bool) {
	if _, ok := t.Expect(lexer.KindIntKw); ok {
		// "int" aliases to "int32"
		return typeNode(&types.Type{Kind: types.KInt32}), true
	}

	return nil, false
}

func typeNode(t *types.Type) *ast.Node {
	return &ast.Node{
		Op:   ast.OpType,
		Type: t,
	}
}
