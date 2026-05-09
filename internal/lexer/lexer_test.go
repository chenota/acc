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

func TestLexer_Invalid(t *testing.T) {
	input := strings.NewReader("%!*$-")

	_, err := Tokenize(input)
	assert.Error(t, err)
}
