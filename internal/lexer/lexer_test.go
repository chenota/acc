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

	token, ok := tokens.Expect(KFunKw)
	require.True(t, ok)

	assert.Equal(t, KFunKw, token.Kind)
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

			id, ok := tokens.ExpectIdentifier()
			require.True(t, ok)
			assert.Equal(t, test, id)
		})
	}
}

func TestLexer_Integer(t *testing.T) {
	tests := []struct {
		test     string
		expected int64
	}{
		{"1", 1},
		{"1_000_000", 1000000},
	}

	for _, test := range tests {
		t.Run(test.test, func(t *testing.T) {
			tokens, err := Tokenize(strings.NewReader(test.test))
			require.NoError(t, err)

			token, ok := tokens.ExpectInteger()
			require.True(t, ok)
			assert.Equal(t, test.expected, token.Int64())

			// these all should be the last token in the list
			assert.True(t, tokens.Empty())
		})
	}
}

func TestLexer_Plus(t *testing.T) {
	tokens, err := Tokenize(strings.NewReader("+"))
	require.NoError(t, err)

	_, ok := tokens.Expect(KPlus)
	require.True(t, ok)
}

func TestLexer_Star(t *testing.T) {
	tokens, err := Tokenize(strings.NewReader("*"))
	require.NoError(t, err)

	_, ok := tokens.Expect(KStar)
	require.True(t, ok)
}

func TestLexer_Invalid(t *testing.T) {
	input := strings.NewReader("%!*$-")

	_, err := Tokenize(input)
	assert.Error(t, err)
}

func TestLexer_Pos(t *testing.T) {
	t.Run("couple of identifiers", func(t *testing.T) {
		tokens, err := Tokenize(strings.NewReader("hello world"), WithFileName("hello"))
		require.NoError(t, err)

		tokens.Expect(KIdentifier)
		token, ok := tokens.Expect(KIdentifier)
		require.True(t, ok)

		assert.Equal(t, 7, token.Pos.Col)
		assert.Equal(t, 1, token.Pos.Line)
		assert.Equal(t, "hello", token.Pos.File)
	})
	t.Run("newlines", func(t *testing.T) {
		tokens, err := Tokenize(strings.NewReader("\n\n\n hello"), WithFileName("burger"))
		require.NoError(t, err)

		token, ok := tokens.Expect(KIdentifier)
		require.True(t, ok)

		assert.Equal(t, 2, token.Pos.Col)
		assert.Equal(t, 4, token.Pos.Line)
		assert.Equal(t, "burger", token.Pos.File)
	})
}
