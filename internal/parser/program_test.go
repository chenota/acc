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
	// we're cheating a little bit by using the lexer here but it makes writing tests much easier
	inputStr := `fun main () -> int { return 0; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := ParseProgram(tokens)
	require.NoError(t, err)

	require.Len(t, funcs, 1)
	fun := funcs[0]

	assert.Equal(t, fun.Name, "main")

	require.NotNil(t, fun.Signature)
	require.NotNil(t, fun.Signature.Result)
	assert.Equal(t, fun.Signature.Result.Type, types.Int32)

	require.Len(t, fun.List, 1)
	ret := fun.List[0]
	assert.Equal(t, ir.OpReturn, ret.Op)

	require.Len(t, ret.List, 1)
	e := ret.List[0]
	assert.Equal(t, ir.OpInt, e.Op)
	assert.NotNil(t, e.Val.(*big.Int))
}
