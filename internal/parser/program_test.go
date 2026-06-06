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

func TestProgram_MainFunc(t *testing.T) {
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

func TestProgram_Err(t *testing.T) {
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
