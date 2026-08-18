package semantic

import (
	"slices"
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
	funcs := mustAnalyze(t, `fun main () -> int { return 0; }`)

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
	funcs := mustAnalyze(t, `fun main (x int, y int) -> int { return 0; }`)

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

func TestAnalyze_NoReturnType(t *testing.T) {
	// not main, which is required to return int
	funcs := mustAnalyze(t, `fun f (x int, y int) { return; }`)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	// and those types should be reflected in the function's own type
	require.NotNil(t, fun.Type)
	want := types.Function([]*types.Type{types.Int(), types.Int()}, types.Unit())
	assert.True(t, types.Equal(want, fun.Type))
}

func TestAnalyze_MissingReturn(t *testing.T) {
	_, err := analyzeSrc(t, `fun f () -> int { let x = 1; }`)

	assert.Error(t, err)
}

func TestAnalyze_UnitFunction_NeedsNoReturn(t *testing.T) {
	// control falling off the end of a unit function is an implicit return
	funcs := mustAnalyze(t, `fun f (x *int) { *x = 15; }`)

	// no statement is synthesized to stand in for the implicit return
	require.Len(t, funcs, 1)
	require.Len(t, funcs[0].List, 1)
	assert.Equal(t, ir.OpAssignment, funcs[0].List[0].Op)
}

func TestAnalyze_MainMustReturnInt(t *testing.T) {
	_, err := analyzeSrc(t, `fun main () { }`)

	assert.Error(t, err)
}

func TestAnalyze_NoReturnType_Err(t *testing.T) {
	_, err := analyzeSrc(t, `fun main (x int, y int) -> int { return; }`)

	assert.Error(t, err)
}

func TestAnalyze_CallAsStmt(t *testing.T) {
	funcs := mustAnalyze(t, `fun f () -> int { return 5; } fun main (x int, y int) -> int { f(); return x + y; }`)

	require.Len(t, funcs, 2)
	fun := funcs[1]

	require.Len(t, fun.List, 2)
	call := fun.List[0]

	assert.True(t, types.Equal(types.Int(), call.Type))
}

func TestAnalyze_DuplicateParam(t *testing.T) {
	_, err := analyzeSrc(t, `fun main (x int, x int) -> int { return 0; }`)

	assert.Error(t, err)
}

func TestAnalyze_Overflow(t *testing.T) {
	_, err := analyzeSrc(t, `fun main () -> int { return 2_147_483_648; }`)

	assert.Error(t, err)
}

func TestAnalyze_SimpleBop(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { return 1 + 2 * 3; }`)

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

func TestAnalyze_BopInvalidType(t *testing.T) {
	_, err := analyzeSrc(t, `fun f () {} fun main () -> int { let x = 10; let y = &x + &x; return 0; }`)

	require.Error(t, err)
}

func TestAnalyze_VariableDeclaration(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let x int = 10; return 0; }`)

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
	funcs := mustAnalyze(t, `fun main () -> int { let x = 10; return 0; }`)

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
	_, err := analyzeSrc(t, `fun main () -> int { let x = 10; let x = 15; return 0; }`)

	require.Error(t, err)
}

func TestAnalyze_VariableUsage(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let x = 10; return x; }`)

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
	_, err := analyzeSrc(t, `fun main () -> int { return x; }`)

	require.Error(t, err)
}

func TestAnalyze_Assignment(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let x int = 10; x = 15; return x; }`)

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
	_, err := analyzeSrc(t, `fun main () -> int { x = 15; return x; }`)

	require.Error(t, err)
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
			_, err := analyzeSrc(t, tt.test)
			assert.Error(t, err)
		})
	}
}

func TestAnalyze_Negation(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let x = -10; return -x; }`)

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
	funcs := mustAnalyze(t, `fun main () -> int { let x int = 10; x += 15; return x; }`)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 3)
	decl := fun.List[0]
	assign := fun.List[1]
	assert.Equal(t, decl.Sym, assign.List[0].Sym)
}

func TestAnalyze_Call(t *testing.T) {
	funcs := mustAnalyze(t, `fun f (x int) -> int { return x; } fun main () -> int { return f(1); }`)

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
	funcs := mustAnalyze(t, `fun f () -> int { return 0; } fun main () -> int { return f(); }`)

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
			_, err := analyzeSrc(t, tt.test)
			assert.Error(t, err)
		})
	}
}

func TestAnalyze_ForwardReference(t *testing.T) {
	// main calls f, which is declared after main
	funcs := mustAnalyze(t, `fun main () -> int { return f(1); } fun f (x int) -> int { return x; }`)

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
	_, err := analyzeSrc(t, `fun f () -> int { return 0; } fun f () -> int { return 1; } fun main () -> int { return 0; }`)

	require.Error(t, err)
}

func TestAnalyze_Reference(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let x int = 10; let p = &x; return 0; }`)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 3)
	decl := fun.List[1]

	require.Len(t, decl.List, 3)
	ref := decl.List[2]
	assert.Equal(t, ir.OpRef, ref.Op)

	// &x on an int variable yields *int
	require.NotNil(t, ref.Type)
	assert.True(t, types.Equal(types.Pointer(types.Int()), ref.Type))
}

func TestAnalyze_Deref(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let x int = 10; let p = &x; return *p; }`)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 3)
	ret := fun.List[2]

	require.Len(t, ret.List, 1)
	deref := ret.List[0]
	assert.Equal(t, ir.OpDeref, deref.Op)

	// *p where p is *int yields the base type int
	require.NotNil(t, deref.Type)
	assert.True(t, types.Equal(types.Int(), deref.Type))
}

func TestAnalyze_Deref_NonPointer(t *testing.T) {
	_, err := analyzeSrc(t, `fun main () -> int { let x int = 10; return *x; }`)

	assert.Error(t, err)
}

func TestAnalyze_Reference_NonLValue(t *testing.T) {
	_, err := analyzeSrc(t, `fun main () -> int { let x int = 1; let p = &(x + 1); return 0; }`)

	assert.Error(t, err)
}

func TestAnalyze_GlobalName(t *testing.T) {
	funcs := mustAnalyze(t, `fun f () -> int { return 0; }`)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	// a named global is registered as a function symbol under that name
	assert.Equal(t, "f", fun.Signature.Name.Ident())
	assert.Equal(t, "f", fun.Signature.Label)
	require.NotNil(t, fun.Sym)
	assert.Equal(t, "f", fun.Sym.Name)
	assert.Equal(t, ir.SymFunc, fun.Sym.Kind)
}

func TestAnalyze_GlobalMissingName_Err(t *testing.T) {
	_, err := analyzeSrc(t, `fun () -> int { return 0; }`)

	assert.Error(t, err)
}

func TestAnalyze_LambdaUnnamed(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let f fun (int) -> int = fun (x int) -> int { return x; }; return f(1); }`)

	require.Len(t, funcs, 2)
	lambda := funcs[1]
	require.Equal(t, ir.OpFunction, lambda.Op)
	require.NotNil(t, lambda.Signature)

	// the lambda carries no name of its own
	assert.Nil(t, lambda.Signature.Name)
	assert.Equal(t, "main.func0", lambda.Signature.Label)
}

func TestAnalyze_LiftsLambdas(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let f fun () -> int = fun () -> int { let g fun () -> int = fun () -> int { return 1; }; return g(); }; return f(); }`)

	// globals come first, then every lambda the program contains
	require.Len(t, funcs, 3)
	assert.Equal(t, "main", funcs[0].Signature.Label)
	assert.Equal(t, "main.func0", funcs[1].Signature.Label)
	assert.Equal(t, "main.func0.func0", funcs[2].Signature.Label)

	// lifting exposes the lambdas without detaching them from the tree they came from
	require.Len(t, funcs[0].List, 2)
	decl := funcs[0].List[0]
	require.Len(t, decl.List, 3)
	assert.Same(t, funcs[1], decl.List[2])
}

func TestAnalyze_LambdaNamed_Err(t *testing.T) {
	_, err := analyzeSrc(t, `fun main () -> int { let f fun (int) -> int = fun g (x int) -> int { return x; }; return f(1); }`)

	assert.Error(t, err)
}

func TestAnalyze_Capture(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let x = 10; let f fun (int) -> int = fun (y int) -> int { return x + y; }; return f(2); }`)

	require.Len(t, funcs, 2)
	main := funcs[0]
	lambda := funcByLabel(t, funcs, "main.func0")

	// x lives in main, so the lambda closes over it
	assert.Equal(t, []string{"x"}, captureNames(lambda))

	// and main itself, which owns x and f, closes over nothing
	assert.Empty(t, captureNames(main))
}

func TestAnalyze_Capture_LambdaParam(t *testing.T) {
	funcs := mustAnalyze(t, `fun adderFactory (x int) -> fun (int) -> int { return fun (y int) -> int { return x + y; }; }`)

	require.Len(t, funcs, 2)
	factory := funcs[0]
	lambda := funcByLabel(t, funcs, "adderFactory.func0")

	// x is the factory's param and y is the lambda's own, so only x crosses the boundary
	assert.Equal(t, []string{"x"}, captureNames(lambda))
	assert.Empty(t, captureNames(factory))
}

func TestAnalyze_Capture_Transitive(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let x = 10; let f fun () -> int = fun () -> int { let g fun () -> int = fun () -> int { return x; }; return g(); }; return f(); }`)

	require.Len(t, funcs, 3)
	main := funcs[0]
	outer := funcByLabel(t, funcs, "main.func0")
	inner := funcByLabel(t, funcs, "main.func0.func0")

	// the middle lambda builds the inner one's environment, so it needs x too
	assert.Equal(t, []string{"x"}, captureNames(inner))
	assert.Equal(t, []string{"x"}, captureNames(outer))
	assert.Empty(t, captureNames(main))
}

func TestAnalyze_Capture_AssignedVariable(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let x = 10; let f fun () -> int = fun () -> int { x = 5; return x; }; return f(); }`)

	require.Len(t, funcs, 2)
	lambda := funcByLabel(t, funcs, "main.func0")

	// writing to an outer variable captures it just as reading does
	assert.Equal(t, []string{"x"}, captureNames(lambda))
}

func TestAnalyze_Capture_Recursion(t *testing.T) {
	funcs := mustAnalyze(t, `fun f (n int) -> int { return f(n); }`)

	require.Len(t, funcs, 1)

	// a function naming itself must not end up capturing itself
	assert.Empty(t, captureNames(funcs[0]))
}

func TestAnalyze_Tuple(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let x (int, ()) = (5, ()); return x.0; }`)

	require.Len(t, funcs, 1)
	f := funcs[0]

	require.Len(t, f.List, 2)
	e := f.List[0]

	require.Len(t, e.List, 3)
	assert.True(t, types.Equal(e.List[2].Type, types.Tuple([]*types.Type{types.Int(), types.Unit()})))
}

func TestAnalyze_Tuple_Inference(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let x = (5, ()); return x.0; }`)

	require.Len(t, funcs, 1)
	f := funcs[0]

	require.Len(t, f.List, 2)
	e := f.List[0]

	require.Len(t, e.List, 3)
	assert.True(t, types.Equal(e.List[2].Type, types.Tuple([]*types.Type{types.Int(), types.Unit()})))
}

func TestAnalyze_Tuple_Direct(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { return (1, 2).0; }`)

	require.Len(t, funcs, 1)
	f := funcs[0]

	require.Len(t, f.List, 1)
	e := f.List[0]

	require.Len(t, e.List, 1)
	dot := e.List[0]

	// an unhinted tuple literal resolves its own elements, so the projection is a concrete int
	require.Len(t, dot.List, 2)
	assert.True(t, types.Equal(dot.List[0].Type, types.Tuple([]*types.Type{types.Int(), types.Int()})))
	assert.True(t, types.Equal(dot.Type, types.Int()))
}

func TestAnalyze_Tuple_Nested(t *testing.T) {
	funcs := mustAnalyze(t, `fun main () -> int { let x = ((1, 2), 3); return x.1; }`)

	require.Len(t, funcs, 1)
	f := funcs[0]

	require.Len(t, f.List, 2)
	e := f.List[0]

	require.Len(t, e.List, 3)
	inner := types.Tuple([]*types.Type{types.Int(), types.Int()})
	assert.True(t, types.Equal(e.List[2].Type, types.Tuple([]*types.Type{inner, types.Int()})))
}

func TestAnalyze_Tuple_LValue(t *testing.T) {
	funcs := mustAnalyze(t, `fun f (x (int, ())) { x.0 = 15; }`)

	require.Len(t, funcs, 1)
	f := funcs[0]

	require.Len(t, f.List, 1)
	e := f.List[0]

	require.Len(t, e.List, 2)
	assert.Equal(t, ir.OpDot, e.List[0].Op)
	assert.True(t, types.Equal(e.List[1].Type, types.Int()))
}

func TestAnalyze_Tuple_Dot_Err(t *testing.T) {
	tests := []struct {
		name string
		test string
	}{
		{"field past end", `fun main () -> int { let x (int,) = (1,); return x.1; }`},
		{"field past end of larger tuple", `fun main () -> int { let x (int, int) = (1, 2); return x.2; }`},
		{"field beyond int64", `fun main () -> int { let x (int,) = (1,); return x.99999999999999999999; }`},
		{"named field", `fun main () -> int { let y int = 0; let x (int, int) = (1, 2); return x.y; }`},
		{"dot on non-tuple", `fun main () -> int { let x int = 1; return x.0; }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := analyzeSrc(t, tt.test)
			assert.Error(t, err)
		})
	}
}

// mustAnalyze parses and analyzes src, returning every function in the program, lifted lambdas included.
func mustAnalyze(t *testing.T, src string) []*ir.Node {
	t.Helper()

	funcs, err := analyzeSrc(t, src)
	require.NoError(t, err)

	return funcs
}

// analyzeSrc parses src and runs analysis over it, handing back whatever the analyzer produced.
func analyzeSrc(t *testing.T, src string) ([]*ir.Node, error) {
	t.Helper()

	return Analyze(mustParse(t, src))
}

// funcByLabel finds the lifted function carrying the given label.
func funcByLabel(t *testing.T, funcs []*ir.Node, label string) *ir.Node {
	t.Helper()

	for _, f := range funcs {
		if f.Signature != nil && f.Signature.Label == label {
			return f
		}
	}

	require.FailNowf(t, "no function with label", "label: %s", label)

	return nil
}

func mustParse(t *testing.T, inputStr string) []*ir.Node {
	t.Helper()

	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	return funcs
}

// captureNames drains a function's capture set into sorted names, since map order is unspecified.
func captureNames(fun *ir.Node) []string {
	var names []string
	for sym := range fun.Captures() {
		names = append(names, sym.Name)
	}
	slices.Sort(names)

	return names
}
