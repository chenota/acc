package parser

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/chenota/acc/internal/ir"
	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/types"
)

func TestParser_MainFunc(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return 0; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Signature)
	require.NotNil(t, fun.Signature.Name)
	assert.Equal(t, "main", fun.Signature.Name.Ident())

	require.NotNil(t, fun.Signature.Result)
	assert.True(t, types.Equal(types.Int(), fun.Signature.Result.Type))

	require.Len(t, fun.List, 1)
	ret := fun.List[0]
	assert.Equal(t, ir.OpReturn, ret.Op)

	require.Len(t, ret.List, 1)
	e := ret.List[0]
	assert.Equal(t, ir.OpInt, e.Op)
	assert.NotNil(t, e.Val.(*big.Int))
}

func TestParser_FunctionErr(t *testing.T) {
	tests := []struct {
		name string
		test string
	}{
		{"missing parenthesis 1", `fun main ( -> int { return 0; }`},
		{"missing parenthesis 2", `fun main ) -> int { return 0; }`},
		{"extra parenthesis", `fun main (() -> int { return 0; }`},
		{"missing bracket 1", `fun main () -> int { return 0;`},
		{"missing bracket 2", `fun main () -> int return 0; }`},
		{"missing fun keyword", `main () -> int { return 0; }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := requireTokenize(t, tt.test)
			_, err := ParseProgram(tokens)
			assert.Error(t, err)
		})
	}
}

func TestParser_StmtErr(t *testing.T) {
	tests := []struct {
		name string
		test string
	}{
		{"missing semicolon", `fun main () -> int { return 0 }`},
		{"let without equals", `fun main () -> int { let x 10; }`},
		{"assignment without expression", `fun main () -> int { x = ; }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := requireTokenize(t, tt.test)
			_, err := ParseProgram(tokens)
			assert.Error(t, err)
		})
	}
}

func TestParser_ExprErr(t *testing.T) {
	tests := []struct {
		name string
		test string
	}{
		{"extra int", `fun main () -> int { return 0 0; }`},
		{"missing right operand", `fun main () -> int { return 4 + ;}`},
		{"missing left operand", `fun main () -> int { return / 5; }`},
		{"operator by itself", `fun main () -> int { return *; }`},
		{"reference missing operand", `fun main () -> int { return &; }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := requireTokenize(t, tt.test)
			_, err := ParseProgram(tokens)
			assert.Error(t, err)
		})
	}
}

func TestParser_Precedence(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return 1 + 1 * 2; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	ret := fun.List[0]

	require.Len(t, ret.List, 1)
	e := ret.List[0]

	assert.Equal(t, ir.OpPlus, e.Op)

	require.Len(t, e.List, 2)
	left := e.List[0]
	right := e.List[1]

	assert.Equal(t, ir.OpInt, left.Op)
	assert.Equal(t, ir.OpTimes, right.Op)
}

func TestParser_Associativity(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return 3 - 1 - 1; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	ret := fun.List[0]

	require.Len(t, ret.List, 1)
	e := ret.List[0]

	assert.Equal(t, ir.OpMinus, e.Op)

	require.Len(t, e.List, 2)
	left := e.List[0]
	right := e.List[1]

	assert.Equal(t, ir.OpMinus, left.Op)
	assert.Equal(t, ir.OpInt, right.Op)
}

func TestParser_NegationPrecedence(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return -x + 2; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	ret := fun.List[0]

	require.Len(t, ret.List, 1)
	e := ret.List[0]

	// top level should be addition
	assert.Equal(t, ir.OpPlus, e.Op)

	require.Len(t, e.List, 2)
	left := e.List[0]
	right := e.List[1]

	assert.Equal(t, ir.OpNegate, left.Op)
	assert.Equal(t, ir.OpInt, right.Op)

	require.Len(t, left.List, 1)
	assert.Equal(t, ir.OpIdent, left.List[0].Op)
}

func TestParser_PrecedenceWithParens(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return (1 + 1) * 2; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	ret := fun.List[0]

	require.Len(t, ret.List, 1)
	e := ret.List[0]

	assert.Equal(t, ir.OpTimes, e.Op)

	require.Len(t, e.List, 2)
	left := e.List[0]
	right := e.List[1]

	assert.Equal(t, ir.OpPlus, left.Op)
	assert.Equal(t, ir.OpInt, right.Op)
}

func TestParser_NestedParens(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return (((((0))))); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	ret := fun.List[0]

	require.Len(t, ret.List, 1)
	e := ret.List[0]
	assert.Equal(t, ir.OpInt, e.Op)
}

func TestParser_Ident(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return _burger123; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	ret := fun.List[0]

	require.Len(t, ret.List, 1)
	e := ret.List[0]
	assert.Equal(t, ir.OpIdent, e.Op)
	assert.Equal(t, "_burger123", e.Ident())
}

func TestParser_Declaration_WithType(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { let x int = 10; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	decl := fun.List[0]
	assert.Equal(t, ir.OpDeclaration, decl.Op)

	require.Len(t, decl.List, 3)
	name := decl.List[0]
	varType := decl.List[1]
	expr := decl.List[2]
	assert.Equal(t, ir.OpIdent, name.Op)
	assert.Equal(t, "x", name.Ident())
	assert.Equal(t, ir.OpType, varType.Op)
	assert.Equal(t, ir.OpInt, expr.Op)
}

func TestParser_Declaration_WithoutType(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { let x = 10; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	decl := fun.List[0]
	assert.Equal(t, ir.OpDeclaration, decl.Op)

	require.Len(t, decl.List, 3)
	name := decl.List[0]
	varType := decl.List[1]
	expr := decl.List[2]
	assert.Equal(t, ir.OpIdent, name.Op)
	assert.Equal(t, "x", name.Ident())
	assert.Nil(t, varType)
	assert.Equal(t, ir.OpInt, expr.Op)
}

func TestParser_Assignment(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { x = 10; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	decl := fun.List[0]
	assert.Equal(t, ir.OpAssignment, decl.Op)

	require.Len(t, decl.List, 2)
	target := decl.List[0]
	expr := decl.List[1]
	assert.Equal(t, ir.OpIdent, target.Op)
	assert.Equal(t, "x", target.Ident())
	assert.Equal(t, ir.OpInt, expr.Op)
}

func TestParser_Assignment_NonIdentTarget(t *testing.T) {
	// the parser blindly accepts any expression as an assignment target;
	// lvalue validity is enforced later in semantic analysis
	tokens := requireTokenize(t, `fun main () -> int { x + 1 = 5; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	assign := fun.List[0]
	assert.Equal(t, ir.OpAssignment, assign.Op)

	require.Len(t, assign.List, 2)
	assert.Equal(t, ir.OpPlus, assign.List[0].Op)
}

func TestParser_StmtList(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { let x int = 5; x = 10; return x; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	assert.Len(t, fun.List, 3)
}

func TestParser_AssignmentOp(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { x += 10; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	decl := fun.List[0]
	assert.Equal(t, ir.OpPlusEq, decl.Op)

	require.Len(t, decl.List, 2)
	target := decl.List[0]
	expr := decl.List[1]
	assert.Equal(t, ir.OpIdent, target.Op)
	assert.Equal(t, "x", target.Ident())
	assert.Equal(t, ir.OpInt, expr.Op)
}

func TestParser_MultiGloblFunc(t *testing.T) {
	tokens := requireTokenize(t, `fun test () -> int { return 10; } fun test2 () -> int { return 10; } fun main () -> int { return 15; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 3)
	assert.Equal(t, "test", funcs[0].Signature.Name.Ident())
	assert.Equal(t, "test2", funcs[1].Signature.Name.Ident())
	assert.Equal(t, "main", funcs[2].Signature.Name.Ident())
}

func TestParser_NoParams(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return 0; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Signature)
	assert.Empty(t, fun.Signature.Params)
}

func TestParser_NoReturnType(t *testing.T) {
	tokens := requireTokenize(t, `fun main () { return 0; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Signature)
	assert.Empty(t, fun.Signature.Params)

	assert.Nil(t, fun.Signature.Result)
}

func TestParser_SingleParam(t *testing.T) {
	tokens := requireTokenize(t, `fun main (x int) -> int { return 0; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Signature)
	require.Len(t, fun.Signature.Params, 1)

	param := fun.Signature.Params[0]
	assert.Equal(t, ir.OpParam, param.Op)
	assert.Equal(t, fun, param.Parent)

	require.Len(t, param.List, 2)
	assert.Equal(t, ir.OpIdent, param.List[0].Op)
	assert.Equal(t, "x", param.List[0].Ident())
	assert.Equal(t, ir.OpType, param.List[1].Op)
	assert.True(t, types.Equal(types.Int(), param.List[1].Type))
}

func TestParser_MultipleParams(t *testing.T) {
	tokens := requireTokenize(t, `fun main (x int, y int, z int) -> int { return 0; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Signature)
	require.Len(t, fun.Signature.Params, 3)

	assert.Equal(t, ir.OpParam, fun.Signature.Params[0].Op)
	assert.Equal(t, "x", fun.Signature.Params[0].List[0].Ident())
	assert.True(t, types.Equal(types.Int(), fun.Signature.Params[0].List[1].Type))

	assert.Equal(t, ir.OpParam, fun.Signature.Params[1].Op)
	assert.Equal(t, "y", fun.Signature.Params[1].List[0].Ident())
	assert.True(t, types.Equal(types.Int(), fun.Signature.Params[1].List[1].Type))

	assert.Equal(t, ir.OpParam, fun.Signature.Params[2].Op)
	assert.Equal(t, "z", fun.Signature.Params[2].List[0].Ident())
	assert.True(t, types.Equal(types.Int(), fun.Signature.Params[2].List[1].Type))
}

func TestParser_ParamErr(t *testing.T) {
	tests := []struct {
		name string
		test string
	}{
		{"leading comma", `fun main (, x int) -> int { return 0; }`},
		{"double comma", `fun main (x int,, y int) -> int { return 0; }`},
		{"missing comma", `fun main (x int y int) -> int { return 0; }`},
		{"missing param type", `fun main (x) -> int { return 0; }`},
		{"missing param type after comma", `fun main (x,) -> int { return 0; }`},
		{"missing param name", `fun main (int) -> int { return 0; }`},
		{"comma only", `fun main (,) -> int { return 0; }`},
		{"unclosed param list", `fun main (x int, -> int { return 0; }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := requireTokenize(t, tt.test)
			_, err := ParseProgram(tokens)
			assert.Error(t, err)
		})
	}
}

func TestParser_Call_AsStmt(t *testing.T) {
	tokens := requireTokenize(t, `fun main () { f(); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	call := fun.List[0]
	assert.Equal(t, ir.OpCall, call.Op)
}

func TestParser_Call_NoArgs(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return f(); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	ret := fun.List[0]

	require.Len(t, ret.List, 1)
	call := ret.List[0]
	assert.Equal(t, ir.OpCall, call.Op)

	// list holds only the callee when there are no arguments
	require.Len(t, call.List, 1)
	assert.Equal(t, ir.OpIdent, call.List[0].Op)
	assert.Equal(t, "f", call.List[0].Ident())
}

func TestParser_Call_OneArg(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return f(1); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	call := fun.List[0].List[0]
	assert.Equal(t, ir.OpCall, call.Op)

	require.Len(t, call.List, 2)
	assert.Equal(t, ir.OpIdent, call.List[0].Op)
	assert.Equal(t, "f", call.List[0].Ident())
	assert.Equal(t, ir.OpInt, call.List[1].Op)
}

func TestParser_Call_MultipleArgs(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return f(1, 2, 3); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	call := fun.List[0].List[0]
	assert.Equal(t, ir.OpCall, call.Op)

	require.Len(t, call.List, 4)
	assert.Equal(t, "f", call.List[0].Ident())
	assert.Equal(t, ir.OpInt, call.List[1].Op)
	assert.Equal(t, ir.OpInt, call.List[2].Op)
	assert.Equal(t, ir.OpInt, call.List[3].Op)
}

func TestParser_Call_Chained(t *testing.T) {
	// calls are left-associative
	tokens := requireTokenize(t, `fun main () -> int { return f()(); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	call := fun.List[0].List[0]
	assert.Equal(t, ir.OpCall, call.Op)

	require.Len(t, call.List, 1)
	inner := call.List[0]
	assert.Equal(t, ir.OpCall, inner.Op)
	require.Len(t, inner.List, 1)
	assert.Equal(t, "f", inner.List[0].Ident())
}

func TestParser_Call_NegationPrecedence(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return -f(); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	e := fun.List[0].List[0]
	assert.Equal(t, ir.OpNegate, e.Op)

	require.Len(t, e.List, 1)
	assert.Equal(t, ir.OpCall, e.List[0].Op)
}

func TestParser_Call_BinaryPrecedence(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return a + f(); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	e := fun.List[0].List[0]
	assert.Equal(t, ir.OpPlus, e.Op)

	require.Len(t, e.List, 2)
	assert.Equal(t, ir.OpIdent, e.List[0].Op)
	assert.Equal(t, ir.OpCall, e.List[1].Op)
}

func TestParser_CallErr(t *testing.T) {
	tests := []struct {
		name string
		test string
	}{
		{"leading comma", `fun main () -> int { return f(,1); }`},
		{"double comma", `fun main () -> int { return f(1,,2); }`},
		{"missing comma", `fun main () -> int { return f(1 2); }`},
		{"unclosed paren", `fun main () -> int { return f(1; }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := requireTokenize(t, tt.test)
			_, err := ParseProgram(tokens)
			assert.Error(t, err)
		})
	}
}

func TestParser_Reference(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return &x; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	e := fun.List[0].List[0]
	assert.Equal(t, ir.OpRef, e.Op)

	require.Len(t, e.List, 1)
	assert.Equal(t, ir.OpIdent, e.List[0].Op)
	assert.Equal(t, "x", e.List[0].Ident())
}

func TestParser_Dereference(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return *p; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	e := fun.List[0].List[0]
	assert.Equal(t, ir.OpDeref, e.Op)

	require.Len(t, e.List, 1)
	assert.Equal(t, ir.OpIdent, e.List[0].Op)
	assert.Equal(t, "p", e.List[0].Ident())
}

func TestParser_Dereference_Nested(t *testing.T) {
	// prefix operators bind right, so **p is *(*p)
	tokens := requireTokenize(t, `fun main () -> int { return **p; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	outer := fun.List[0].List[0]
	assert.Equal(t, ir.OpDeref, outer.Op)

	require.Len(t, outer.List, 1)
	inner := outer.List[0]
	assert.Equal(t, ir.OpDeref, inner.Op)

	require.Len(t, inner.List, 1)
	assert.Equal(t, ir.OpIdent, inner.List[0].Op)
	assert.Equal(t, "p", inner.List[0].Ident())
}

func TestParser_Dereference_MultiplyPrecedence(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { return *a * *b; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	e := fun.List[0].List[0]
	assert.Equal(t, ir.OpTimes, e.Op)

	require.Len(t, e.List, 2)
	left := e.List[0]
	right := e.List[1]

	assert.Equal(t, ir.OpDeref, left.Op)
	require.Len(t, left.List, 1)
	assert.Equal(t, "a", left.List[0].Ident())

	assert.Equal(t, ir.OpDeref, right.Op)
	require.Len(t, right.List, 1)
	assert.Equal(t, "b", right.List[0].Ident())
}

func TestParser_Reference_BinaryPrecedence(t *testing.T) {
	// &x + 1 parses as (&x) + 1
	tokens := requireTokenize(t, `fun main () -> int { return &x + 1; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	e := fun.List[0].List[0]
	assert.Equal(t, ir.OpPlus, e.Op)

	require.Len(t, e.List, 2)
	assert.Equal(t, ir.OpRef, e.List[0].Op)
	assert.Equal(t, ir.OpInt, e.List[1].Op)
}

func TestParser_Dereference_CallPrecedence(t *testing.T) {
	// *f() parses as *(f())
	tokens := requireTokenize(t, `fun main () -> int { return *f(); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	e := fun.List[0].List[0]
	assert.Equal(t, ir.OpDeref, e.Op)

	require.Len(t, e.List, 1)
	assert.Equal(t, ir.OpCall, e.List[0].Op)
}

func TestParser_PointerType(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { let p *int = x; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	decl := fun.List[0]
	assert.Equal(t, ir.OpDeclaration, decl.Op)

	require.Len(t, decl.List, 3)
	varType := decl.List[1]
	assert.Equal(t, ir.OpType, varType.Op)
	require.NotNil(t, varType.Type)
	assert.True(t, types.Equal(types.Pointer(types.Int()), varType.Type))
}

func TestParser_PointerType_Nested(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { let p **int = x; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	decl := fun.List[0]

	require.Len(t, decl.List, 3)
	varType := decl.List[1]
	assert.Equal(t, ir.OpType, varType.Op)
	require.NotNil(t, varType.Type)
	assert.True(t, types.Equal(types.Pointer(types.Pointer(types.Int())), varType.Type))
}

func TestParser_PointerType_Result(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> *int { return x; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Signature)
	require.NotNil(t, fun.Signature.Result)
	require.NotNil(t, fun.Signature.Result.Type)
	assert.True(t, types.Equal(types.Pointer(types.Int()), fun.Signature.Result.Type))
}

func TestParser_TypeErr(t *testing.T) {
	tests := []struct {
		name string
		test string
	}{
		{"pointer missing subtype", `fun main () -> int { let p * = x; }`},
		{"pointer result missing subtype", `fun main () -> * { return x; }`},
		{"function type missing parameter list", `fun () -> fun {}`},
		{"function type missing closing paren", `fun () -> fun (int -> int {}`},
		{"function type missing result", `fun () -> fun (int) -> {}`},
		{"function type missing result in declaration", `fun main () -> int { let f fun (int) -> = x; }`},
		{"function type missing fun keyword", `fun () -> (int) -> int {}`},
		{"type list leading comma", `fun () -> fun (, int) {}`},
		{"type list double comma", `fun () -> fun (int,, int) {}`},
		{"type list missing comma", `fun () -> fun (int int) {}`},
		{"unit type missing closing paren", `fun () -> ( {}`},
		{"parenthesized type", `fun () -> (int) {}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := requireTokenize(t, tt.test)
			_, err := ParseProgram(tokens)
			assert.Error(t, err)
		})
	}
}

func TestParser_AnonymousFunc(t *testing.T) {
	tokens := requireTokenize(t, `fun () { return; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	assert.Nil(t, fun.Signature.Name)
}

func TestParser_FuncAsExpr(t *testing.T) {
	tokens := requireTokenize(t, `fun () { let f = fun () -> int { return 10; }; return; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 2)
	innerFun := fun.List[0].List[2]

	require.NotNil(t, innerFun.Signature)
	assert.Nil(t, innerFun.Signature.Name)
	assert.Len(t, innerFun.List, 1)
}

func TestParser_FunType(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		want *types.Type
	}{
		{
			"single param",
			`fun (int) -> int`,
			types.Function([]*types.Type{types.Int()}, types.Int()),
		},
		{
			"implicit unit result",
			`fun ()`,
			types.Function(nil, types.Unit()),
		},
		{
			"function result",
			`fun () -> fun ()`,
			types.Function(nil, types.Function(nil, types.Unit())),
		},
		{
			"multiple params",
			`fun (int, int) -> ()`,
			types.Function([]*types.Type{types.Int(), types.Int()}, types.Unit()),
		},
		{
			"function type param",
			`fun (fun (int) -> int) -> int`,
			types.Function([]*types.Type{types.Function([]*types.Type{types.Int()}, types.Int())}, types.Int()),
		},
		{
			"pointer to function",
			`*fun ()`,
			types.Pointer(types.Function(nil, types.Unit())),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// each type is tested as the result type of an empty function
			tokens := requireTokenize(t, fmt.Sprintf(`fun () -> %s {}`, tt.typ))

			funcs, err := ParseProgram(tokens)
			require.NoError(t, err)

			require.Len(t, funcs, 1)
			fun := funcs[0]

			require.NotNil(t, fun.Signature)
			require.NotNil(t, fun.Signature.Result)
			assert.True(t, types.Equal(tt.want, fun.Signature.Result.Type))
		})
	}
}

func TestParser_Dot(t *testing.T) {
	tokens := requireTokenize(t, `fun () { return x.y.z; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	dot := fun.List[0].List[0]

	assert.Equal(t, ir.OpDot, dot.Op)
	require.Len(t, dot.List, 2)

	// left associativity
	assert.Equal(t, ir.OpDot, dot.List[0].Op)
	assert.Equal(t, ir.OpIdent, dot.List[1].Op)
}

func TestParser_DotInt(t *testing.T) {
	tokens := requireTokenize(t, `fun () { return x.1; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	dot := fun.List[0].List[0]

	assert.Equal(t, ir.OpDot, dot.Op)
	require.Len(t, dot.List, 2)

	assert.Equal(t, ir.OpInt, dot.List[1].Op)
}

func TestParser_DotBadInt(t *testing.T) {
	tokens := requireTokenize(t, `fun () { return x.1_0; }`)

	_, err := ParseProgram(tokens)
	require.Error(t, err)
}

func TestParser_Unit(t *testing.T) {
	tokens := requireTokenize(t, `fun () { return (); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	ret := fun.List[0]

	require.Len(t, ret.List, 1)
	e := ret.List[0]

	assert.Equal(t, ir.OpUnit, e.Op)
	assert.Empty(t, e.List)
}

func TestParser_SingleElementTuple(t *testing.T) {
	tokens := requireTokenize(t, `fun () { return (x,); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	ret := fun.List[0]

	require.Len(t, ret.List, 1)
	tuple := ret.List[0]

	assert.Equal(t, ir.OpTuple, tuple.Op)

	require.Len(t, tuple.List, 1)
	assert.Equal(t, ir.OpIdent, tuple.List[0].Op)
	assert.Equal(t, "x", tuple.List[0].Ident())
}

func TestParser_Tuple(t *testing.T) {
	tokens := requireTokenize(t, `fun () { return (x, y, z); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	ret := fun.List[0]

	require.Len(t, ret.List, 1)
	tuple := ret.List[0]

	assert.Equal(t, ir.OpTuple, tuple.Op)

	require.Len(t, tuple.List, 3)
	assert.Equal(t, ir.OpIdent, tuple.List[0].Op)
	assert.Equal(t, "x", tuple.List[0].Ident())
	assert.Equal(t, ir.OpIdent, tuple.List[1].Op)
	assert.Equal(t, "y", tuple.List[1].Ident())
	assert.Equal(t, ir.OpIdent, tuple.List[2].Op)
	assert.Equal(t, "z", tuple.List[2].Ident())
}

func TestParser_TupleErr(t *testing.T) {
	tests := []struct {
		name string
		test string
	}{
		{"missing separator", `fun () { return (1 2); }`},
		{"missing separator after comma", `fun () { return (1, 2 3); }`},
		{"missing closing parenthesis", `fun () { return (1 + 2; }`},
		{"missing closing parenthesis after comma", `fun () { return (1, 2; }`},
		{"leading comma", `fun () { return (, 1); }`},
		{"consecutive commas", `fun () { return (1,, 2); }`},
		{"comma by itself", `fun () { return (,); }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := requireTokenize(t, tt.test)
			_, err := ParseProgram(tokens)
			assert.Error(t, err)
		})
	}
}

func TestParser_CallTrailingComma(t *testing.T) {
	tokens := requireTokenize(t, `fun () { return f(1, 2,); }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	ret := fun.List[0]

	require.Len(t, ret.List, 1)
	call := ret.List[0]

	assert.Equal(t, ir.OpCall, call.Op)

	require.Len(t, call.List, 3)
	assert.Equal(t, ir.OpIdent, call.List[0].Op)
	assert.Equal(t, ir.OpInt, call.List[1].Op)
	assert.Equal(t, ir.OpInt, call.List[2].Op)
}

func TestParser_ParamsTrailingComma(t *testing.T) {
	tokens := requireTokenize(t, `fun main (x int, y int,) -> int { return 0; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.NotNil(t, fun.Signature)
	require.Len(t, fun.Signature.Params, 2)

	assert.Equal(t, ir.OpParam, fun.Signature.Params[0].Op)
	assert.True(t, types.Equal(types.Int(), fun.Signature.Params[0].List[1].Type))
	assert.Equal(t, ir.OpParam, fun.Signature.Params[1].Op)
	assert.True(t, types.Equal(types.Int(), fun.Signature.Params[1].List[1].Type))
}

func TestParser_FunctionTypeTrailingComma(t *testing.T) {
	tokens := requireTokenize(t, `fun main () -> int { let f fun (int, int,) -> int = g; }`)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	require.Len(t, fun.List, 1)
	decl := fun.List[0]
	assert.Equal(t, ir.OpDeclaration, decl.Op)

	require.Len(t, decl.List, 3)
	declType := decl.List[1]
	require.NotNil(t, declType)
	assert.True(t, types.Equal(types.Function([]*types.Type{types.Int(), types.Int()}, types.Int()), declType.Type))
}

func requireTokenize(t *testing.T, input string) *lexer.TokenList {
	tokens, err := lexer.Tokenize(strings.NewReader(input))
	require.NoError(t, err)
	return tokens
}
