package semantic

import (
	"strings"
	"testing"

	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/parser"
	"github.com/chenota/acc/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyze_Basic(t *testing.T) {
	inputStr := `fun main () -> int { return 0; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Type)
	assert.Equal(t, types.KFunction, fun.Type.Kind)

	require.NotNil(t, fun.Sym)
	assert.Equal(t, "main", fun.Sym.Name)

	require.Len(t, fun.List, 1)
	e := fun.List[0].List[0]
	assert.Equal(t, types.KInt32, e.Type.Kind)
}

func TestAnalyze_Overflow(t *testing.T) {
	inputStr := `fun main () -> int { return 2_147_483_648; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	assert.Error(t, Analyze(funcs))
}
