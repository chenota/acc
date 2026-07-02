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

	e := decl.List[2]
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

	e := decl.List[2]
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
	assert.Equal(t, decl.Sym, assign.List[0].Sym)
}

func TestAnalyze_Assignment_BeforeDeclared(t *testing.T) {
	funcs := mustParse(t, `fun main () -> int { x = 15; return x; }`)

	require.Error(t, Analyze(funcs))
}

func TestAnalyze_Assignment_InvalidLvalue(t *testing.T) {
	tests := []struct {
		name string
		test string
	}{
		{"integer literal", `fun main () -> int { 1 = 2; return 0; }`},
		{"arithmetic expression", `fun main () -> int { let x int = 1; x + 1 = 2; return 0; }`},
		{"negation", `fun main () -> int { let x int = 1; -x = 2; return 0; }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			funcs := mustParse(t, tt.test)
			assert.Error(t, Analyze(funcs))
		})
	}
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
	assert.Equal(t, decl.Sym, assign.List[0].Sym)
}

func TestAnalyze_Call(t *testing.T) {
	funcs := mustParse(t, `fun f (x int) -> int { return x; } fun main () -> int { return f(1); }`)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 2)
	f := funcs[0]
	main := funcs[1]

	require.Len(t, main.List, 1)
	ret := main.List[0]
	require.Len(t, ret.List, 1)
	call := ret.List[0]
	assert.Equal(t, ir.OpCall, call.Op)

	// the call expression takes the callee's result type
	assert.True(t, types.Equal(types.Int(), call.Type))

	// callee resolves to the function's symbol and function type
	require.Len(t, call.List, 2)
	callee := call.List[0]
	require.NotNil(t, callee.Sym)
	assert.Equal(t, f.Sym, callee.Sym)
	assert.True(t, types.Equal(types.Function([]*types.Type{types.Int()}, types.Int()), callee.Type))

	// the untyped literal argument is resolved to the parameter type
	assert.True(t, types.Equal(types.Int(), call.List[1].Type))
}

func TestAnalyze_Call_ZeroArgs(t *testing.T) {
	funcs := mustParse(t, `fun f () -> int { return 0; } fun main () -> int { return f(); }`)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 2)
	main := funcs[1]

	require.Len(t, main.List, 1)
	call := main.List[0].List[0]
	assert.Equal(t, ir.OpCall, call.Op)

	require.Len(t, call.List, 1)
	assert.True(t, types.Equal(types.Int(), call.Type))
}

func TestAnalyze_CallErr(t *testing.T) {
	tests := []struct {
		name string
		test string
	}{
		{"non-function callee", `fun main () -> int { let x int = 1; return x(1); }`},
		{"too few args", `fun f (x int) -> int { return x; } fun main () -> int { return f(); }`},
		{"too many args", `fun f (x int) -> int { return x; } fun main () -> int { return f(1, 2); }`},
		{"undefined callee", `fun main () -> int { return g(1); }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			funcs := mustParse(t, tt.test)
			assert.Error(t, Analyze(funcs))
		})
	}
}

func TestAnalyze_ForwardReference(t *testing.T) {
	// main calls f, which is declared after main
	funcs := mustParse(t, `fun main () -> int { return f(1); } fun f (x int) -> int { return x; }`)

	require.NoError(t, Analyze(funcs))

	require.Len(t, funcs, 2)
	main := funcs[0]
	f := funcs[1]

	// the forward call actually resolved to the later function's symbol
	require.Len(t, main.List, 1)
	call := main.List[0].List[0]
	require.Equal(t, ir.OpCall, call.Op)
	require.Len(t, call.List, 2)
	assert.Equal(t, f.Sym, call.List[0].Sym)
}

func TestAnalyze_DuplicateFunction(t *testing.T) {
	funcs := mustParse(t, `fun f () -> int { return 0; } fun f () -> int { return 1; } fun main () -> int { return 0; }`)

	require.Error(t, Analyze(funcs))
}

func mustParse(t *testing.T, inputStr string) []*ir.Node {
	t.Helper()

	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	return funcs
}
