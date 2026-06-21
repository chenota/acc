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
	curr, ok := t.Peek()
	if !ok || curr.Kind != kind {
		return Token{}, false
	}
	t.Advance()
	return curr, true
}

func (t *TokenList) ExpectIdentifier() (string, bool) {
	ident, ok := t.Expect(KIdentifier)
	if !ok {
		return "", false
	}

	return ident.Text, true
}

// ExpectInteger returns a parsed integer value from the token list
func (t *TokenList) ExpectInteger() (*big.Int, bool) {
	token, ok := t.Expect(KInteger)
	if !ok {
		return nil, false
	}

	return token.ParseInteger()
}

func (t *TokenList) Empty() bool {
	if t == nil {
		return true
	}
	return t.i >= len(t.tokens)
}

func (t *TokenList) Pos() diagnostic.Pos {
	if t.Empty() {
		// if we're at the end then return last token pos + 1
		if len(t.tokens) > 0 {
			pos := t.tokens[len(t.tokens)-1].Pos
			pos.Col += 1
			return pos
		}
		return diagnostic.Pos{}
	}

	return t.tokens[t.i].Pos
}

func (t *TokenList) Peek() (Token, bool) {
	if t.Empty() {
		return Token{}, false
	}
	return t.tokens[t.i], true
}

func (t *TokenList) Advance() {
	if !t.Empty() {
		t.i += 1
	}
}

func (t Token) ParseInteger() (*big.Int, bool) {
	reducedText := strings.ReplaceAll(t.Text, `_`, ``)
	return new(big.Int).SetString(reducedText, 10)
}
