package ssa

import (
	"fmt"
	"math"
	"slices"
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

func TestLowerCalls_ArgRegisters(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun target (a int, b int, c int) -> int { return 0; }
		fun main () -> int { return target(1, 2, 3); }
	`)

	call := requireCall(t, funcs, "main")

	// Args[0] is the callee reference; the three arguments follow
	require.Len(t, call.Args, 4)

	require.Equal(t, OpFuncRef, call.Args[0].Op)
	callee, ok := call.Args[0].Value.(*Func)
	require.True(t, ok, "callee payload should be a *Func")
	assert.Equal(t, "target", callee.Name)

	assert.Equal(t, LocRegister, call.Args[1].Loc.Kind)
	assert.Equal(t, register.RegDI, call.Args[1].Loc.Reg)

	assert.Equal(t, LocRegister, call.Args[2].Loc.Kind)
	assert.Equal(t, register.RegSI, call.Args[2].Loc.Reg)

	assert.Equal(t, LocRegister, call.Args[3].Loc.Kind)
	assert.Equal(t, register.RegD, call.Args[3].Loc.Reg)
}

func TestLowerCalls_ResultInRAX(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun target (a int) -> int { return 0; }
		fun main () -> int { return target(7); }
	`)

	call := requireCall(t, funcs, "main")

	assert.Equal(t, LocRegister, call.Loc.Kind)
	assert.Equal(t, register.RegA, call.Loc.Reg)
}

func TestLowerCalls_ClobbersCallerSaved(t *testing.T) {
	funcs := requireBuildSSA(t, `
		fun target (a int) -> int { return 0; }
		fun main () -> int { return target(7); }
	`)

	call := requireCall(t, funcs, "main")

	assert.ElementsMatch(t, slices.Collect(register.CallerSaved.All()), call.Clobbers)
}

func TestGenSsa_Params_PinnedToArgRegisters(t *testing.T) {
	funcs := requireBuildSSA(t, `fun target (a int, b int, c int) -> int { return 0; }`)

	f := requireFunc(t, funcs, "target")
	params := findValues(f.Entry.Values, OpParam)
	require.Len(t, params, 3)

	// each incoming parameter is pinned to its System V argument register, in order
	assert.Equal(t, LocRegister, params[0].Loc.Kind)
	assert.Equal(t, register.RegDI, params[0].Loc.Reg)

	assert.Equal(t, LocRegister, params[1].Loc.Kind)
	assert.Equal(t, register.RegSI, params[1].Loc.Reg)

	assert.Equal(t, LocRegister, params[2].Loc.Kind)
	assert.Equal(t, register.RegD, params[2].Loc.Reg)
}

func TestGenSsa_Param_FlowsToReturn(t *testing.T) {
	// returning a parameter used to fail with "variable used before declared"
	funcs := requireBuildSSA(t, `fun identity (x int) -> int { return x; }`)

	f := requireFunc(t, funcs, "identity")

	// the parameter is copied out of its argument register into the return register
	ctrl := f.Entry.Control
	require.NotNil(t, ctrl)
	require.Equal(t, OpCopy, ctrl.Op)
	assert.Equal(t, register.RegA, ctrl.Loc.Reg)

	require.Len(t, ctrl.Args, 1)
	assert.Equal(t, OpParam, ctrl.Args[0].Op)
	assert.Equal(t, register.RegDI, ctrl.Args[0].Loc.Reg)
}

func TestGenSsa_Param_Reassigned(t *testing.T) {
	// a parameter is a mutable local - reassigning it before use discards the incoming value
	funcs := requireBuildSSA(t, `fun f (x int) -> int { x = 55; return x; }`)

	f := requireFunc(t, funcs, "f")

	ctrl := f.Entry.Control
	require.NotNil(t, ctrl)
	assert.Equal(t, OpLiteral, ctrl.Op)
	assert.Equal(t, int32(55), ctrl.Value)
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

func requireFunc(t *testing.T, funcs []*Func, name string) *Func {
	t.Helper()
	for _, f := range funcs {
		if f.Name == name {
			return f
		}
	}
	require.Failf(t, "function not found", "no function named %q", name)
	return nil
}

// requireCall returns the single OpCall value in the named function.
func requireCall(t *testing.T, funcs []*Func, funcName string) *Value {
	t.Helper()
	f := requireFunc(t, funcs, funcName)
	calls := findValues(f.Entry.Values, OpCall)
	require.Len(t, calls, 1)
	return calls[0]
}
