package parser

import (
	"github.com/chenota/acc/internal/lexer"
)

func parseType(t *lexer.TokenList) (Type, bool) {
	_, ok := t.Expect(lexer.KindIntKw)

	if !ok {
		return nil, false
	}

	return TypeAtom{Kind: AtomKindInt}, true
}
