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

func TestAnalyze_SimpleBop(t *testing.T) {
	inputStr := `fun main () -> int { return 1 + 2 * 3; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Type)
	assert.Equal(t, types.KFunction, fun.Type.Kind)

	require.Len(t, fun.List, 1)
	bopExpr := fun.List[0].List[0]

	assert.Equal(t, types.KInt32, bopExpr.Type.Kind)
	assert.Equal(t, types.KInt32, bopExpr.List[0].Type.Kind)
	assert.Equal(t, types.KInt32, bopExpr.List[1].Type.Kind)
}

func TestAnalyze_VariableDeclaration(t *testing.T) {
	inputStr := `fun main () -> int { let x int = 10; return 0; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Type)
	assert.Equal(t, types.KFunction, fun.Type.Kind)

	require.Len(t, fun.List, 2)
	decl := fun.List[0]
	require.NotNil(t, decl.Sym)
	assert.Equal(t, types.KInt32, decl.Sym.Type.Kind)
	assert.Equal(t, "x", decl.Sym.Name)

	e := decl.List[1]
	require.NotNil(t, e)
	assert.Equal(t, e.Type.Kind, types.KInt32)
}

func TestAnalyze_VariableDeclaration_Inference(t *testing.T) {
	inputStr := `fun main () -> int { let x = 10; return 0; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Type)
	assert.Equal(t, types.KFunction, fun.Type.Kind)

	require.Len(t, fun.List, 2)
	decl := fun.List[0]
	require.NotNil(t, decl.Sym)
	assert.Equal(t, types.KInt32, decl.Sym.Type.Kind)
	assert.Equal(t, "x", decl.Sym.Name)

	e := decl.List[1]
	require.NotNil(t, e)
	assert.Equal(t, e.Type.Kind, types.KInt32)
}

func TestAnalyze_VariableDeclaration_Redeclare(t *testing.T) {
	inputStr := `fun main () -> int { let x = 10; let x = 15; return 0; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.Error(t, Analyze(funcs))
}

func TestAnalyze_VariableUsage(t *testing.T) {
	inputStr := `fun main () -> int { let x = 10; return x; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Type)
	assert.Equal(t, types.KFunction, fun.Type.Kind)

	require.Len(t, fun.List, 2)
	ret := fun.List[1]

	require.NotNil(t, ret)
	require.Len(t, ret.List, 1)
	e := ret.List[0]

	require.NotNil(t, e)
	assert.Equal(t, e.Type.Kind, types.KInt32)
	assert.Equal(t, fun.List[0].Sym, e.Sym)
}

func TestAnalyze_VariableUsage_BeforeDeclared(t *testing.T) {
	inputStr := `fun main () -> int { return x; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.Error(t, Analyze(funcs))
}
