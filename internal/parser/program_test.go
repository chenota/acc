package parser

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chenota/acc/internal/ast"
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/types"
)

func TestProgram_MainFunc(t *testing.T) {
	// we're cheating a little bit by using the lexer here but it makes writing tests much easier
	inputStr := `fun main () -> int { return 0; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)

	fun := funcs[0]
	assert.Equal(t, fun.Name, "main")

	_, ok := fun.Type.Output.(types.Int)
	assert.True(t, ok)

	require.Len(t, fun.Body.Statements, 1)
	ret, ok := fun.Body.Statements[0].(ast.StmtReturn)
	require.True(t, ok)

	e, ok := ret.Expr.(ast.ExprInt)
	require.True(t, ok)
	assert.NotNil(t, e.Value)
}

func TestProgram_MultipleReturns(t *testing.T) {
	inputStr := `fun main () -> int { return 0; return 1; return 2; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	assert.Len(t, funcs[0].Body.Statements, 1)
}
