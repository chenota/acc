package parser

import (
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/types"
)

func parseType(t *lexer.TokenList) (*types.Type, bool) {
	loc := t.Mark()

	if _, ok := t.Expect(lexer.KindLParen); ok {
		if _, ok := t.Expect(lexer.KindRParen); !ok {
			t.Restore(loc)
			return nil, false
		}

		// Seeing an arrow indicates that this is a function type
		if _, ok := t.Expect(lexer.KindArrow); ok {
			if returnType, ok := parseType(t); ok {
				return &types.Type{
					Kind:   types.KFunction,
					Output: returnType,
				}, true
			} else {
				t.Restore(loc)
				return nil, false
			}
		}

		return &types.Type{Kind: types.KUnit}, true
	}

	// Try to parse an int
	if _, ok := t.Expect(lexer.KindIntKw); ok {
		// "int" aliases to "int32"
		return &types.Type{Kind: types.KInt32}, true
	}

	return nil, false
}
