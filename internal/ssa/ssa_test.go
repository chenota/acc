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
	funcs := requireBuildSSA(t, `fun main () -> int { return 0; }`)

	require.Len(t, funcs, 1)
	f := funcs[0]
	assert.Equal(t, "main", f.Name)

	b := f.Blocks[0]
	assert.Equal(t, BlockRet, b.Kind)

	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, types.Int(), b.Control.Type)
	assert.Equal(t, LocRegister, b.Control.Loc.Kind)
	assert.Equal(t, register.RegA, b.Control.Loc.Reg)
}

func TestGenSsa_ConstantFolding(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { return 1 + 1; }`)

	b := funcs[0].Blocks[0]

	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, types.Int(), b.Control.Type)
	assert.Equal(t, int32(2), b.Control.Value)
}

func TestGenSsa_DivideByZero(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { return 1 / 0; }`)

	b := funcs[0].Blocks[0]

	require.NotNil(t, b.Control)
	assert.Equal(t, OpDivide, b.Control.Op)
}

func TestGenSsa_AdditionOverflow(t *testing.T) {
	src := fmt.Sprintf(`fun main () -> int { return %d + 1; }`, math.MaxInt32)
	funcs := requireBuildSSA(t, src)

	b := funcs[0].Blocks[0]

	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, types.Int(), b.Control.Type)
	assert.Equal(t, int32(math.MinInt32), b.Control.Value)
}

func TestGenSsa_Variable(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 10; return x; }`)

	b := funcs[0].Blocks[0]

	// mem2reg promotes x, so no memory operations survive
	assert.Empty(t, findValues(b.Values, OpAlloca), "alloca should be promoted away")
	assert.Empty(t, findValues(b.Values, OpStore), "store should be promoted away")
	assert.Empty(t, findValues(b.Values, OpLoad), "load should be promoted away")

	// the stored value flows directly into the return
	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, int32(10), b.Control.Value)
	assert.Equal(t, LocRegister, b.Control.Loc.Kind)
	assert.Equal(t, register.RegA, b.Control.Loc.Reg)
}

func TestGenSsa_Variable_Assignment(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 10; x = 20; return x; }`)

	b := funcs[0].Blocks[0]

	assert.Empty(t, findValues(b.Values, OpStore), "stores should be promoted away")
	assert.Empty(t, findValues(b.Values, OpLoad), "loads should be promoted away")

	// the most recent definition (20) reaches the return
	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, int32(20), b.Control.Value)
	assert.Equal(t, LocRegister, b.Control.Loc.Kind)
	assert.Equal(t, register.RegA, b.Control.Loc.Reg)
}

func TestGenSsa_Divide(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 10; let y = 2; return x / y; }`)

	b := funcs[0].Blocks[0]

	// both operands promote to constants, so the divide folds away
	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, int32(5), b.Control.Value)
}

func TestGenSsa_Variable_InExpression(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 5; return x + 1; }`)

	b := funcs[0].Blocks[0]

	// x promotes to 5, so x + 1 folds to 6
	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, int32(6), b.Control.Value)
}

func TestGenSsa_Negate_Fold(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { return -10; }`)
	b := funcs[0].Blocks[0]

	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, int32(-10), b.Control.Value.(int32))
}

func TestGenSsa_Variable_Assignment_Operator(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 10; x += 20; return x; }`)

	b := funcs[0].Blocks[0]

	assert.Empty(t, findValues(b.Values, OpStore), "stores should be promoted away")
	assert.Empty(t, findValues(b.Values, OpLoad), "loads should be promoted away")

	// x += 20 promotes and folds to 30
	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, int32(30), b.Control.Value)
	assert.Equal(t, LocRegister, b.Control.Loc.Kind)
	assert.Equal(t, register.RegA, b.Control.Loc.Reg)
}

func requireBuildSSA(t *testing.T, src string) []*Func {
	t.Helper()
	tokens, err := lexer.Tokenize(strings.NewReader(src))
	require.NoError(t, err)
	funcs, err := parser.ParseProgram(tokens)
	require.NoError(t, err)
	require.NoError(t, semantic.Analyze(funcs))
	result, err := BuildAndAllocate(funcs)
	require.NoError(t, err)
	return result
}

func findValues(values []*Value, op Op) []*Value {
	var result []*Value
	for _, v := range values {
		if v.Op == op {
			result = append(result, v)
		}
	}
	return result
}
