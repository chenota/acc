package lexer

import (
	"errors"
	"io"
)

// TokenKind represents a specific kind of token
type TokenKind int

// Token represents a single token in the acc language
type Token struct {
	Kind TokenKind
	Text string
}

// Tokenize processes an input into a list of tokens
func Tokenize(r io.Reader) (*TokenList, error) {
	bytes, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var tokens []Token

	i := 0
	for i < len(bytes) {
		var bestKind TokenKind
		var bestLen int

		for _, rule := range rules {
			if loc := rule.pattern.FindIndex(bytes[i:]); loc != nil && loc[1] > bestLen {
				bestLen = loc[1]
				bestKind = rule.kind
			}
		}

		if bestLen == 0 {
			return nil, errors.New("invalid token")
		}

		if bestKind != KindSkip {
			tokens = append(tokens, Token{bestKind, string(bytes[i : i+bestLen])})
		}
		i += bestLen
	}

	return NewTokenList(tokens), nil
}
