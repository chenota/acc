package lexer

import (
	"errors"
	"io"
)

func Tokenize(r io.Reader) ([]Token, error) {
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

	return tokens, nil
}
