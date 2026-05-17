package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chenota/acc/internal/lexer"
)

func TestProgram_MainFunc(t *testing.T) {
	// we're cheating a little bit by using the lexer here but it makes writing tests much easier
	inputStr := `fun main int { return 0; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	program, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, program.Functions, 1)

	fun := program.Functions[0]
	assert.Equal(t, fun.Name, "main")

	atom, ok := fun.Output.(TypeAtom)
	require.True(t, ok)
	assert.Equal(t, AtomKindInt, atom.Kind)

	require.Len(t, fun.Body.Statements, 1)
	ret, ok := fun.Body.Statements[0].(StmtReturn)
	require.True(t, ok)

	e, ok := ret.Expr.(ExprInt)
	require.True(t, ok)
	assert.NotNil(t, e.Value)
}

func TestProgram_MultipleReturns(t *testing.T) {
	// we're cheating a little bit by using the lexer here but it makes writing tests much easier
	inputStr := `fun main int { return 0; return 1; return 2; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	program, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, program.Functions, 1)
	assert.Len(t, program.Functions[0].Body.Statements, 1)
}
