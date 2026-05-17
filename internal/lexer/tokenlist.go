package lexer

type TokenList struct {
	i      int
	tokens []Token
}

func NewTokenList(tokens []Token) *TokenList {
	return &TokenList{
		tokens: tokens,
	}
}

func (t *TokenList) Mark() int {
	return t.i
}

func (t *TokenList) Restore(i int) {
	t.i = i
}

func (t *TokenList) Expect(kind TokenKind) (Token, bool) {
	if t == nil || t.i >= len(t.tokens) || t.tokens[t.i].Kind != kind {
		return Token{}, false
	}

	t.i += 1
	return t.tokens[t.i-1], true
}
