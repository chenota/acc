package semantic

import (
	"strings"
	"testing"

	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/parser"
	"github.com/chenota/acc/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyze_Basic(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { return 0; }`)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Type)
	assert.Equal(t, types.Function(nil, types.Int()), fun.Type)

	require.NotNil(t, fun.Sym)
	assert.Equal(t, "main", fun.Sym.Name)

	require.Len(t, fun.List, 1)
	e := fun.List[0].List[0]
	assert.Equal(t, types.Int(), e.Type)
}

func TestAnalyze_ParamTypes(t *testing.T) {
	funcs := mustParse(t, `fun main (x int, y int) -> int { return 0; }`)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	// each param node should carry the type pulled up from its type node
	require.Len(t, fun.Signature.Params, 2)
	assert.True(t, types.Equal(types.Int(), fun.Signature.Params[0].Type))
	assert.True(t, types.Equal(types.Int(), fun.Signature.Params[1].Type))

	// and those types should be reflected in the function's own type
	require.NotNil(t, fun.Type)
	want := types.Function([]*types.Type{types.Int(), types.Int()}, types.Int())
	assert.True(t, types.Equal(want, fun.Type))
}

func TestAnalyze_DuplicateParam(t *testing.T) {
	funcs := mustParse(t, `fun main (x int, x int) -> int { return 0; }`)

	assert.Error(t, Analyze(funcs))
}

func TestAnalyze_Overflow(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { return 2_147_483_648; }`)

	assert.Error(t, Analyze(funcs))
}

func TestAnalyze_SimpleBop(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { return 1 + 2 * 3; }`)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Type)
	assert.Equal(t, types.Function(nil, types.Int()), fun.Type)

	require.Len(t, fun.List, 1)
	bopExpr := fun.List[0].List[0]

	assert.Equal(t, types.Int(), bopExpr.Type)
	assert.Equal(t, types.Int(), bopExpr.List[0].Type)
	assert.Equal(t, types.Int(), bopExpr.List[1].Type)
}

func TestAnalyze_VariableDeclaration(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { let x int = 10; return 0; }`)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Type)
	assert.Equal(t, types.Function(nil, types.Int()), fun.Type)

	require.Len(t, fun.List, 2)
	decl := fun.List[0]
	require.NotNil(t, decl.Sym)
	assert.Equal(t, types.Int(), decl.Sym.Type)
	assert.Equal(t, "x", decl.Sym.Name)

	e := decl.List[1]
	require.NotNil(t, e)
	assert.Equal(t, types.Int(), e.Type)
}

func TestAnalyze_VariableDeclaration_Inference(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { let x = 10; return 0; }`)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Type)
	assert.Equal(t, types.Function(nil, types.Int()), fun.Type)

	require.Len(t, fun.List, 2)
	decl := fun.List[0]
	require.NotNil(t, decl.Sym)
	assert.Equal(t, types.Int(), decl.Sym.Type)
	assert.Equal(t, "x", decl.Sym.Name)

	e := decl.List[1]
	require.NotNil(t, e)
	assert.Equal(t, types.Int(), e.Type)
}

func TestAnalyze_VariableDeclaration_Redeclare(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { let x = 10; let x = 15; return 0; }`)

	require.Error(t, Analyze(funcs))
}

func TestAnalyze_VariableUsage(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { let x = 10; return x; }`)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Type)
	assert.Equal(t, types.Function(nil, types.Int()), fun.Type)

	require.Len(t, fun.List, 2)
	ret := fun.List[1]

	require.NotNil(t, ret)
	require.Len(t, ret.List, 1)
	e := ret.List[0]

	require.NotNil(t, e)
	assert.Equal(t, types.Int(), e.Type)
	assert.Equal(t, fun.List[0].Sym, e.Sym)
}

func TestAnalyze_VariableUsage_BeforeDeclared(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { return x; }`)

	require.Error(t, Analyze(funcs))
}

func TestAnalyze_Assignment(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { let x int = 10; x = 15; return x; }`)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Type)
	assert.Equal(t, types.Function(nil, types.Int()), fun.Type)

	require.Len(t, fun.List, 3)
	decl := fun.List[0]
	assign := fun.List[1]
	assert.Equal(t, decl.Sym, assign.Sym)
}

func TestAnalyze_Assignment_BeforeDeclared(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { x = 15; return x; }`)

	require.Error(t, Analyze(funcs))
}

func TestAnalyze_Negation(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { let x = -10; return -x; }`)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 2)
	ret := fun.List[1]

	require.NotNil(t, ret)
	require.Len(t, ret.List, 1)
	e := ret.List[0]

	require.NotNil(t, e)
	assert.Equal(t, types.Int(), e.Type)
}

func TestAnalyze_AssignmentOp(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { let x int = 10; x += 15; return x; }`)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 3)
	decl := fun.List[0]
	assign := fun.List[1]
	assert.Equal(t, decl.Sym, assign.Sym)
}

func mustParse(t *testing.T, inputStr string) []*ir.Node {
	t.Helper()

	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	return funcs
}
