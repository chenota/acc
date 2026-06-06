package lexer

import (
	"math/big"
	"strings"

	"github.com/chenota/acc/internal/diagnostic"
)

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

// ExpectInteger returns a parsed integer value from the token list
func (t *TokenList) ExpectInteger() (*big.Int, bool) {
	token, ok := t.Expect(KindInteger)
	if !ok {
		return nil, false
	}

	return token.ParseInteger()
}

func (t *TokenList) Empty() bool {
	return t.i >= len(t.tokens)
}

func (t *TokenList) Pos() diagnostic.Pos {
	if t == nil || t.i >= len(t.tokens) {
		return diagnostic.Pos{}
	}

	return t.tokens[t.i].Pos
}

func (t Token) ParseInteger() (*big.Int, bool) {
	reducedText := strings.ReplaceAll(t.Text, `_`, ``)
	return new(big.Int).SetString(reducedText, 10)
}
