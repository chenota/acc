package lexer

import (
	"io"

	"github.com/chenota/acc/internal/diagnostic"
)

// TokenKind represents a specific kind of token
type TokenKind int

// Token represents a single token in the acc language
type Token struct {
	Kind TokenKind
	Text string
	Pos  diagnostic.Pos
}

// Tokenize processes an input file into a list of tokens
func Tokenize(r io.Reader, options ...Option) (*TokenList, error) {
	cfg := config{}

	for _, o := range options {
		o(&cfg)
	}

	bytes, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var line int
	var col int

	var tokens []Token

	i := 0
	for i < len(bytes) {
		var bestKind TokenKind
		var bestLen int

		pos := diagnostic.Pos{File: cfg.FileName, Col: col + 1, Line: line + 1}

		for _, rule := range rules {
			if loc := rule.pattern.FindIndex(bytes[i:]); loc != nil && loc[1] > bestLen {
				bestLen = loc[1]
				bestKind = rule.kind
			}
		}

		if bestLen == 0 {
			return nil, diagnostic.NewError("invalid token", pos)
		}

		if !(bestKind == KWhitespace || bestKind == KNewLines) {
			tokens = append(tokens, Token{
				Kind: bestKind,
				Text: string(bytes[i : i+bestLen]),
				Pos:  pos,
			})
		}

		i += bestLen

		if bestKind == KNewLines {
			line += bestLen
			col = 0
		} else {
			col += bestLen
		}
	}

	return NewTokenList(tokens), nil
}
