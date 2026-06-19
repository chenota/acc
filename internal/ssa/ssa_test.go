package ssa

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/chenota/acc/internal/lexer"
	"github.com/chenota/acc/internal/parser"
	"github.com/chenota/acc/internal/register"
	"github.com/chenota/acc/internal/semantic"
	"github.com/chenota/acc/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenSsa_Basic(t *testing.T) {
	inputStr := `fun main () -> int { return 0; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, semantic.Analyze(funcs))

	// not really necessary but ensuring the RAX is the return register
	returnRegister := register.RegA
	compiledFuncs, err := BuildAndAllocate(funcs, WithReturnRegister(returnRegister))
	require.NoError(t, err)

	require.Len(t, compiledFuncs, 1)
	f := compiledFuncs[0]

	assert.Equal(t, "main", f.Name)

	require.Len(t, f.Blocks, 1)
	b := f.Blocks[0]

	assert.Equal(t, BlockRet, b.Kind)

	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, types.KInt32, b.Control.Type.Kind)
	assert.Equal(t, LocRegister, b.Control.Loc.Kind)
	assert.Equal(t, returnRegister, b.Control.Loc.Reg)
}

func TestGenSsa_ConstantFolding(t *testing.T) {
	inputStr := `fun main () -> int { return 1 + 1; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, semantic.Analyze(funcs))

	// not really necessary but ensuring the RAX is the return register
	returnRegister := register.RegA
	compiledFuncs, err := BuildAndAllocate(funcs, WithReturnRegister(returnRegister))
	require.NoError(t, err)

	require.Len(t, compiledFuncs, 1)
	f := compiledFuncs[0]

	assert.Equal(t, "main", f.Name)

	require.Len(t, f.Blocks, 1)
	b := f.Blocks[0]

	assert.Equal(t, BlockRet, b.Kind)

	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, types.KInt32, b.Control.Type.Kind)
	assert.Equal(t, int32(2), b.Control.Value)
}

func TestGenSsa_DivideByZero(t *testing.T) {
	inputStr := `fun main () -> int { return 1 / 0; }`
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, semantic.Analyze(funcs))

	// not really necessary but ensuring the RAX is the return register
	returnRegister := register.RegA
	compiledFuncs, err := BuildAndAllocate(funcs, WithReturnRegister(returnRegister))
	require.NoError(t, err)

	require.Len(t, compiledFuncs, 1)
	f := compiledFuncs[0]

	assert.Equal(t, "main", f.Name)

	require.Len(t, f.Blocks, 1)
	b := f.Blocks[0]

	assert.Equal(t, BlockRet, b.Kind)

	require.NotNil(t, b.Control)
	assert.Equal(t, OpDivide, b.Control.Op)
}

func TestGenSsa_AdditionOverflow(t *testing.T) {
	inputStr := fmt.Sprintf(`fun main () -> int { return %d + 1; }`, math.MaxInt32)
	tokens, err := lexer.Tokenize(strings.NewReader(inputStr))
	require.NoError(t, err)

	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)

	require.NoError(t, semantic.Analyze(funcs))

	// not really necessary but ensuring the RAX is the return register
	returnRegister := register.RegA
	compiledFuncs, err := BuildAndAllocate(funcs, WithReturnRegister(returnRegister))
	require.NoError(t, err)

	require.Len(t, compiledFuncs, 1)
	f := compiledFuncs[0]

	assert.Equal(t, "main", f.Name)

	require.Len(t, f.Blocks, 1)
	b := f.Blocks[0]

	assert.Equal(t, BlockRet, b.Kind)

	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, types.KInt32, b.Control.Type.Kind)
	assert.Equal(t, int32(math.MinInt32), b.Control.Value)
}
