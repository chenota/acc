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

	assert.Equal(t, fun.Name, "main")

	require.NotNil(t, fun.Signature)
	require.NotNil(t, fun.Signature.Result)
	assert.Equal(t, types.KInt32, fun.Signature.Result.Type.Kind)

	require.Len(t, fun.List, 1)
	ret := fun.List[0]
	assert.Equal(t, ir.OpReturn, ret.Op)

	require.Len(t, ret.List, 1)
	e := ret.List[0]
	assert.Equal(t, ir.OpInt, e.Op)
	assert.NotNil(t, e.Val.(*big.Int))
}

func TestParser_Err(t *testing.T) {
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
		{"missing semicolon", `fun main () -> int { return 0 }`},
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
	assert.Equal(t, "_burger123", e.Name)
}

func requireTokenize(t *testing.T, input string) *lexer.TokenList {
	tokens, err := lexer.Tokenize(strings.NewReader(input))
	require.NoError(t, err)
	return tokens
}
