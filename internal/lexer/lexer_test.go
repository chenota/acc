package lexer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLexer_Keyword(t *testing.T) {
	input := strings.NewReader("fun fun")

	tokens, err := Tokenize(input)
	require.NoError(t, err)

	require.Len(t, tokens, 2)
	assert.Equal(t, KindFunKw, tokens[0].Kind)
	assert.Equal(t, "fun", tokens[0].Text)
}

func TestLexer_Identifier(t *testing.T) {
	tests := []string{
		"hello",
		"this_is_valid",
		"cool1",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			tokens, err := Tokenize(strings.NewReader(test))
			require.NoError(t, err)

			require.Len(t, tokens, 1)
			assert.Equal(t, KindIdentifier, tokens[0].Kind)
			assert.Equal(t, test, tokens[0].Text)
		})
	}
}

func TestLexer_Integer(t *testing.T) {
	tests := []string{
		"1",
		"-123",
		"1_000_000",
	}

	for _, test := range tests {
		t.Run(test, func(t *testing.T) {
			tokens, err := Tokenize(strings.NewReader(test))
			require.NoError(t, err)

			require.Len(t, tokens, 1)
			assert.Equal(t, KindInteger, tokens[0].Kind)
			assert.Equal(t, test, tokens[0].Text)
		})
	}
}

func TestLexer_Invalid(t *testing.T) {
	input := strings.NewReader("%!*$-")

	_, err := Tokenize(input)
	assert.Error(t, err)
}
