package lexer

import "math/big"

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
	if i >= 0 && i < len(t.tokens) {
		t.i = i
	}
}

func (t *TokenList) Expect(kind TokenKind) (Token, bool) {
	if t == nil || t.i >= len(t.tokens) || t.tokens[t.i].Kind != kind {
		return Token{}, false
	}

	t.i += 1
	return t.tokens[t.i-1], true
}

func (t *TokenList) ExpectIdentifier() (string, bool) {
	ident, ok := t.Expect(KindIdentifier)
	if !ok {
		return "", false
	}

	return ident.Text, true
}

func (t *TokenList) ExpectInteger() (*big.Int, bool) {
	token, ok := t.Expect(KindInteger)
	if !ok {
		return big.NewInt(0), false
	}

	return new(big.Int).SetString(token.Text, 10)
}
