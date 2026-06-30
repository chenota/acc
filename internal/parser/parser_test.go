package parser

import (
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
		{"missing fun name", `fun () -> int { return 0; }`},
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
		{"missing int", `fun main () -> int { return ; }`},
		{"extra int", `fun main () -> int { return 0 0; }`},
		{"missing right operand", `fun main () -> int { return 4 + ;}`},
		{"missing left operand", `fun main () -> int { return / 5; }`},
		{"operator by itself", `fun main () -> int { return *; }`},
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
		{"trailing comma", `fun main (x int,) -> int { return 0; }`},
		{"leading comma", `fun main (, x int) -> int { return 0; }`},
		{"double comma", `fun main (x int,, y int) -> int { return 0; }`},
		{"missing comma", `fun main (x int y int) -> int { return 0; }`},
		{"missing param type", `fun main (x) -> int { return 0; }`},
		{"missing param name", `fun main (int) -> int { return 0; }`},
		{"comma only", `fun main (,) -> int { return 0; }`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := requireTokenize(t, tt.test)
			_, err := ParseProgram(tokens)
			assert.Error(t, err)
		})
	}
}

func requireTokenize(t *testing.T, input string) *lexer.TokenList {
	tokens, err := lexer.Tokenize(strings.NewReader(input))
	require.NoError(t, err)
	return tokens
}
