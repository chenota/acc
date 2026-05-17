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

	token, ok := tokens.Expect(KindFunKw)
	require.True(t, ok)

	assert.Equal(t, KindFunKw, token.Kind)
	assert.Equal(t, "fun", token.Text)
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

			token, ok := tokens.Expect(KindIdentifier)
			require.True(t, ok)

			assert.Equal(t, KindIdentifier, token.Kind)
			assert.Equal(t, test, token.Text)
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

			token, ok := tokens.Expect(KindInteger)
			require.True(t, ok)

			assert.Equal(t, KindInteger, token.Kind)
			assert.Equal(t, test, token.Text)
		})
	}
}

func TestLexer_Invalid(t *testing.T) {
	input := strings.NewReader("%!*$-")

	_, err := Tokenize(input)
	assert.Error(t, err)
}
