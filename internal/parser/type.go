package parser

import (
	"github.com/chenota/acc/internal/lexer"
)

func parseType(t *lexer.TokenList) (Type, bool) {
	loc := t.Mark()

	if _, ok := t.Expect(lexer.KindLParen); ok {
		if _, ok := t.Expect(lexer.KindRParen); !ok {
			t.Restore(loc)
			return nil, false
		}

		// Seeing an arrow indicates that this is a function type
		if _, ok := t.Expect(lexer.KindArrow); ok {
			if returnType, ok := parseType(t); ok {
				return TypeFunction{Output: returnType}, true
			} else {
				t.Restore(loc)
				return nil, false
			}
		}

		return TypeUnit{}, true
	}

	// Try to parse an int
	if _, ok := t.Expect(lexer.KindIntKw); ok {
		return TypeAtom{Kind: AtomKindInt}, true
	}

	return nil, false
}
