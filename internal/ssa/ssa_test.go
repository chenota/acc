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

	assertContainsOpSeq(t, b.Values, OpLiteral, OpStore, OpLoad)

	stores := findValues(b.Values, OpStore)
	require.Len(t, stores, 1)
	require.Len(t, stores[0].Args, 2)
	assert.Equal(t, OpLiteral, stores[0].Args[0].Op)
	assert.Equal(t, int32(10), stores[0].Args[0].Value)
	assert.Equal(t, OpAlloca, stores[0].Args[1].Op)

	loads := findValues(b.Values, OpLoad)
	require.Len(t, loads, 1)
	require.Len(t, loads[0].Args, 1)
	assert.Equal(t, stores[0].Args[1], loads[0].Args[0])

	assert.Equal(t, loads[0], b.Control)
	assert.Equal(t, LocRegister, b.Control.Loc.Kind)
	assert.Equal(t, register.RegA, b.Control.Loc.Reg)
}

func TestGenSsa_Variable_Assignment(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 10; x = 20; return x; }`)

	b := funcs[0].Blocks[0]

	stores := findValues(b.Values, OpStore)
	require.Len(t, stores, 2)
	require.Len(t, stores[1].Args, 2)
	assert.Equal(t, int32(20), stores[1].Args[0].Value)
	assert.Equal(t, stores[0].Args[1], stores[1].Args[1]) // same alloca

	require.NotNil(t, b.Control)
	assert.Equal(t, OpLoad, b.Control.Op)
	assert.Equal(t, LocRegister, b.Control.Loc.Kind)
	assert.Equal(t, register.RegA, b.Control.Loc.Reg)
}

func TestGenSsa_Divide(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 10; let y = 2; return x / y; }`)

	b := funcs[0].Blocks[0]

	require.NotNil(t, b.Control)
	assert.Equal(t, OpDivide, b.Control.Op)
	// divide result is always pre-allocated to %eax by prepareDivides
	assert.Equal(t, LocRegister, b.Control.Loc.Kind)
	assert.Equal(t, register.RegA, b.Control.Loc.Reg)
}

func TestGenSsa_Variable_InExpression(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 5; return x + 1; }`)

	b := funcs[0].Blocks[0]

	require.NotNil(t, b.Control)
	assert.Equal(t, OpAdd, b.Control.Op)
}

func TestGenSsa_Reassociate_FoldMixed(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 5; return 2 + x + 2; }`)
	b := funcs[0].Blocks[0]

	require.NotNil(t, b.Control)
	assert.Equal(t, OpAdd, b.Control.Op)

	var litVals []int32
	for _, lit := range findValues(b.Values, OpLiteral) {
		litVals = append(litVals, lit.Value.(int32))
	}
	assert.Contains(t, litVals, int32(4), "expected 2+2 to fold into 4")
	assert.NotContains(t, litVals, int32(2), "original 2s should be consumed by folding")
}

func TestGenSsa_Negate_Fold(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { return -10; }`)
	b := funcs[0].Blocks[0]

	require.NotNil(t, b.Control)
	assert.Equal(t, OpLiteral, b.Control.Op)
	assert.Equal(t, int32(-10), b.Control.Value.(int32))
}

func TestGenSsa_Negate_NoFold(t *testing.T) {
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 10; return -x; }`)
	b := funcs[0].Blocks[0]

	require.NotNil(t, b.Control)
	assert.Equal(t, OpNegate, b.Control.Op)
}

func TestGenSsa_NegSquash(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		wantOp Op
	}{
		{"subtract", `fun main () -> int { let y = 5; return 0 - -y; }`, OpAdd},
		{"add", `fun main () -> int { let y = 5; return 0 + -y; }`, OpSubtract},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			funcs := requireBuildSSA(t, tt.src)
			b := funcs[0].Blocks[0]

			require.NotNil(t, b.Control)
			assert.Equal(t, tt.wantOp, b.Control.Op)
			assert.Empty(t, findValues(b.Values, OpNegate), "negate should be squashed away")
		})
	}
}

func TestGenSsa_Variable_Assignment_Operator(t *testing.T) {
	returnRegister := register.RegA
	funcs := requireBuildSSA(t, `fun main () -> int { let x = 10; x += 20; return x; }`)

	b := funcs[0].Blocks[0]

	stores := findValues(b.Values, OpStore)
	require.Len(t, stores, 2)
	loads := findValues(b.Values, OpLoad)
	require.Len(t, loads, 2)

	require.NotNil(t, b.Control)
	assert.Equal(t, OpLoad, b.Control.Op)
	assert.Equal(t, LocRegister, b.Control.Loc.Kind)
	assert.Equal(t, returnRegister, b.Control.Loc.Reg)
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

func assertContainsOpSeq(t *testing.T, values []*Value, seq ...Op) {
	t.Helper()
	var seqIdx int
	for _, v := range values {
		if v.Op == seq[seqIdx] {
			seqIdx++
		}
		if seqIdx >= len(seq) {
			return
		}
	}
	assert.Fail(t, "values do not contain the specified sequence of operations", seq)
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
